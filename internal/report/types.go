package report

import (
	"errors"
	"time"
)

var (
	ErrInvalidRange   = errors.New("report end must not be before start")
	ErrInvalidBucket  = errors.New("report bucket must be at least one minute")
	ErrTooManyBuckets = errors.New("report window contains too many timeline buckets")
)

type Request struct {
	From       *time.Time
	To         *time.Time
	Actors     []string
	Actions    []string
	Targets    []string
	Bucket     time.Duration
	TopLimit   int
	RiskPolicy RiskPolicy
}

type Report struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Window      Window           `json:"window"`
	Summary     Summary          `json:"summary"`
	Timeline    []Bucket         `json:"timeline"`
	Actors      []Rank           `json:"actors"`
	Actions     []Rank           `json:"actions"`
	Targets     []Rank           `json:"targets"`
	Risks       RiskSummary      `json:"risks"`
	Integrity   IntegritySummary `json:"integrity"`
}

type Window struct {
	From   *time.Time    `json:"from,omitempty"`
	To     *time.Time    `json:"to,omitempty"`
	Bucket time.Duration `json:"bucket"`
}

type Summary struct {
	TotalEntries   int     `json:"total_entries"`
	UniqueActors   int     `json:"unique_actors"`
	UniqueActions  int     `json:"unique_actions"`
	UniqueTargets  int     `json:"unique_targets"`
	FirstSequence  int64   `json:"first_sequence,omitempty"`
	LastSequence   int64   `json:"last_sequence,omitempty"`
	AveragePerHour float64 `json:"average_per_hour"`
}

type Bucket struct {
	Start     time.Time      `json:"start"`
	End       time.Time      `json:"end"`
	Count     int            `json:"count"`
	RiskCount int            `json:"risk_count"`
	Actions   map[string]int `json:"actions"`
}

type Rank struct {
	Name       string    `json:"name"`
	Count      int       `json:"count"`
	Percentage float64   `json:"percentage"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type RiskPolicy struct {
	CriticalActions []string
	HighActions     []string
	MediumActions   []string
	FailureTerms    []string
}

type RiskEvent struct {
	Seq       int64     `json:"seq"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Level     RiskLevel `json:"level"`
	Reason    string    `json:"reason"`
	EventTime time.Time `json:"event_time"`
}

type RiskSummary struct {
	Score    int               `json:"score"`
	ByLevel  map[RiskLevel]int `json:"by_level"`
	ByReason map[string]int    `json:"by_reason"`
	Events   []RiskEvent       `json:"events"`
}

type IntegrityIssue struct {
	Seq      int64  `json:"seq"`
	Kind     string `json:"kind"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type IntegritySummary struct {
	Continuous bool             `json:"continuous"`
	Issues     []IntegrityIssue `json:"issues"`
}
