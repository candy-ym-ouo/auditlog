package main

import (
	"auditlog/internal/config"
	"auditlog/internal/server"
	"auditlog/internal/service"
	"auditlog/internal/store"
	"auditlog/internal/worker"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, e := config.Load(os.Args[1:])
	if e != nil {
		log.Fatal(e)
	}
	db, e := store.Open(cfg.DatabasePath)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	audit := service.NewAudit(db)
	archive := service.NewArchive(db, cfg.ArchiveThreshold, cfg.ArchiveKeepMin, cfg.ArchiveMaxAgeDays)
	trace := service.NewTrace(db)
	verify := service.NewVerify(db)
	srv := server.New(cfg.Addr, audit, archive, trace, verify, cfg.Token)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	stop := worker.Start(ctx, archive, cfg.ArchiveInterval)
	defer stop()
	go func() {
		log.Printf("auditlog listening on %s", cfg.Addr)
		if e := srv.HTTP.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	<-ctx.Done()
	srv.HTTP.Shutdown(context.Background())
}
