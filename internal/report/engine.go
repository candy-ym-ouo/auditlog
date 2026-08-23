package report

import (
	"context"
	"sort"
	"strings"
	"time"

	"auditlog/internal/model"
)

const (
	defaultBucket   = time.Hour
	defaultTopLimit = 10
	maxTopLimit     = 100
	maxBucketCount  = 10000
)

type Engine struct {
	now func() time.Time
}

func New() *Engine {
	return &Engine{now: time.Now}
}

func (e *Engine) Generate(ctx context.Context, entries []model.Entry, req Request) (Report, error) {
	req, err := normalizeRequest(req)
	if err != nil {
		return Report{}, err
	}

	selected := make([]model.Entry, 0, len(entries))
	for i, entry := range entries {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return Report{}, err
			}
		}
		if matches(entry, req) {
			selected = append(selected, entry)
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].EventTime.Equal(selected[j].EventTime) {
			return selected[i].Seq < selected[j].Seq
		}
		return selected[i].EventTime.Before(selected[j].EventTime)
	})

	risk := evaluateRisks(selected, req.RiskPolicy)
	timeline, err := buildTimeline(selected, risk.Events, req)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		GeneratedAt: e.now().UTC(),
		Window:      Window{From: req.From, To: req.To, Bucket: req.Bucket},
		Summary:     summarize(selected),
		Timeline:    timeline,
		Actors:      rankEntries(selected, req.TopLimit, func(entry model.Entry) string { return entry.Actor }),
		Actions:     rankEntries(selected, req.TopLimit, func(entry model.Entry) string { return entry.Action }),
		Targets:     rankEntries(selected, req.TopLimit, func(entry model.Entry) string { return entry.Target }),
		Risks:       risk,
		Integrity:   inspectIntegrity(selected),
	}
	return report, nil
}

func normalizeRequest(req Request) (Request, error) {
	if req.From != nil && req.To != nil && req.To.Before(*req.From) {
		return Request{}, ErrInvalidRange
	}
	if req.Bucket == 0 {
		req.Bucket = defaultBucket
	}
	if req.Bucket < time.Minute {
		return Request{}, ErrInvalidBucket
	}
	if req.TopLimit <= 0 {
		req.TopLimit = defaultTopLimit
	}
	if req.TopLimit > maxTopLimit {
		req.TopLimit = maxTopLimit
	}
	req.Actors = normalizedSet(req.Actors)
	req.Actions = normalizedSet(req.Actions)
	req.Targets = normalizedSet(req.Targets)
	return req, nil
}

func normalizedSet(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func matches(entry model.Entry, req Request) bool {
	if req.From != nil && entry.EventTime.Before(*req.From) {
		return false
	}
	if req.To != nil && entry.EventTime.After(*req.To) {
		return false
	}
	return matchesExact(entry.Actor, req.Actors) &&
		matchesExact(entry.Action, req.Actions) &&
		matchesContains(entry.Target, req.Targets)
}

func matchesExact(value string, candidates []string) bool {
	if len(candidates) == 0 {
		return true
	}
	value = strings.ToLower(value)
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func matchesContains(value string, candidates []string) bool {
	if len(candidates) == 0 {
		return true
	}
	value = strings.ToLower(value)
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
