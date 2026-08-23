package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const MaxFieldLength = 256

type Entry struct {
	ID        int64           `json:"id"`
	Seq       int64           `json:"seq"`
	PrevHash  string          `json:"prev_hash"`
	Hash      string          `json:"hash"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	Target    string          `json:"target"`
	Detail    json.RawMessage `json:"detail"`
	EventTime time.Time       `json:"event_time"`
}

type AppendRequest struct {
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	Target    string          `json:"target"`
	Detail    json.RawMessage `json:"detail"`
	EventTime *time.Time      `json:"event_time,omitempty"`
}

type Head struct {
	Seq       int64     `json:"seq"`
	Hash      string    `json:"hash"`
	PrevHash  string    `json:"prev_hash"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Query struct {
	Actor, Action, Target string
	From, To              *time.Time
	SeqFrom, SeqTo        int64
	IncludeArchived       bool
	Page, PageSize        int
}

type Page struct {
	Items    []Entry `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Total    int64   `json:"total"`
}

func (r *AppendRequest) Normalize(now time.Time) error {
	r.Actor = strings.TrimSpace(r.Actor)
	r.Action = strings.TrimSpace(r.Action)
	r.Target = strings.TrimSpace(r.Target)
	for name, value := range map[string]string{"actor": r.Actor, "action": r.Action, "target": r.Target} {
		if value == "" || len(value) > MaxFieldLength {
			return fmt.Errorf("%s must contain 1-%d characters", name, MaxFieldLength)
		}
		if strings.Contains(value, "|") {
			return fmt.Errorf("%s contains reserved delimiter", name)
		}
	}
	if r.EventTime != nil && r.EventTime.After(now.Add(5*time.Minute)) {
		return errors.New("event_time is more than five minutes in the future")
	}
	canonical, err := CanonicalJSON(r.Detail)
	if err != nil {
		return fmt.Errorf("detail: %w", err)
	}
	r.Detail = canonical
	return nil
}

func CanonicalJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.New("must be valid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("must contain exactly one JSON value")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, errors.New("must be a JSON object")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(canonical), "|") {
		return nil, errors.New("contains reserved delimiter")
	}
	return canonical, nil
}
