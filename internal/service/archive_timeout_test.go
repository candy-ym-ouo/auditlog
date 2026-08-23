package service

import (
	"auditlog/internal/store"
	"context"
	"errors"
	"testing"
)

func TestBug008ArchiveHonorsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = NewArchive(db, 1, 0, 30).Trigger(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("archive error = %v, want context.Canceled", err)
	}
}
