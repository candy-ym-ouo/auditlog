package service

import (
	"auditlog/internal/chain"
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
	"errors"
	"time"
)

type AuditService struct{ Store store.Store }

func NewAudit(s store.Store) *AuditService { return &AuditService{Store: s} }
func (a *AuditService) Append(ctx context.Context, r model.AppendRequest) (model.Entry, error) {
	now := time.Now().UTC()
	if err := r.Normalize(now); err != nil {
		return model.Entry{}, err
	}
	h, err := a.Store.Head(ctx)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return model.Entry{}, err
	}
	prev := h.Hash
	if h.Seq == 0 {
		prev = chain.ZeroHash
	}
	e, err := chain.NewEntry(h.Seq+1, prev, r, now)
	if err != nil {
		return model.Entry{}, err
	}
	return a.Store.Append(ctx, e)
}
func (a *AuditService) Entries(c context.Context, q model.Query) (model.Page, error) {
	return a.Store.Entries(c, q)
}
func (a *AuditService) Stats(c context.Context) (model.Stats, error) { return a.Store.Stats(c) }
func (a *AuditService) Head(c context.Context) (model.Head, error)   { return a.Store.Head(c) }
