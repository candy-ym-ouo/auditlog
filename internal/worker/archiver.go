package worker

import (
	"auditlog/internal/service"
	"context"
)

type Archiver struct{ Service *service.ArchiveService }

func (a *Archiver) Run(ctx context.Context) { _ = a.Service.Trigger(ctx) }
