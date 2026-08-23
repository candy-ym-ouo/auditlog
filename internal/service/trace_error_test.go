package service

import (
	"auditlog/internal/store"
	"context"
	"errors"
	"testing"
)

func TestBug006MissingEntryPreservesNotFound(t *testing.T) {
	_, err := NewTrace(store.NewMemory()).Context(context.Background(), 999, 5)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing entry error = %v, want errors.Is(ErrNotFound)", err)
	}
}
