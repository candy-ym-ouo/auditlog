package server

import (
	"auditlog/internal/api"
	"auditlog/internal/service"
	"embed"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

type Server struct{ HTTP *http.Server }

func New(addr string, a *service.AuditService, ar *service.ArchiveService, t *service.TraceService, v *service.VerifyService, token string) *Server {
	h := api.New(a, ar, t, v, token).Handler()
	mux := http.NewServeMux()
	mux.Handle("/api/", h)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := "assets" + r.URL.Path
		if name == "assets/" {
			name = "assets/index.html"
		}
		f, e := assets.ReadFile(name)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(f)
	})
	return &Server{HTTP: &http.Server{Addr: addr, Handler: mux}}
}
