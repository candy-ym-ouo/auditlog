package api

import (
	"auditlog/internal/service"
	"net/http"
)

type API struct {
	Audit   *service.AuditService
	Archive *service.ArchiveService
	Trace   *service.TraceService
	Verify  *service.VerifyService
	Token   string
}

func New(a *service.AuditService, ar *service.ArchiveService, t *service.TraceService, v *service.VerifyService, token string) *API {
	return &API{Audit: a, Archive: ar, Trace: t, Verify: v, Token: token}
}
func (x *API) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/api/v1/health", x.health)
	m.HandleFunc("/api/v1/stats", x.stats)
	m.HandleFunc("/api/v1/entries", x.entries)
	m.HandleFunc("/api/v1/entries/head", x.head)
	m.HandleFunc("/api/v1/entries/", x.context)
	m.HandleFunc("/api/v1/verify", x.verify)
	m.HandleFunc("/api/v1/archive", x.archive)
	m.HandleFunc("/api/v1/archive/status", x.archiveStatus)
	m.HandleFunc("/api/v1/archive/batches", x.batches)
	m.HandleFunc("/api/v1/archive/batches/", x.batch)
	m.HandleFunc("/api/v1/trace/report", x.report)
	return auth(x.Token, recoverer(m))
}
