package chain

import (
	"context"
	"fmt"
	"time"

	"auditlog/internal/model"
)

func Verify(ctx context.Context, entries []model.Entry, head model.Head, request model.VerifyRequest) model.VerifyReport {
	started := time.Now()
	report := model.VerifyReport{Mode: request.Mode, Status: model.VerifyOK, HeadHash: head.Hash, VerifiedAt: started.UTC(), Archives: []model.ArchiveVerification{}}
	if report.Mode == "" {
		report.Mode = "full"
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			report.DurationMS = time.Since(started).Milliseconds()
			return report
		}
	}
	selected := selectEntries(entries, request)
	if len(selected) == 0 {
		report.DurationMS = time.Since(started).Milliseconds()
		return report
	}
	report.StartSeq, report.EndSeq = selected[0].Seq, selected[len(selected)-1].Seq
	for i, entry := range selected {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				break
			}
		}
		report.CheckedEntries++
		expectedPrev := entry.PrevHash
		if entry.Seq == 1 {
			expectedPrev = ZeroHash
		} else if index := findSeq(entries, entry.Seq-1); index >= 0 {
			expectedPrev = entries[index].Hash
		}
		if entry.PrevHash != expectedPrev {
			fail(&report, model.VerifyLinkBroken, entry.Seq, "prev_hash does not reference preceding entry")
			break
		}
		hash, err := HashEntry(entry)
		if err != nil || hash != entry.Hash {
			reason := "stored hash differs from recomputed hash"
			if err != nil {
				reason = err.Error()
			}
			fail(&report, model.VerifyHashMismatch, entry.Seq, reason)
			break
		}
		_ = i
	}
	if report.Status == model.VerifyOK && (request.Mode == "" || request.Mode == "full" || request.Mode == "head") {
		if len(entries) > 0 && entries[len(entries)-1].Hash != head.Hash {
			fail(&report, model.VerifyHeadMismatch, entries[len(entries)-1].Seq, "chain tail differs from stored head")
		}
	}
	report.DurationMS = time.Since(started).Milliseconds()
	return report
}

func selectEntries(entries []model.Entry, request model.VerifyRequest) []model.Entry {
	mode := request.Mode
	if mode == "" || mode == "full" {
		return entries
	}
	result := make([]model.Entry, 0)
	switch mode {
	case "range":
		for _, entry := range entries {
			if entry.Seq >= request.StartSeq && entry.Seq <= request.EndSeq {
				result = append(result, entry)
			}
		}
	case "head":
		start := len(entries) - 10
		if start < 0 {
			start = 0
		}
		result = append(result, entries[start:]...)
	case "spot":
		step := request.SampleStep
		if step < 1 {
			step = 100
		}
		for i, entry := range entries {
			if i%step == 0 || i == len(entries)-1 {
				result = append(result, entry)
			}
		}
	}
	return result
}

func ValidateVerifyRequest(request model.VerifyRequest) error {
	if request.Mode == "" {
		return nil
	}
	switch request.Mode {
	case "full", "head":
		return nil
	case "range":
		if request.StartSeq < 1 || request.EndSeq < request.StartSeq {
			return fmt.Errorf("range requires valid start_seq and end_seq")
		}
		return nil
	case "spot":
		if request.SampleStep < 1 {
			return fmt.Errorf("spot requires positive sample_step")
		}
		return nil
	default:
		return fmt.Errorf("unknown verification mode %q", request.Mode)
	}
}

func findSeq(entries []model.Entry, seq int64) int {
	for i := range entries {
		if entries[i].Seq == seq {
			return i
		}
	}
	return -1
}
func fail(report *model.VerifyReport, status string, seq int64, reason string) {
	report.Status, report.BreakSeq, report.BreakReason = status, seq, reason
}
