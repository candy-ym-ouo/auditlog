package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"fmt"
	"testing"
	"time"
)

type archiveDeadlineStore struct {
	store.Store
}

func (archiveDeadlineStore) Archive(ctx context.Context, _, _ int) (model.ArchiveBatch, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return model.ArchiveBatch{}, fmt.Errorf("archive context has no deadline")
	}
	if remaining := time.Until(deadline); remaining < 10*time.Second {
		return model.ArchiveBatch{}, fmt.Errorf("archive deadline is too short: %v", remaining)
	}
	return model.ArchiveBatch{}, nil
}

func TestBug008ArchiveAllowsLongRunningStoreOperation(t *testing.T) {
	err := NewArchive(archiveDeadlineStore{}, 1, 0, 30).Trigger(context.Background())
	if err != nil {
		t.Fatalf("archive rejected a long-running store operation: %v", err)
	}
}
