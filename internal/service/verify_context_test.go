package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"testing"
)

func TestBug007VerifyPropagatesCancellation(t *testing.T) {
	m := store.NewMemory()
	if _, err := NewAudit(m).Append(context.Background(), model.AppendRequest{Actor: "test", Action: "verify", Target: "cancel"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := NewVerify(m).Verify(ctx, model.VerifyRequest{Mode: "full"})
	if err != nil {
		t.Fatal(err)
	}
	if report.CheckedEntries != 0 {
		t.Fatalf("canceled verify checked %d entries", report.CheckedEntries)
	}
}
