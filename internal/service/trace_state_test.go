package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"testing"
)

func TestBug009TraceContextOwnsChainPosition(t *testing.T) {
	m := store.NewMemory()
	a := NewAudit(m)
	for i := 0; i < 2; i++ {
		if _, err := a.Append(context.Background(), model.AppendRequest{Actor: "test", Action: "trace", Target: "state"}); err != nil {
			t.Fatal(err)
		}
	}
	tc := NewTrace(m)
	one, err := tc.Context(context.Background(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	two, err := tc.Context(context.Background(), 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	one.ChainPosition["request_id"] = "first"
	if two.ChainPosition["request_id"] != nil {
		t.Fatal("trace contexts share mutable chain position")
	}
}
