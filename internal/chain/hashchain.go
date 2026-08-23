package chain

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"auditlog/internal/model"
)

func HashEntry(entry model.Entry) (string, error) {
	if strings.Contains(entry.Actor, "|") || strings.Contains(entry.Action, "|") || strings.Contains(entry.Target, "|") || strings.Contains(string(entry.Detail), "|") {
		return "", fmt.Errorf("entry contains reserved delimiter")
	}
	canonical, err := model.CanonicalJSON(entry.Detail)
	if err != nil {
		return "", err
	}
	parts := []string{strconv.FormatInt(entry.Seq, 10), entry.PrevHash, entry.EventTime.UTC().Format(time.RFC3339Nano), entry.Actor, entry.Action, entry.Target, string(canonical)}
	return Digest([]byte(strings.Join(parts, "|"))), nil
}

func NewEntry(seq int64, prevHash string, request model.AppendRequest, now time.Time) (model.Entry, error) {
	if err := request.Normalize(now); err != nil {
		return model.Entry{}, err
	}
	eventTime := now.UTC()
	if request.EventTime != nil {
		eventTime = request.EventTime.UTC()
	}
	entry := model.Entry{Seq: seq, PrevHash: prevHash, Actor: request.Actor, Action: request.Action, Target: request.Target, Detail: request.Detail, EventTime: eventTime}
	hash, err := HashEntry(entry)
	if err != nil {
		return model.Entry{}, err
	}
	entry.Hash = hash
	return entry, nil
}

func Genesis(now time.Time) (model.Entry, error) {
	return NewEntry(1, ZeroHash, model.AppendRequest{Actor: "system", Action: "genesis", Target: "audit_chain"}, now)
}
