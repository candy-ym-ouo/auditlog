package report

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"auditlog/internal/model"
)

func TestGenerateBuildsAggregatesRisksAndIntegrity(t *testing.T) {
	base := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	entries := []model.Entry{
		entry(1, "alice", "login", "console", "zero", "h1", base, `{"status":"ok"}`),
		entry(2, "alice", "export", "customers", "h1", "h2", base.Add(20*time.Minute), `{"status":"ok"}`),
		entry(3, "bob", "delete", "invoice/42", "wrong", "h3", base.Add(80*time.Minute), `{"status":"denied"}`),
	}
	engine := New()
	engine.now = func() time.Time { return base.Add(2 * time.Hour) }

	report, err := engine.Generate(context.Background(), entries, Request{Bucket: time.Hour, TopLimit: 5})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if report.Summary.TotalEntries != 3 || report.Summary.UniqueActors != 2 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.Timeline) != 2 || report.Timeline[0].Count != 2 || report.Timeline[1].Count != 1 {
		t.Fatalf("unexpected timeline: %+v", report.Timeline)
	}
	if len(report.Actors) != 2 || report.Actors[0].Name != "alice" || report.Actors[0].Percentage != 66.67 {
		t.Fatalf("unexpected actor ranking: %+v", report.Actors)
	}
	if report.Risks.ByLevel[RiskHigh] != 2 || report.Risks.Score == 0 {
		t.Fatalf("unexpected risks: %+v", report.Risks)
	}
	if report.Integrity.Continuous || len(report.Integrity.Issues) != 1 {
		t.Fatalf("unexpected integrity result: %+v", report.Integrity)
	}
}

func TestGenerateFiltersAndHonorsCancellation(t *testing.T) {
	base := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	entries := []model.Entry{
		entry(1, "Alice", "read", "orders/1", "zero", "h1", base, `{}`),
		entry(2, "Bob", "read", "users/1", "h1", "h2", base.Add(time.Hour), `{}`),
	}
	result, err := New().Generate(context.Background(), entries, Request{Actors: []string{" alice "}, Targets: []string{"orders"}})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Summary.TotalEntries != 1 || result.Actors[0].Name != "Alice" {
		t.Fatalf("filter did not select expected entry: %+v", result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().Generate(ctx, entries, Request{}); err != context.Canceled {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
}

func TestGenerateValidatesRequest(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	if _, err := New().Generate(context.Background(), nil, Request{From: &now, To: &earlier}); err != ErrInvalidRange {
		t.Fatalf("range error = %v", err)
	}
	if _, err := New().Generate(context.Background(), nil, Request{Bucket: time.Second}); err != ErrInvalidBucket {
		t.Fatalf("bucket error = %v", err)
	}
	entries := []model.Entry{
		entry(1, "alice", "read", "record", "zero", "h1", now.Add(-20000*time.Hour), `{}`),
		entry(2, "alice", "read", "record", "h1", "h2", now, `{}`),
	}
	if _, err := New().Generate(context.Background(), entries, Request{Bucket: time.Hour}); err != ErrTooManyBuckets {
		t.Fatalf("bucket count error = %v", err)
	}
}

func TestWriteCSV(t *testing.T) {
	base := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	report, err := New().Generate(context.Background(), []model.Entry{
		entry(1, "alice", "export", "customers", "zero", "h1", base, `{}`),
	}, Request{})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := WriteCSV(&output, report); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}
	for _, expected := range []string{"section,name,count", "summary,total_entries,1", "actor,alice,1", "risk,high:export:customers"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("CSV output missing %q:\n%s", expected, output.String())
		}
	}
}

func entry(seq int64, actor, action, target, prevHash, hash string, at time.Time, detail string) model.Entry {
	return model.Entry{Seq: seq, Actor: actor, Action: action, Target: target, PrevHash: prevHash, Hash: hash, EventTime: at, Detail: json.RawMessage(detail)}
}
