package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"errors"
	"sync"
	"time"
)

// defaultArchiveTimeout bounds a single archive run when no explicit timeout is
// configured. It is intentionally generous: archiving large active tables is a
// read-all-then-write-batch operation, and the old hard-coded 2s deadline was
// the root cause of the recurring "deadline exceeded" failures. Callers that
// need a tighter bound pass ArchiveTimeout via NewArchiveWithTimeout.
const defaultArchiveTimeout = 30 * time.Second

type ArchiveService struct {
	Store                          store.Store
	Threshold, KeepMin, MaxAgeDays int
	Timeout                        time.Duration
	mu                             sync.Mutex
	status                         model.ArchiveStatus
}

func NewArchive(s store.Store, t, k, a int) *ArchiveService {
	return NewArchiveWithTimeout(s, t, k, a, defaultArchiveTimeout)
}

func NewArchiveWithTimeout(s store.Store, t, k, a int, timeout time.Duration) *ArchiveService {
	if timeout <= 0 {
		timeout = defaultArchiveTimeout
	}
	return &ArchiveService{Store: s, Threshold: t, KeepMin: k, MaxAgeDays: a, Timeout: timeout, status: model.ArchiveStatus{Threshold: t, KeepMin: k, MaxAgeDays: a}}
}

func (a *ArchiveService) Trigger(c context.Context) error {
	a.mu.Lock()
	if a.status.Running {
		a.mu.Unlock()
		return nil
	}
	a.status.Running = true
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.status.Running = false; a.status.LastRunAt = time.Now().UTC(); a.mu.Unlock() }()

	// The run context is intentionally decoupled from the caller's context for
	// its timeout: HTTP triggers pass context.Background() and the scheduler
	// passes a root context that is only canceled on shutdown. We layer a
	// deadline on top so a single archive run cannot run forever, while still
	// honoring caller cancellation (shutdown) by deriving from c.
	runCtx, cancel := context.WithTimeout(c, a.Timeout)
	defer cancel()

	b, err := a.Store.Archive(runCtx, a.Threshold, a.KeepMin)

	a.mu.Lock()
	defer a.mu.Unlock()

	switch {
	case err == nil:
		a.status.LastError = ""
		if b.BatchNo > 0 {
			a.status.LastBatchNo = b.BatchNo
			a.status.LastStatus = model.ArchiveStatusOK
		} else {
			a.status.LastStatus = model.ArchiveStatusNoop
		}
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		// Cancellation is expected on shutdown or when the run exceeds its
		// budget. Record it distinctly from store errors so operators can tell
		// a policy stop apart from a real failure; leave LastBatchNo untouched
		// since no batch was committed.
		a.status.LastStatus = model.ArchiveStatusCanceled
		a.status.LastError = err.Error()
	default:
		a.status.LastStatus = model.ArchiveStatusError
		a.status.LastError = err.Error()
	}
	return err
}
func (a *ArchiveService) Status() model.ArchiveStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status
}
func (a *ArchiveService) Batches(c context.Context, p, s int) (model.BatchPage, error) {
	return a.Store.Batches(c, p, s)
}
func (a *ArchiveService) Batch(c context.Context, n int64) (model.ArchiveExport, error) {
	return a.Store.Batch(c, n)
}
