package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"auditlog/internal/chain"
	"auditlog/internal/model"
	_ "modernc.org/sqlite"
)

type SQLite struct {
	db      *sql.DB
	writeMu sync.Mutex
}

func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;`); err != nil {
		db.Close()
		return nil, err
	}
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	s := &SQLite{db: db}
	if err = s.ensureGenesis(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *SQLite) Close() error { return s.db.Close() }
func (s *SQLite) ensureGenesis(ctx context.Context) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_entries`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now().UTC()
	detail, _ := json.Marshal(map[string]any{})
	e := model.Entry{Seq: 1, PrevHash: chain.ZeroHash, Actor: "system", Action: "genesis", Target: "audit_chain", Detail: detail, EventTime: now}
	h, err := chain.HashEntry(e)
	if err != nil {
		return err
	}
	e.Hash = h
	_, err = s.db.ExecContext(ctx, `INSERT INTO audit_entries(seq,prev_hash,hash,actor,action,target,detail,event_time) VALUES(?,?,?,?,?,?,?,?)`, e.Seq, e.PrevHash, e.Hash, e.Actor, e.Action, e.Target, detail, now.Format(time.RFC3339Nano))
	return err
}
func (s *SQLite) Append(ctx context.Context, e model.Entry) (model.Entry, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return e, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO audit_entries(seq,prev_hash,hash,actor,action,target,detail,event_time) VALUES(?,?,?,?,?,?,?,?)`, e.Seq, e.PrevHash, e.Hash, e.Actor, e.Action, e.Target, string(e.Detail), e.EventTime.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return e, err
	}
	e.ID, _ = res.LastInsertId()
	if err = tx.Commit(); err != nil {
		return e, err
	}
	return e, nil
}
func (s *SQLite) Head(ctx context.Context) (model.Head, error) {
	var h model.Head
	var t string
	err := s.db.QueryRowContext(ctx, `SELECT seq,hash,prev_hash,event_time FROM audit_entries ORDER BY seq DESC LIMIT 1`).Scan(&h.Seq, &h.Hash, &h.PrevHash, &t)
	if err != nil {
		return h, err
	}
	h.UpdatedAt, _ = time.Parse(time.RFC3339Nano, t)
	return h, nil
}
func (s *SQLite) AllEntries(ctx context.Context) ([]model.Entry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM audit_entries UNION ALL SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM archive_entries ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *SQLite) Entries(ctx context.Context, q model.Query) (model.Page, error) {
	all, err := s.AllEntries(ctx)
	if err != nil {
		return model.Page{}, err
	}
	if !q.IncludeArchived {
		all = all[:0]
		rows, queryErr := s.db.QueryContext(ctx, `SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM audit_entries ORDER BY seq`)
		if queryErr != nil {
			return model.Page{}, queryErr
		}
		defer rows.Close()
		for rows.Next() {
			e, scanErr := scanEntry(rows)
			if scanErr != nil {
				return model.Page{}, scanErr
			}
			all = append(all, e)
		}
		if err := rows.Err(); err != nil {
			return model.Page{}, err
		}
	}
	return page(filter(all, q), q), nil
}
func (s *SQLite) EntryByID(ctx context.Context, id int64) (model.Entry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM audit_entries WHERE id=? UNION ALL SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM archive_entries WHERE id=?`, id, id)
	return scanEntry(row)
}
func (s *SQLite) Stats(ctx context.Context) (model.Stats, error) {
	var x model.Stats
	err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM audit_entries)+(SELECT COUNT(*) FROM archive_entries),(SELECT COUNT(*) FROM archive_entries),(SELECT COUNT(*) FROM audit_entries),(SELECT COUNT(*) FROM archive_batches)`).Scan(&x.TotalEntries, &x.ArchivedEntries, &x.ActiveEntries, &x.BatchCount)
	if err != nil {
		return x, err
	}
	h, err := s.Head(ctx)
	if err == nil {
		x.HeadSeq = h.Seq
		x.HeadHash = h.Hash
	}
	x.LastVerifyAt, x.LastVerifyStatus, _ = s.LastVerify(ctx)
	return x, nil
}
func (s *SQLite) Archive(ctx context.Context, threshold, keep int) (model.ArchiveBatch, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_entries`).Scan(&count); err != nil {
		return model.ArchiveBatch{}, err
	}
	if count < threshold || count <= keep {
		return model.ArchiveBatch{}, nil
	}
	limit := count - keep
	rows, err := s.db.QueryContext(ctx, `SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM audit_entries ORDER BY seq LIMIT ?`, limit)
	if err != nil {
		return model.ArchiveBatch{}, err
	}
	// Fully materialize the rows before touching a transaction. With
	// SetMaxOpenConns(1) a held read cursor would contend with the subsequent
	// write transaction on the single connection, and if the deadline fires
	// mid-scan rows.Err() must be consulted — otherwise a truncated result is
	// silently treated as "nothing to archive" and the write proceeds on a
	// canceled context. Close the cursor before BeginTx so the read side is
	// released before the write side starts.
	es, scanErr := scanEntries(rows)
	if scanErr != nil {
		return model.ArchiveBatch{}, scanErr
	}
	if len(es) == 0 {
		return model.ArchiveBatch{}, nil
	}
	payload, _ := json.Marshal(es)
	b := model.ArchiveBatch{StartSeq: es[0].Seq, EndSeq: es[len(es)-1].Seq, PrevHash: es[0].PrevHash, HeadHash: es[len(es)-1].Hash, ItemCount: len(es), PayloadHash: fmt.Sprintf("%x", payload), ArchivedAt: time.Now().UTC()}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(batch_no),0)+1 FROM archive_batches`).Scan(&b.BatchNo); err != nil {
		return b, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return b, err
	}
	// Safety net so every return path — including context cancellation
	// mid-loop — rolls back the uncommitted transaction. Commit flips
	// committed to true and Rollback is a benign no-op (returns
	// sql.ErrTxDone) after a successful commit.
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()
	for _, e := range es {
		if _, err = tx.ExecContext(ctx, `INSERT INTO archive_entries(id,seq,prev_hash,hash,actor,action,target,detail,event_time,batch_no) VALUES(?,?,?,?,?,?,?,?,?,?)`, e.ID, e.Seq, e.PrevHash, e.Hash, e.Actor, e.Action, e.Target, string(e.Detail), e.EventTime.Format(time.RFC3339Nano), b.BatchNo); err != nil {
			// A canceled/deadlined context makes every subsequent Exec fail
			// immediately; stop looping and let the deferred Rollback clean up.
			return b, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO archive_batches VALUES(?,?,?,?,?,?,?,?)`, b.BatchNo, b.StartSeq, b.EndSeq, b.PrevHash, b.HeadHash, b.ItemCount, b.PayloadHash, b.ArchivedAt.Format(time.RFC3339Nano)); err != nil {
		return b, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM audit_entries WHERE seq<=?`, b.EndSeq); err != nil {
		return b, err
	}
	if err = tx.Commit(); err != nil {
		return b, err
	}
	committed = true
	return b, nil
}

// scanEntries drains a rows cursor into a slice, closing the cursor and
// surfacing a mid-iteration error (including context cancellation) via
// rows.Err(). It exists so Archive never opens a write transaction while a
// read cursor is still live or mistakes a truncated scan for an empty result.
func scanEntries(rows *sql.Rows) ([]model.Entry, error) {
	defer rows.Close()
	var out []model.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *SQLite) Batches(ctx context.Context, p, size int) (model.BatchPage, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM archive_batches`).Scan(&total); err != nil {
		return model.BatchPage{}, err
	}
	if p < 1 {
		p = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	maxPage := (total / int64(size)) + 1
	if int64(p) > maxPage {
		return model.BatchPage{Items: []model.ArchiveBatch{}, Page: p, PageSize: size, Total: total}, nil
	}
	offset := (int64(p) - 1) * int64(size)
	rows, err := s.db.QueryContext(ctx, `SELECT batch_no,start_seq,end_seq,prev_hash,head_hash,item_count,payload_hash,archived_at FROM archive_batches ORDER BY batch_no DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return model.BatchPage{}, err
	}
	defer rows.Close()
	var out []model.ArchiveBatch
	for rows.Next() {
		var b model.ArchiveBatch
		var t string
		if err := rows.Scan(&b.BatchNo, &b.StartSeq, &b.EndSeq, &b.PrevHash, &b.HeadHash, &b.ItemCount, &b.PayloadHash, &t); err != nil {
			return model.BatchPage{}, err
		}
		b.ArchivedAt, _ = time.Parse(time.RFC3339Nano, t)
		out = append(out, b)
	}
	return model.BatchPage{Items: out, Page: p, PageSize: size, Total: total}, nil
}
func (s *SQLite) Batch(ctx context.Context, no int64) (model.ArchiveExport, error) {
	var b model.ArchiveBatch
	var t string
	if err := s.db.QueryRowContext(ctx, `SELECT batch_no,start_seq,end_seq,prev_hash,head_hash,item_count,payload_hash,archived_at FROM archive_batches WHERE batch_no=?`, no).Scan(&b.BatchNo, &b.StartSeq, &b.EndSeq, &b.PrevHash, &b.HeadHash, &b.ItemCount, &b.PayloadHash, &t); err != nil {
		return model.ArchiveExport{}, ErrNotFound
	}
	b.ArchivedAt, _ = time.Parse(time.RFC3339Nano, t)
	rows, err := s.db.QueryContext(ctx, `SELECT id,seq,prev_hash,hash,actor,action,target,detail,event_time FROM archive_entries WHERE batch_no=? ORDER BY seq`, no)
	if err != nil {
		return model.ArchiveExport{}, err
	}
	defer rows.Close()
	var es []model.Entry
	for rows.Next() {
		e, eerr := scanEntry(rows)
		if eerr != nil {
			return model.ArchiveExport{}, eerr
		}
		es = append(es, e)
	}
	return model.ArchiveExport{Batch: b, Entries: es}, rows.Err()
}
func (s *SQLite) SetVerify(ctx context.Context, r model.VerifyReport) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(key,value) VALUES('last_verify',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fmt.Sprintf(`{"at":%q,"status":%q}`, r.VerifiedAt.Format(time.RFC3339Nano), r.Status))
	return err
}
func (s *SQLite) LastVerify(ctx context.Context) (time.Time, string, error) {
	var v string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key='last_verify'`).Scan(&v); err != nil {
		return time.Time{}, "", nil
	}
	var x struct {
		At     string `json:"at"`
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(v), &x) != nil {
		return time.Time{}, "", nil
	}
	t, _ := time.Parse(time.RFC3339Nano, x.At)
	return t, x.Status, nil
}
func scanEntry(s interface{ Scan(...any) error }) (model.Entry, error) {
	var e model.Entry
	var detail, tm string
	if err := s.Scan(&e.ID, &e.Seq, &e.PrevHash, &e.Hash, &e.Actor, &e.Action, &e.Target, &detail, &tm); err != nil {
		return e, err
	}
	e.Detail = json.RawMessage(detail)
	e.EventTime, _ = time.Parse(time.RFC3339Nano, tm)
	return e, nil
}

var _ = sort.Slice
