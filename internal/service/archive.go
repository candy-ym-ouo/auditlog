package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"sync"
	"time"
)

type ArchiveService struct {
	Store                          store.Store
	Threshold, KeepMin, MaxAgeDays int
	mu                             sync.Mutex
	status                         model.ArchiveStatus
}

func NewArchive(s store.Store, t, k, a int) *ArchiveService {
	return &ArchiveService{Store: s, Threshold: t, KeepMin: k, MaxAgeDays: a, status: model.ArchiveStatus{Threshold: t, KeepMin: k, MaxAgeDays: a}}
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
	b, e := a.Store.Archive(c, a.Threshold, a.KeepMin)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status.Metadata["completed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	if e != nil {
		a.status.LastError = e.Error()
	} else if b.BatchNo > 0 {
		a.status.LastBatchNo = b.BatchNo
	}
	return e
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
