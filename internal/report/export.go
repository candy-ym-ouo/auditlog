package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
)

func WriteCSV(w io.Writer, report Report) error {
	writer := csv.NewWriter(w)
	rows := [][]string{
		{"section", "name", "count", "percentage", "start", "end", "risk_count"},
		{"summary", "total_entries", strconv.Itoa(report.Summary.TotalEntries), "", "", "", ""},
		{"summary", "unique_actors", strconv.Itoa(report.Summary.UniqueActors), "", "", "", ""},
		{"summary", "unique_actions", strconv.Itoa(report.Summary.UniqueActions), "", "", "", ""},
		{"summary", "unique_targets", strconv.Itoa(report.Summary.UniqueTargets), "", "", "", ""},
	}
	for _, rank := range report.Actors {
		rows = append(rows, rankRow("actor", rank))
	}
	for _, rank := range report.Actions {
		rows = append(rows, rankRow("action", rank))
	}
	for _, rank := range report.Targets {
		rows = append(rows, rankRow("target", rank))
	}
	for _, bucket := range report.Timeline {
		rows = append(rows, []string{
			"timeline", bucket.Start.Format(time.RFC3339), strconv.Itoa(bucket.Count), "",
			bucket.Start.Format(time.RFC3339), bucket.End.Format(time.RFC3339), strconv.Itoa(bucket.RiskCount),
		})
	}
	for _, event := range report.Risks.Events {
		rows = append(rows, []string{
			"risk", fmt.Sprintf("%s:%s:%s", event.Level, event.Action, event.Target), "1", "",
			event.EventTime.Format(time.RFC3339), "", "1",
		})
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func rankRow(section string, rank Rank) []string {
	return []string{
		section, rank.Name, strconv.Itoa(rank.Count), strconv.FormatFloat(rank.Percentage, 'f', 2, 64),
		rank.FirstSeen.Format(time.RFC3339), rank.LastSeen.Format(time.RFC3339), "",
	}
}
