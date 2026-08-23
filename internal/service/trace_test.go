package service

import (
	"context"
	"sync"
	"testing"

	"auditlog/internal/model"
	"auditlog/internal/store"
)

// seedMemory fills a store with a small ordered chain so Context can return a
// populated ChainPosition map rather than the zero-value TraceContext.
func seedMemory(t *testing.T) *store.Memory {
	t.Helper()
	m := store.NewMemory()
	for i := 1; i <= 5; i++ {
		if _, err := m.Append(context.Background(), model.Entry{
			Seq: int64(i), Hash: "h", PrevHash: "p", Actor: "a", Action: "act",
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	return m
}

// TestContextConcurrentRace exercises Context under the race detector with
// many goroutines hitting the shared *TraceService at once. Before the fix the
// service stored a single mutable map and mutated it per call, so this test
// failed under -race with concurrent map writes and produced cross-request
// field bleed.
func TestContextConcurrentRace(t *testing.T) {
	svc := NewTrace(seedMemory(t))

	const n = 60
	var wg sync.WaitGroup
	for g := 0; g < n; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, err := svc.Context(context.Background(), 3, 2)
			if err != nil {
				t.Errorf("context: %v", err)
				return
			}
			// Each goroutine independently mutates the returned map, mimicking
			// how the handler injects request-scoped fields. With a shared map
			// this write would race against every other goroutine's write and
			// corrupt their responses.
			ctx.ChainPosition["request_id"] = "leak-check"
			if got := ctx.ChainPosition["request_id"]; got != "leak-check" {
				t.Errorf("request_id bled: got %v", got)
			}
		}()
	}
	wg.Wait()
}

// TestContextChainPositionIsRequestOwned verifies that the ChainPosition map
// returned to one caller is not shared with another caller's response. The
// previous implementation returned the service's single shared map, so a field
// written by one caller (e.g. request_id) would surface in every other
// caller's response.
func TestContextChainPositionIsRequestOwned(t *testing.T) {
	svc := NewTrace(seedMemory(t))

	ctx1, err := svc.Context(context.Background(), 3, 2)
	if err != nil {
		t.Fatalf("context1: %v", err)
	}
	ctx2, err := svc.Context(context.Background(), 3, 2)
	if err != nil {
		t.Fatalf("context2: %v", err)
	}

	// Inject a distinct request-scoped field into ctx1 only.
	ctx1.ChainPosition["request_id"] = "REQ-1"

	if _, present := ctx2.ChainPosition["request_id"]; present {
		t.Fatalf("ctx1's request_id leaked into ctx2's ChainPosition: %#v", ctx2.ChainPosition)
	}
	if ctx1.ChainPosition["request_id"] != "REQ-1" {
		t.Fatalf("ctx1 request_id not set: %#v", ctx1.ChainPosition)
	}
	// The service-populated fields must survive the caller's mutation on ctx1.
	if ctx1.ChainPosition["seq"] == nil {
		t.Fatalf("ctx1 service fields lost after request_id injection: %#v", ctx1.ChainPosition)
	}
}
