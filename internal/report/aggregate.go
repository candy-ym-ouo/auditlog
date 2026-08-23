package report

import (
	"math"
	"sort"
	"time"

	"auditlog/internal/model"
)

type rankAccumulator struct {
	count     int
	firstSeen time.Time
	lastSeen  time.Time
}

func summarize(entries []model.Entry) Summary {
	summary := Summary{TotalEntries: len(entries)}
	if len(entries) == 0 {
		return summary
	}
	actors := make(map[string]struct{})
	actions := make(map[string]struct{})
	targets := make(map[string]struct{})
	minSeq, maxSeq := entries[0].Seq, entries[0].Seq
	first, last := entries[0].EventTime, entries[0].EventTime
	for _, entry := range entries {
		actors[entry.Actor] = struct{}{}
		actions[entry.Action] = struct{}{}
		targets[entry.Target] = struct{}{}
		if entry.Seq < minSeq {
			minSeq = entry.Seq
		}
		if entry.Seq > maxSeq {
			maxSeq = entry.Seq
		}
		if entry.EventTime.Before(first) {
			first = entry.EventTime
		}
		if entry.EventTime.After(last) {
			last = entry.EventTime
		}
	}
	summary.UniqueActors = len(actors)
	summary.UniqueActions = len(actions)
	summary.UniqueTargets = len(targets)
	summary.FirstSequence = minSeq
	summary.LastSequence = maxSeq
	hours := last.Sub(first).Hours()
	if hours < 1 {
		hours = 1
	}
	summary.AveragePerHour = round(float64(len(entries))/hours, 2)
	return summary
}

func rankEntries(entries []model.Entry, limit int, key func(model.Entry) string) []Rank {
	values := make(map[string]*rankAccumulator)
	for _, entry := range entries {
		name := key(entry)
		current := values[name]
		if current == nil {
			values[name] = &rankAccumulator{count: 1, firstSeen: entry.EventTime, lastSeen: entry.EventTime}
			continue
		}
		current.count++
		if entry.EventTime.Before(current.firstSeen) {
			current.firstSeen = entry.EventTime
		}
		if entry.EventTime.After(current.lastSeen) {
			current.lastSeen = entry.EventTime
		}
	}
	ranks := make([]Rank, 0, len(values))
	for name, value := range values {
		ranks = append(ranks, Rank{
			Name: name, Count: value.count,
			Percentage: percentage(value.count, len(entries)),
			FirstSeen:  value.firstSeen, LastSeen: value.lastSeen,
		})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].Count == ranks[j].Count {
			return ranks[i].Name < ranks[j].Name
		}
		return ranks[i].Count > ranks[j].Count
	})
	if len(ranks) > limit {
		ranks = ranks[:limit]
	}
	return ranks
}

func buildTimeline(entries []model.Entry, risks []RiskEvent, req Request) ([]Bucket, error) {
	if len(entries) == 0 {
		return []Bucket{}, nil
	}
	start := entries[0].EventTime
	end := entries[len(entries)-1].EventTime
	if req.From != nil {
		start = *req.From
	}
	if req.To != nil {
		end = *req.To
	}
	start = start.Truncate(req.Bucket)
	end = end.Truncate(req.Bucket)
	count := int(end.Sub(start)/req.Bucket) + 1
	if count < 1 {
		count = 1
	}
	if count > maxBucketCount {
		return nil, ErrTooManyBuckets
	}
	buckets := make([]Bucket, count)
	for i := range buckets {
		bucketStart := start.Add(time.Duration(i) * req.Bucket)
		buckets[i] = Bucket{Start: bucketStart, End: bucketStart.Add(req.Bucket), Actions: map[string]int{}}
	}
	for _, entry := range entries {
		index := int(entry.EventTime.Sub(start) / req.Bucket)
		if index >= 0 && index < len(buckets) {
			buckets[index].Count++
			buckets[index].Actions[entry.Action]++
		}
	}
	for _, event := range risks {
		index := int(event.EventTime.Sub(start) / req.Bucket)
		if index >= 0 && index < len(buckets) {
			buckets[index].RiskCount++
		}
	}
	return buckets, nil
}

func percentage(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return round(float64(count)*100/float64(total), 2)
}

func round(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}
