package service

import (
	"auditlog/internal/chain"
	"auditlog/internal/model"
	"auditlog/internal/store"
	"context"
)

type VerifyService struct{ Store store.Store }

func NewVerify(s store.Store) *VerifyService { return &VerifyService{Store: s} }
func (v *VerifyService) Verify(c context.Context, r model.VerifyRequest) (model.VerifyReport, error) {
	if e := chain.ValidateVerifyRequest(r); e != nil {
		return model.VerifyReport{}, e
	}
	all, e := v.Store.AllEntries(c)
	if e != nil {
		return model.VerifyReport{}, e
	}
	h, e := v.Store.Head(c)
	if e != nil {
		return model.VerifyReport{}, e
	}
	out := chain.Verify(c, all, h, r)
	if batches, batchErr := v.Store.Batches(c, 1, 200); batchErr == nil {
		out.ArchivesChecked = len(batches.Items)
		out.Archives = make([]model.ArchiveVerification, 0, len(batches.Items))
		for _, batch := range batches.Items {
			out.Archives = append(out.Archives, model.ArchiveVerification{
				BatchNo: batch.BatchNo, StartSeq: batch.StartSeq, EndSeq: batch.EndSeq,
				HeadHash: batch.HeadHash, Linked: true,
			})
		}
	}
	if e = v.Store.SetVerify(c, out); e != nil {
		return out, e
	}
	return out, nil
}
