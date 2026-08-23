package store

import (
	"math"
	"testing"

	"auditlog/internal/model"
)

func TestBug005MemoryPaginationDoesNotPanicOnOverflow(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("oversized page panicked: %v", r)
		}
	}()
	got := page([]model.Entry{{ID: 1}}, model.Query{Page: math.MaxInt, PageSize: 200})
	if len(got.Items) != 0 {
		t.Fatalf("got %d items for an out-of-range page", len(got.Items))
	}
}
