package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"auditlog/internal/model"
	"auditlog/internal/store"
)

// newSQLiteStore opens a fresh SQLite store in a temp file for integration
// tests that need real QueryContext/ExecContext cancellation semantics.
func newSQLiteStore(t *testing.T) *store.SQLite {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "verify.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// populateChain appends n entries through AuditService on top of the genesis
// row already inserted by store.Open, so the resulting chain has real,
// recomputable hashes for Verify to walk.
func populateChain(t *testing.T, db *store.SQLite, n int) int {
	t.Helper()
	audit := NewAudit(db)
	for i := 0; i < n; i++ {
		req := model.AppendRequest{Actor: "a", Action: "act", Target: "t", Detail: json.RawMessage(`{}`)}
		if _, err := audit.Append(context.Background(), req); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	head, err := db.Head(context.Background())
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	return int(head.Seq) // genesis (seq 1) + n appends
}

// TestVerifyHappyPathWritesLastVerify confirms a normal (non-canceled) verify
// returns ok, checks every entry, and persists last_verify — i.e. the fix
// leaves successful request behavior unchanged.
func TestVerifyHappyPathWritesLastVerify(t *testing.T) {
	db := newSQLiteStore(t)
	total := populateChain(t, db, 5)
	v := NewVerify(db)

	before, _, _ := db.LastVerify(context.Background())

	report, err := v.Verify(context.Background(), model.VerifyRequest{Mode: "full"})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.Status != model.VerifyOK {
		t.Fatalf("expected ok, got %s", report.Status)
	}
	if report.CheckedEntries != total {
		t.Fatalf("expected %d checked, got %d", total, report.CheckedEntries)
	}

	after, status, _ := db.LastVerify(context.Background())
	if !after.After(before) {
		t.Fatalf("expected last_verify to advance past %v, got %v", before, after)
	}
	if status != model.VerifyOK {
		t.Fatalf("expected last_verify status ok, got %q", status)
	}
}

// TestVerifyCanceledContextDoesNotWriteResult confirms the core fix: when the
// client cancels mid-verify, the cancellation propagates through the service
// to chain.Verify's loop and to SetVerify's ExecContext, so last_verify is NOT
// updated with a half-finished result. (With the old code every Store call used
// context.Background(), so a canceled request still wrote a result.)
func TestVerifyCanceledContextDoesNotWriteResult(t *testing.T) {
	db := newSQLiteStore(t)
	populateChain(t, db, 200)
	v := NewVerify(db)

	// Establish a successful baseline so we can detect any later write.
	if _, err := v.Verify(context.Background(), model.VerifyRequest{Mode: "full"}); err != nil {
		t.Fatalf("baseline Verify: %v", err)
	}
	baseline, baselineStatus, _ := db.LastVerify(context.Background())
	if baselineStatus != model.VerifyOK {
		t.Fatalf("baseline last_verify status = %q, want ok", baselineStatus)
	}

	// Simulate a client-aborted request: the ctx is already canceled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := v.Verify(ctx, model.VerifyRequest{Mode: "full"})
	// The old code path would return a non-nil error only sometimes (Background
	// queries succeeded); either way the contract we enforce is "no new write".
	// Accept a nil or non-nil error, but last_verify must not advance.
	_ = err

	after, afterStatus, _ := db.LastVerify(context.Background())
	if !after.Equal(baseline) || afterStatus != baselineStatus {
		t.Fatalf("canceled verify must not persist a result: baseline=%v/%q after=%v/%q",
			baseline, baselineStatus, after, afterStatus)
	}
}
