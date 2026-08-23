package api

import (
	"auditlog/internal/model"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (x *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "version": "1.0.0", "db": "ok", "time": time.Now().UTC()})
}
func (x *API) stats(w http.ResponseWriter, r *http.Request) {
	v, e := x.Audit.Stats(r.Context())
	if e != nil {
		writeError(w, 500, "internal_error", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (x *API) head(w http.ResponseWriter, r *http.Request) {
	v, e := x.Audit.Head(r.Context())
	if e != nil {
		writeError(w, 500, "internal_error", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (x *API) entries(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var q model.AppendRequest
		if !decodeSingleJSON(r, &q) {
			writeError(w, 400, "invalid_request", "invalid JSON")
			return
		}
		e, err := x.Audit.Append(r.Context(), q)
		if err != nil {
			writeError(w, 400, "invalid_request", err.Error())
			return
		}
		writeJSON(w, 201, e)
		return
	}
	z := r.URL.Query()
	includeArchived := true
	if z.Get("include_archived") == "false" {
		includeArchived = false
	}
	q := model.Query{Actor: z.Get("actor"), Action: z.Get("action"), Target: z.Get("target"), IncludeArchived: includeArchived, Page: atoi(z.Get("page"), 1), PageSize: atoi(z.Get("page_size"), 20), SeqFrom: int64(atoi(z.Get("seq_from"), 0)), SeqTo: int64(atoi(z.Get("seq_to"), 0))}
	p, e := x.Audit.Entries(r.Context(), q)
	if e != nil {
		writeError(w, 500, "internal_error", e.Error())
		return
	}
	writeJSON(w, 200, p)
}
func (x *API) context(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[3] == "" {
		writeError(w, 404, "not_found", "entry not found")
		return
	}
	id, e := strconv.ParseInt(parts[3], 10, 64)
	if e != nil {
		writeError(w, 400, "invalid_request", "invalid id")
		return
	}
	v, e := x.Trace.Context(r.Context(), id, atoi(r.URL.Query().Get("radius"), 5))
	if e != nil {
		writeError(w, 404, "not_found", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (x *API) verify(w http.ResponseWriter, r *http.Request) {
	var q model.VerifyRequest
	if r.Body != nil && !decodeSingleJSON(r, &q) {
		writeError(w, 400, "invalid_request", "invalid JSON request body")
		return
	}
	v, e := x.Verify.Verify(r.Context(), q)
	if e != nil {
		writeError(w, 400, "invalid_request", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (x *API) archive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "invalid_request", "method not allowed")
		return
	}
	go func(ctx context.Context) {
		<-ctx.Done()
		x.Archive.Trigger(ctx)
	}(r.Context())
	writeJSON(w, 200, map[string]any{"accepted": true, "note": "archive task enqueued"})
}
func (x *API) archiveStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, x.Archive.Status())
}
func (x *API) batches(w http.ResponseWriter, r *http.Request) {
	v, e := x.Archive.Batches(r.Context(), atoi(r.URL.Query().Get("page"), 1), atoi(r.URL.Query().Get("page_size"), 20))
	if e != nil {
		writeError(w, 500, "internal_error", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (x *API) batch(w http.ResponseWriter, r *http.Request) {
	no, e := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/archive/batches/"), 10, 64)
	if e != nil {
		writeError(w, 400, "invalid_request", "invalid batch number")
		return
	}
	v, e := x.Archive.Batch(r.Context(), no)
	if e != nil {
		writeError(w, 404, "not_found", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func (x *API) report(w http.ResponseWriter, r *http.Request) {
	v, e := x.Trace.Report(r.Context(), model.Query{Actor: r.URL.Query().Get("actor"), Page: 1, PageSize: 200})
	if e != nil {
		writeError(w, 500, "internal_error", e.Error())
		return
	}
	writeJSON(w, 200, v)
}
func atoi(s string, d int) int {
	if n, e := strconv.Atoi(s); e == nil && n > 0 {
		return n
	}
	return d
}

func decodeSingleJSON(r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if err == io.EOF {
			return true
		}
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}
