package store

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"auditlog/internal/chain"
	"auditlog/internal/model"
)

func TestPageHandlesOversizedPageWithoutOverflow(t *testing.T) {
	entries := []model.Entry{{ID: 1}, {ID: 2}}
	got := page(entries, model.Query{Page: math.MaxInt, PageSize: 200})
	if got.Page != math.MaxInt || got.Total != 2 || len(got.Items) != 0 {
		t.Fatalf("unexpected oversized page result: %#v", got)
	}
}

func TestPageNormalAndEmptyCasesStayCompatible(t *testing.T) {
	entries := []model.Entry{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}}

	// Normal middle page.
	got := page(entries, model.Query{Page: 2, PageSize: 2})
	if got.Page != 2 || got.PageSize != 2 || got.Total != 5 || len(got.Items) != 2 || got.Items[0].ID != 3 {
		t.Fatalf("unexpected normal page: %#v", got)
	}

	// Last partial page.
	got = page(entries, model.Query{Page: 3, PageSize: 2})
	if len(got.Items) != 1 || got.Items[0].ID != 5 {
		t.Fatalf("unexpected partial last page: %#v", got)
	}

	// Page just past the end yields an empty page, not a panic.
	got = page(entries, model.Query{Page: 6, PageSize: 1})
	if len(got.Items) != 0 || got.Total != 5 || got.Page != 6 {
		t.Fatalf("unexpected past-end page: %#v", got)
	}

	// Defaults applied for zero/negative inputs.
	got = page(entries, model.Query{Page: 0, PageSize: 0})
	if got.Page != 1 || got.PageSize != 20 || len(got.Items) != 5 {
		t.Fatalf("unexpected defaulted page: %#v", got)
	}

	// Huge page_size is clamped, huge page is empty without overflow.
	got = page(entries, model.Query{Page: math.MaxInt32, PageSize: math.MaxInt})
	if got.PageSize != 200 || len(got.Items) != 0 {
		t.Fatalf("unexpected clamped oversized result: %#v", got)
	}
}

func TestBatchPageHandlesOversizedWithoutOverflow(t *testing.T) {
	batches := []model.ArchiveBatch{{BatchNo: 1}, {BatchNo: 2}, {BatchNo: 3}}
	got := batchPage(batches, math.MaxInt, 999)
	if got.PageSize != 200 || len(got.Items) != 0 || got.Total != 3 {
		t.Fatalf("unexpected bounded oversized batch page: %#v", got)
	}
}

func TestMemoryBatchesArePagedAndBounded(t *testing.T) {
	m := NewMemory()
	m.batches = []model.ArchiveBatch{{BatchNo: 1}, {BatchNo: 2}, {BatchNo: 3}}

	got, err := m.Batches(context.Background(), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 3 || got.Page != 2 || got.PageSize != 2 || len(got.Items) != 1 || got.Items[0].BatchNo != 3 {
		t.Fatalf("unexpected batch page: %#v", got)
	}

	got, err = m.Batches(context.Background(), math.MaxInt, 999)
	if err != nil {
		t.Fatal(err)
	}
	if got.PageSize != 200 || len(got.Items) != 0 {
		t.Fatalf("unexpected bounded oversized batch page: %#v", got)
	}
}

// newSQLiteForTest opens a fresh SQLite store on a temp file, seeds a couple of
// archive batches, and returns it (plus a cleanup func).
func newSQLiteForBatches(t *testing.T) (*SQLite, func()) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "audit.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 1; i <= 2; i++ {
		e := model.Entry{Seq: int64(i), PrevHash: chain.ZeroHash, Actor: "system", Action: "archive", Target: "t", Detail: []byte("{}"), EventTime: now}
		h, err := chain.HashEntry(e)
		if err != nil {
			s.Close()
			t.Fatalf("hash entry: %v", err)
		}
		e.Hash = h
		// Insert directly into archive tables to back the batches listing.
		_, execErr := s.db.ExecContext(ctx, `INSERT INTO archive_entries(id,seq,prev_hash,hash,actor,action,target,detail,event_time,batch_no) VALUES(?,?,?,?,?,?,?,?,?,?)`, int64(i), e.Seq, e.PrevHash, e.Hash, e.Actor, e.Action, e.Target, string(e.Detail), e.EventTime.Format(time.RFC3339Nano), int64(i))
		if execErr != nil {
			s.Close()
			t.Fatalf("seed archive entry: %v", execErr)
		}
		_, execErr = s.db.ExecContext(ctx, `INSERT INTO archive_batches VALUES(?,?,?,?,?,?,?,?)`, int64(i), int64(i), int64(i), chain.ZeroHash, e.Hash, 1, "payload", e.EventTime.Format(time.RFC3339Nano))
		if execErr != nil {
			s.Close()
			t.Fatalf("seed archive batch: %v", execErr)
		}
	}
	return s, func() { s.Close() }
}

func TestSQLiteBatchesRejectsOverflowingOffset(t *testing.T) {
	s, cleanup := newSQLiteForBatches(t)
	defer cleanup()

	// A page near MaxInt would, without the guard, make (p-1)*size wrap to a
	// negative int and be handed to SQLite as a negative OFFSET, which modern
	// SQLite rejects. The fix must return an empty page with no error.
	got, err := s.Batches(context.Background(), math.MaxInt, 999)
	if err != nil {
		t.Fatalf("batches with overflowing page: %v", err)
	}
	if got.PageSize != 200 || len(got.Items) != 0 || got.Total != 2 {
		t.Fatalf("unexpected sqlite oversized batch page: %#v", got)
	}

	// Normal paging still works.
	got, err = s.Batches(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("batches normal page: %v", err)
	}
	if got.Total != 2 || got.Page != 1 || len(got.Items) != 2 {
		t.Fatalf("unexpected sqlite normal batch page: %#v", got)
	}

	// Page past the end returns empty without error.
	got, err = s.Batches(context.Background(), 1000, 10)
	if err != nil {
		t.Fatalf("batches past-end page: %v", err)
	}
	if len(got.Items) != 0 || got.Total != 2 || got.Page != 1000 {
		t.Fatalf("unexpected past-end batch page: %#v", got)
	}
}
