package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// spyStore lets a test observe how ArchiveService drives the store and inject a
// canned batch/error per call. It also supports gating Archive behind a release
// channel so re-entry can be exercised deterministically.
type spyStore struct {
	mu      sync.Mutex
	calls   int32
	batch   model.ArchiveBatch
	err     error
	started chan struct{}
	release chan struct{}
}

func newSpyStore() *spyStore { return &spyStore{} }

func (s *spyStore) set(b model.ArchiveBatch, e error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batch, s.err = b, e
}
func (s *spyStore) callCount() int { return int(atomic.LoadInt32(&s.calls)) }
func (s *spyStore) Close() error   { return nil }
func (s *spyStore) Append(context.Context, model.Entry) (model.Entry, error) {
	return model.Entry{}, nil
}
func (s *spyStore) Head(context.Context) (model.Head, error) { return model.Head{}, nil }
func (s *spyStore) Entries(context.Context, model.Query) (model.Page, error) {
	return model.Page{}, nil
}
func (s *spyStore) AllEntries(context.Context) ([]model.Entry, error) { return nil, nil }
func (s *spyStore) EntryByID(context.Context, int64) (model.Entry, error) {
	return model.Entry{}, store.ErrNotFound
}
func (s *spyStore) Stats(context.Context) (model.Stats, error) { return model.Stats{}, nil }
func (s *spyStore) Archive(_ context.Context, _, _ int) (model.ArchiveBatch, error) {
	atomic.AddInt32(&s.calls, 1)
	s.mu.Lock()
	b, e, started, release := s.batch, s.err, s.started, s.release
	s.mu.Unlock()
	if started != nil {
		// Signal that the run has entered the store, then block on release so a
		// concurrent Trigger observes Running=true.
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
	}
	return b, e
}
func (s *spyStore) Batches(context.Context, int, int) (model.BatchPage, error) {
	return model.BatchPage{}, nil
}
func (s *spyStore) Batch(context.Context, int64) (model.ArchiveExport, error) {
	return model.ArchiveExport{}, nil
}
func (s *spyStore) SetVerify(context.Context, model.VerifyReport) error { return nil }
func (s *spyStore) LastVerify(context.Context) (time.Time, string, error) {
	return time.Time{}, "", nil
}

func TestArchiveServiceRecordsOKOnBatch(t *testing.T) {
	spy := newSpyStore()
	spy.set(model.ArchiveBatch{BatchNo: 7, ItemCount: 3}, nil)
	svc := NewArchive(spy, 2, 1, 0)

	if err := svc.Trigger(context.Background()); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	st := svc.Status()
	if st.LastStatus != model.ArchiveStatusOK {
		t.Fatalf("status = %q, want %q", st.LastStatus, model.ArchiveStatusOK)
	}
	if st.LastBatchNo != 7 || st.LastError != "" {
		t.Fatalf("unexpected status: %#v", st)
	}
	if spy.callCount() != 1 {
		t.Fatalf("expected exactly one store call, got %d", spy.callCount())
	}
}

func TestArchiveServiceRecordsNoopWhenNothingToDo(t *testing.T) {
	spy := newSpyStore()
	// Zero BatchNo means "nothing archived" per the store contract.
	spy.set(model.ArchiveBatch{}, nil)
	svc := NewArchive(spy, 2, 1, 0)

	if err := svc.Trigger(context.Background()); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	st := svc.Status()
	if st.LastStatus != model.ArchiveStatusNoop {
		t.Fatalf("status = %q, want %q", st.LastStatus, model.ArchiveStatusNoop)
	}
	if st.LastError != "" || st.LastBatchNo != 0 {
		t.Fatalf("noop should not touch batch/error: %#v", st)
	}
}

func TestArchiveServiceRecordsErrorOnStoreFailure(t *testing.T) {
	spy := newSpyStore()
	spy.set(model.ArchiveBatch{}, errors.New("disk full"))
	svc := NewArchive(spy, 2, 1, 0)

	if err := svc.Trigger(context.Background()); err == nil {
		t.Fatal("expected error to propagate")
	}
	st := svc.Status()
	if st.LastStatus != model.ArchiveStatusError {
		t.Fatalf("status = %q, want %q", st.LastStatus, model.ArchiveStatusError)
	}
	if st.LastError != "disk full" {
		t.Fatalf("last error = %q", st.LastError)
	}
}

func TestArchiveServiceDistinguishesCancellationFromError(t *testing.T) {
	spy := newSpyStore()
	spy.set(model.ArchiveBatch{}, context.DeadlineExceeded)
	svc := NewArchive(spy, 2, 1, 0)

	if err := svc.Trigger(context.Background()); err == nil {
		t.Fatal("expected deadline error to propagate")
	}
	st := svc.Status()
	if st.LastStatus != model.ArchiveStatusCanceled {
		t.Fatalf("status = %q, want %q", st.LastStatus, model.ArchiveStatusCanceled)
	}
	// Cancellation must not clobber a previously recorded successful batch.
	if st.LastBatchNo != 0 {
		t.Fatalf("canceled run should leave LastBatchNo untouched: %#v", st)
	}
	if st.LastError == "" {
		t.Fatalf("canceled run should still surface the cause in LastError: %#v", st)
	}
}

// TestArchiveServiceCancellationPreservesPriorBatch asserts that a canceled
// run does not overwrite the LastBatchNo recorded by a prior successful run.
func TestArchiveServiceCancellationPreservesPriorBatch(t *testing.T) {
	spy := newSpyStore()
	svc := NewArchive(spy, 2, 1, 0)

	spy.set(model.ArchiveBatch{BatchNo: 5, ItemCount: 2}, nil)
	if err := svc.Trigger(context.Background()); err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	if got := svc.Status().LastBatchNo; got != 5 {
		t.Fatalf("first run batch = %d, want 5", got)
	}

	spy.set(model.ArchiveBatch{}, context.Canceled)
	if err := svc.Trigger(context.Background()); err == nil {
		t.Fatal("expected cancellation error to propagate")
	}
	st := svc.Status()
	if st.LastStatus != model.ArchiveStatusCanceled {
		t.Fatalf("status = %q, want %q", st.LastStatus, model.ArchiveStatusCanceled)
	}
	if st.LastBatchNo != 5 {
		t.Fatalf("cancellation clobbered prior batch: got %d, want 5", st.LastBatchNo)
	}
}

func TestArchiveServiceReentryGuarded(t *testing.T) {
	spy := newSpyStore()
	spy.started = make(chan struct{})
	spy.release = make(chan struct{})
	spy.set(model.ArchiveBatch{BatchNo: 1, ItemCount: 1}, nil)
	svc := NewArchive(spy, 2, 1, 0)

	done := make(chan error, 1)
	go func() { done <- svc.Trigger(context.Background()) }()

	// Wait until the in-flight run has entered the store, then fire a second
	// Trigger while the first is still blocked.
	select {
	case <-spy.started:
	case <-time.After(time.Second):
		t.Fatal("first trigger did not start within 1s")
	}
	if err := svc.Trigger(context.Background()); err != nil {
		t.Fatalf("concurrent trigger should be a no-op, got %v", err)
	}
	close(spy.release)
	if err := <-done; err != nil {
		t.Fatalf("first trigger: %v", err)
	}
	if got := spy.callCount(); got != 1 {
		t.Fatalf("store called %d times, want 1 (re-entry should be skipped)", got)
	}
	if svc.Status().Running {
		t.Fatal("Running flag should be clear once the run completes")
	}
}

func TestArchiveServiceTimeoutIsConfigurable(t *testing.T) {
	spy := newSpyStore()
	spy.set(model.ArchiveBatch{}, nil)
	svc := NewArchiveWithTimeout(spy, 2, 1, 0, 5*time.Second)
	if svc.Timeout != 5*time.Second {
		t.Fatalf("timeout = %v, want 5s", svc.Timeout)
	}
	// Zero/negative timeout must fall back to the default, never panic.
	svc0 := NewArchiveWithTimeout(spy, 2, 1, 0, 0)
	if svc0.Timeout != defaultArchiveTimeout {
		t.Fatalf("zero timeout should fall back to default, got %v", svc0.Timeout)
	}
}
