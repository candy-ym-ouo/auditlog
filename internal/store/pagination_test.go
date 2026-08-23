package store

import (
	"context"
	"math"
	"testing"

	"auditlog/internal/model"
)

func TestPageHandlesOversizedPageWithoutOverflow(t *testing.T) {
	entries := []model.Entry{{ID: 1}, {ID: 2}}
	got := page(entries, model.Query{Page: math.MaxInt, PageSize: 200})
	if got.Page != math.MaxInt || got.Total != 2 || len(got.Items) != 0 {
		t.Fatalf("unexpected oversized page result: %#v", got)
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
