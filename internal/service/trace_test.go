package service

import (
	"context"
	"errors"
	"testing"

	"auditlog/internal/store"
)

// TestTraceContextNonexistentIDIsErrNotFound guards the store → TraceService →
// handler error chain. The original bug wrapped the store error with %v, which
// flattened ErrNotFound to a plain string and broke errors.Is at the HTTP
// layer, turning a 404 into a 500. The fix uses %w so the sentinel survives.
func TestTraceContextNonexistentIDIsErrNotFound(t *testing.T) {
	svc := NewTrace(store.NewMemory())
	_, err := svc.Context(context.Background(), 999999, 5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The bug was a %v wrap that severed this chain; %w must preserve it.
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, store.ErrNotFound), got %v", err)
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error message for triage")
	}
}
