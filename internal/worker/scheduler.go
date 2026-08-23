package worker

import (
	"auditlog/internal/service"
	"context"
	"time"
)

func Start(ctx context.Context, s *service.ArchiveService, interval time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				s.Trigger(ctx)
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}
