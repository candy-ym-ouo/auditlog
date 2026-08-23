package store

import (
	"auditlog/internal/chain"
	"auditlog/internal/model"
	"context"
	"path/filepath"
	"testing"
	"time"
)

// seedEntries inserts n entries into audit_entries so Archive has data to move.
// The store seeds a genesis entry (seq=1) on Open, so chaining continues from
// the current head rather than from seq=1.
func seedEntries(t *testing.T, s *SQLite, n int) {
	t.Helper()
	h, err := s.Head(context.Background())
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	prev := h.Hash
	for i := 0; i < n; i++ {
		e := model.Entry{Seq: h.Seq + int64(i+1), PrevHash: prev, Actor: "system", Action: "seed", Target: "t", Detail: []byte(`{}`), EventTime: time.Now().UTC()}
		hsh, err := chain.HashEntry(e)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		e.Hash = hsh
		if _, err := s.Append(context.Background(), e); err != nil {
			t.Fatalf("append: %v", err)
		}
		prev = hsh
	}
}

func countAudit(t *testing.T, s *SQLite) int {
	t.Helper()
	var c int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_entries`).Scan(&c); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return c
}
func countArchiveEntries(t *testing.T, s *SQLite) int {
	t.Helper()
	var c int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM archive_entries`).Scan(&c); err != nil {
		t.Fatalf("count archive_entries: %v", err)
	}
	return c
}
func countBatches(t *testing.T, s *SQLite) int {
	t.Helper()
	var c int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM archive_batches`).Scan(&c); err != nil {
		t.Fatalf("count batches: %v", err)
	}
	return c
}

func TestSQLiteArchiveMovesEntriesAndKeepsMinimum(t *testing.T) {
	s := openTemp(t)
	seedEntries(t, s, 10)

	// Open seeds one genesis entry, so there are 11 active entries total.
	// threshold 10, keep 3 => move 8 into the archive, leave 3 active.
	total := countAudit(t, s) // 11 (genesis + 10)
	const keep = 3
	wantMoved := total - keep
	b, err := s.Archive(context.Background(), 10, keep)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if b.ItemCount != wantMoved {
		t.Fatalf("item count = %d, want %d", b.ItemCount, wantMoved)
	}
	if got := countAudit(t, s); got != keep {
		t.Fatalf("active entries after archive = %d, want %d", got, keep)
	}
	if got := countArchiveEntries(t, s); got != wantMoved {
		t.Fatalf("archived entries = %d, want %d", got, wantMoved)
	}
	if got := countBatches(t, s); got != 1 {
		t.Fatalf("batches = %d, want 1", got)
	}
	// Batch bookkeeping must agree with the moved range (genesis + first seeds).
	if b.StartSeq != 1 {
		t.Fatalf("batch start seq = %d, want 1", b.StartSeq)
	}
	if b.EndSeq != int64(wantMoved) {
		t.Fatalf("batch end seq = %d, want %d", b.EndSeq, wantMoved)
	}
}

// TestSQLiteArchiveAlreadyCanceledDoesNotCommit asserts that when the context
// is already canceled before Archive runs, no partial state is written: the
// transaction is rolled back, so archive_entries / archive_batches stay empty
// and audit_entries are untouched. This is the core of the "rows/transaction
// release order" fix — a canceled scan must not bleed into a write.
func TestSQLiteArchiveAlreadyCanceledDoesNotCommit(t *testing.T) {
	s := openTemp(t)
	seedEntries(t, s, 20)
	total := countAudit(t, s) // 21 (genesis + 20)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	b, err := s.Archive(ctx, 10, 3)
	if err == nil {
		t.Fatalf("expected error from canceled archive, got nil (batch=%#v)", b)
	}
	// Nothing should have been committed.
	if got := countArchiveEntries(t, s); got != 0 {
		t.Fatalf("canceled archive wrote %d archive_entries, want 0", got)
	}
	if got := countBatches(t, s); got != 0 {
		t.Fatalf("canceled archive wrote %d batches, want 0", got)
	}
	if got := countAudit(t, s); got != total {
		t.Fatalf("canceled archive changed audit_entries: %d, want %d", got, total)
	}
}

// TestSQLiteArchiveNoopBelowThreshold verifies the early-out path releases all
// resources and records nothing when there is insufficient data to archive.
func TestSQLiteArchiveNoopBelowThreshold(t *testing.T) {
	s := openTemp(t)
	seedEntries(t, s, 5)
	total := countAudit(t, s) // 6 (genesis + 5)

	b, err := s.Archive(context.Background(), 100, 3)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if b.BatchNo != 0 {
		t.Fatalf("expected noop (zero batch), got batch_no=%d", b.BatchNo)
	}
	if got := countBatches(t, s); got != 0 {
		t.Fatalf("noop wrote %d batches, want 0", got)
	}
	if got := countAudit(t, s); got != total {
		t.Fatalf("noop changed audit_entries: %d, want %d", got, total)
	}
}

func openTemp(t *testing.T) *SQLite {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
