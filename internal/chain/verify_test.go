package chain

import (
	"context"
	"testing"
	"time"

	"auditlog/internal/model"
)

// buildChain constructs a valid contiguous hash chain of n entries so Verify
// performs real per-entry work (prev_hash checks + hash recomputation).
func buildChain(t *testing.T, n int) ([]model.Entry, model.Head) {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := make([]model.Entry, 0, n)
	prev := ZeroHash
	for i := 1; i <= n; i++ {
		req := model.AppendRequest{Actor: "a", Action: "act", Target: "t"}
		e, err := NewEntry(int64(i), prev, req, now.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatalf("NewEntry: %v", err)
		}
		entries = append(entries, e)
		prev = e.Hash
	}
	return entries, model.Head{Seq: entries[len(entries)-1].Seq, Hash: entries[len(entries)-1].Hash, PrevHash: entries[len(entries)-1].PrevHash, UpdatedAt: now}
}

func TestVerifyFullChecksAllEntries(t *testing.T) {
	entries, head := buildChain(t, 5)
	report := Verify(context.Background(), entries, head, model.VerifyRequest{Mode: "full"})
	if report.Status != model.VerifyOK {
		t.Fatalf("expected ok, got %s (%s)", report.Status, report.BreakReason)
	}
	if report.CheckedEntries != 5 {
		t.Fatalf("expected 5 checked, got %d", report.CheckedEntries)
	}
}

// TestVerifyStopsEarlyOnCanceledContext ensures the long per-entry loop honors
// ctx cancellation: a pre-canceled ctx must short-circuit before checking all
// entries, so a client-aborted verify does not keep recomputing the chain.
func TestVerifyStopsEarlyOnCanceledContext(t *testing.T) {
	entries, head := buildChain(t, 500)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate an already-aborted client request

	report := Verify(ctx, entries, head, model.VerifyRequest{Mode: "full"})

	if report.CheckedEntries >= len(entries) {
		t.Fatalf("canceled ctx should stop before checking all %d entries, checked %d", len(entries), report.CheckedEntries)
	}
}
