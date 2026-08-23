package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"auditlog/internal/model"
)

var defaultRiskPolicy = RiskPolicy{
	CriticalActions: []string{"drop", "purge", "disable_audit", "rotate_root_key"},
	HighActions:     []string{"delete", "grant_admin", "export", "impersonate"},
	MediumActions:   []string{"update_policy", "login", "permission_change", "archive"},
	FailureTerms:    []string{"denied", "failed", "failure", "timeout", "unauthorized", "invalid"},
}

func evaluateRisks(entries []model.Entry, policy RiskPolicy) RiskSummary {
	policy = mergeRiskPolicy(policy)
	summary := RiskSummary{
		ByLevel:  map[RiskLevel]int{RiskLow: 0, RiskMedium: 0, RiskHigh: 0, RiskCritical: 0},
		ByReason: make(map[string]int),
		Events:   make([]RiskEvent, 0),
	}
	for _, entry := range entries {
		level, reason := classifyRisk(entry, policy)
		if level == RiskLow {
			continue
		}
		summary.ByLevel[level]++
		summary.ByReason[reason]++
		summary.Events = append(summary.Events, RiskEvent{
			Seq: entry.Seq, Actor: entry.Actor, Action: entry.Action, Target: entry.Target,
			Level: level, Reason: reason, EventTime: entry.EventTime,
		})
	}
	sort.SliceStable(summary.Events, func(i, j int) bool {
		left, right := riskWeight(summary.Events[i].Level), riskWeight(summary.Events[j].Level)
		if left == right {
			return summary.Events[i].EventTime.After(summary.Events[j].EventTime)
		}
		return left > right
	})
	summary.Score = riskScore(summary.ByLevel, len(entries))
	return summary
}

func mergeRiskPolicy(policy RiskPolicy) RiskPolicy {
	if len(policy.CriticalActions) == 0 {
		policy.CriticalActions = defaultRiskPolicy.CriticalActions
	}
	if len(policy.HighActions) == 0 {
		policy.HighActions = defaultRiskPolicy.HighActions
	}
	if len(policy.MediumActions) == 0 {
		policy.MediumActions = defaultRiskPolicy.MediumActions
	}
	if len(policy.FailureTerms) == 0 {
		policy.FailureTerms = defaultRiskPolicy.FailureTerms
	}
	policy.CriticalActions = normalizedSet(policy.CriticalActions)
	policy.HighActions = normalizedSet(policy.HighActions)
	policy.MediumActions = normalizedSet(policy.MediumActions)
	policy.FailureTerms = normalizedSet(policy.FailureTerms)
	return policy
}

func classifyRisk(entry model.Entry, policy RiskPolicy) (RiskLevel, string) {
	action := strings.ToLower(entry.Action)
	if containsAny(action, policy.CriticalActions) {
		return RiskCritical, "critical action"
	}
	if containsAny(action, policy.HighActions) {
		return RiskHigh, "privileged or destructive action"
	}
	if detailContains(entry.Detail, policy.FailureTerms) {
		return RiskHigh, "failure indicator"
	}
	if containsAny(action, policy.MediumActions) {
		return RiskMedium, "sensitive action"
	}
	return RiskLow, ""
}

func detailContains(raw json.RawMessage, terms []string) bool {
	value := strings.ToLower(string(raw))
	return containsAny(value, terms)
}

func containsAny(value string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func riskWeight(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 8
	case RiskHigh:
		return 4
	case RiskMedium:
		return 2
	default:
		return 0
	}
}

func riskScore(levels map[RiskLevel]int, total int) int {
	if total == 0 {
		return 0
	}
	weighted := levels[RiskCritical]*8 + levels[RiskHigh]*4 + levels[RiskMedium]*2
	score := weighted * 100 / (total * 8)
	if score > 100 {
		return 100
	}
	return score
}

func inspectIntegrity(entries []model.Entry) IntegritySummary {
	result := IntegritySummary{Continuous: true, Issues: []IntegrityIssue{}}
	if len(entries) < 2 {
		return result
	}
	ordered := append([]model.Entry(nil), entries...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })
	for i := 1; i < len(ordered); i++ {
		previous, current := ordered[i-1], ordered[i]
		if current.Seq != previous.Seq+1 {
			result.Issues = append(result.Issues, IntegrityIssue{
				Seq: current.Seq, Kind: "sequence_gap",
				Expected: fmt.Sprintf("%d", previous.Seq+1), Actual: fmt.Sprintf("%d", current.Seq),
			})
		}
		if current.PrevHash != previous.Hash {
			result.Issues = append(result.Issues, IntegrityIssue{
				Seq: current.Seq, Kind: "previous_hash_mismatch", Expected: previous.Hash, Actual: current.PrevHash,
			})
		}
	}
	result.Continuous = len(result.Issues) == 0
	return result
}
