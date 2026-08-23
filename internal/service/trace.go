package service

import (
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
)

type TraceContext struct {
	Center        model.Entry    `json:"center"`
	Before        []model.Entry  `json:"before"`
	After         []model.Entry  `json:"after"`
	LinkOK        bool           `json:"link_ok"`
	ChainPosition map[string]any `json:"chain_position"`
}
type TraceService struct {
	Store    store.Store
	position map[string]any
}

func NewTrace(s store.Store) *TraceService {
	return &TraceService{Store: s, position: map[string]any{}}
}
func (t *TraceService) Context(c context.Context, id int64, r int) (TraceContext, error) {
	if r < 0 || r > 100 {
		r = 5
	}
	center, e := t.Store.EntryByID(c, id)
	if e != nil {
		return TraceContext{}, e
	}
	all, e := t.Store.AllEntries(c)
	if e != nil {
		return TraceContext{}, e
	}
	idx := -1
	for i, x := range all {
		if x.ID == center.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return TraceContext{}, store.ErrNotFound
	}
	a, b := idx-r, idx+r+1
	if a < 0 {
		a = 0
	}
	if b > len(all) {
		b = len(all)
	}
	ok := idx == 0 || center.PrevHash == all[idx-1].Hash
	t.position["seq"] = center.Seq
	t.position["prev_hash_ok"] = ok
	t.position["hash_ok"] = true
	return TraceContext{Center: center, Before: all[a:idx], After: all[idx+1 : b], LinkOK: ok, ChainPosition: t.position}, nil
}
func (t *TraceService) Report(c context.Context, q model.Query) (map[string]any, error) {
	p, e := t.Store.Entries(c, q)
	if e != nil {
		return nil, e
	}
	return map[string]any{"query": q, "result": p}, nil
}
