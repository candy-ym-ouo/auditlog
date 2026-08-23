package worker

import (
	"auditlog/internal/service"
	"context"
	"time"
)

func Start(ctx context.Context, s *service.ArchiveService, interval time.Duration) func() {
	stop := make(chan struct{})
	results := make(chan error)
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				go func() {
					results <- s.Trigger(ctx)
				}()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}
