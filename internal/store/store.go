package store

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"auditlog/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	Close() error
	Append(context.Context, model.Entry) (model.Entry, error)
	Head(context.Context) (model.Head, error)
	Entries(context.Context, model.Query) (model.Page, error)
	AllEntries(context.Context) ([]model.Entry, error)
	EntryByID(context.Context, int64) (model.Entry, error)
	Stats(context.Context) (model.Stats, error)
	Archive(context.Context, int, int) (model.ArchiveBatch, error)
	Batches(context.Context, int, int) (model.BatchPage, error)
	Batch(context.Context, int64) (model.ArchiveExport, error)
	SetVerify(context.Context, model.VerifyReport) error
	LastVerify(context.Context) (time.Time, string, error)
}

type Memory struct {
	mu       sync.RWMutex
	entries  []model.Entry
	archives []model.Entry
	batches  []model.ArchiveBatch
	head     model.Head
}

func NewMemory() *Memory       { return &Memory{} }
func (m *Memory) Close() error { return nil }
func (m *Memory) Append(_ context.Context, e model.Entry) (model.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = int64(len(m.entries) + len(m.archives) + 1)
	m.entries = append(m.entries, e)
	m.head = model.Head{Seq: e.Seq, Hash: e.Hash, PrevHash: e.PrevHash, UpdatedAt: time.Now().UTC()}
	return e, nil
}
func (m *Memory) Head(_ context.Context) (model.Head, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.head, nil
}
func (m *Memory) AllEntries(_ context.Context) ([]model.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := append([]model.Entry{}, m.archives...)
	result = append(result, m.entries...)
	sort.Slice(result, func(i, j int) bool { return result[i].Seq < result[j].Seq })
	return result, nil
}
func (m *Memory) Entries(ctx context.Context, q model.Query) (model.Page, error) {
	all, _ := m.AllEntries(ctx)
	if !q.IncludeArchived {
		m.mu.RLock()
		all = append([]model.Entry{}, m.entries...)
		m.mu.RUnlock()
	}
	filtered := filter(all, q)
	return page(filtered, q), nil
}
func (m *Memory) EntryByID(_ context.Context, id int64) (model.Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, e := range append(append([]model.Entry{}, m.entries...), m.archives...) {
		if e.ID == id {
			return e, nil
		}
	}
	return model.Entry{}, ErrNotFound
}
func (m *Memory) Stats(_ context.Context) (model.Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return model.Stats{TotalEntries: int64(len(m.entries) + len(m.archives)), ArchivedEntries: int64(len(m.archives)), ActiveEntries: int64(len(m.entries)), HeadSeq: m.head.Seq, HeadHash: m.head.Hash, BatchCount: int64(len(m.batches))}, nil
}
func (m *Memory) Archive(_ context.Context, threshold, keep int) (model.ArchiveBatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.entries) - keep
	if n <= 0 || len(m.entries) < threshold {
		return model.ArchiveBatch{}, nil
	}
	moved := append([]model.Entry{}, m.entries[:n]...)
	m.entries = append([]model.Entry{}, m.entries[n:]...)
	b := model.ArchiveBatch{BatchNo: int64(len(m.batches) + 1), StartSeq: moved[0].Seq, EndSeq: moved[len(moved)-1].Seq, PrevHash: moved[0].PrevHash, HeadHash: moved[len(moved)-1].Hash, ItemCount: len(moved), ArchivedAt: time.Now().UTC()}
	m.archives = append(m.archives, moved...)
	m.batches = append(m.batches, b)
	return b, nil
}
func (m *Memory) Batches(_ context.Context, p, s int) (model.BatchPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return batchPage(m.batches, p, s), nil
}
func (m *Memory) Batch(_ context.Context, no int64) (model.ArchiveExport, error) {
	for _, b := range m.batches {
		if b.BatchNo == no {
			var es []model.Entry
			for _, e := range m.archives {
				if e.Seq >= b.StartSeq && e.Seq <= b.EndSeq {
					es = append(es, e)
				}
			}
			return model.ArchiveExport{Batch: b, Entries: es}, nil
		}
	}
	return model.ArchiveExport{}, ErrNotFound
}
func (m *Memory) SetVerify(_ context.Context, r model.VerifyReport) error { return nil }
func (m *Memory) LastVerify(_ context.Context) (time.Time, string, error) {
	return time.Time{}, "", nil
}

func filter(entries []model.Entry, q model.Query) []model.Entry {
	out := make([]model.Entry, 0)
	for _, e := range entries {
		if q.Actor != "" && e.Actor != q.Actor || q.Action != "" && e.Action != q.Action || q.Target != "" && !contains(e.Target, q.Target) || q.SeqFrom > 0 && e.Seq < q.SeqFrom || q.SeqTo > 0 && e.Seq > q.SeqTo || q.From != nil && e.EventTime.Before(*q.From) || q.To != nil && e.EventTime.After(*q.To) {
			continue
		}
		out = append(out, e)
	}
	return out
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// paginationBounds clamps page/size to sane defaults and returns the slice
// half-open interval [start, end) for a collection of length n without ever
// performing an overflowing multiplication. page/size are validated here so
// both the memory and SQLite stores share identical edge-case semantics.
//
// The offset (page-1)*size is computed in int64 to avoid the silent wraparound
// that happens when page is near MaxInt: (MaxInt-1)*200 underflows to a negative
// int, which then panics on slice indexing and corrupts SQLite OFFSET. If the
// offset cannot be safely represented or lands past the end of the data, an empty
// interval [n, n) is returned so callers yield zero items while still echoing the
// requested page/size and the true total.
func paginationBounds(page, size, n int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	page64 := int64(page)
	size64 := int64(size)
	// (page-1)*size overflows int64 only when page-1 exceeds maxInt64/size;
	// guard once before multiplying, then again when narrowing back to int.
	if page64-1 > maxInt64/size64 {
		return n, n
	}
	offset := (page64 - 1) * size64
	if offset < 0 || offset > maxInt64 {
		return n, n
	}
	start := int(offset)
	if start < 0 || start >= n {
		return n, n
	}
	end := start + size
	if end < 0 || end > n {
		end = n
	}
	return start, end
}

func page(entries []model.Entry, q model.Query) model.Page {
	start, end := paginationBounds(q.Page, q.PageSize, len(entries))
	page, size := clampPageArgs(q.Page, q.PageSize)
	return model.Page{Items: entries[start:end], Page: page, PageSize: size, Total: int64(len(entries))}
}

func batchPage(batches []model.ArchiveBatch, p, size int) model.BatchPage {
	start, end := paginationBounds(p, size, len(batches))
	page, s := clampPageArgs(p, size)
	return model.BatchPage{Items: append([]model.ArchiveBatch{}, batches[start:end]...), Page: page, PageSize: s, Total: int64(len(batches))}
}

// clampPageArgs mirrors the normalization paginationBounds applies to page/size
// so the echoed page/page_size fields stay consistent with the interval used.
func clampPageArgs(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}
