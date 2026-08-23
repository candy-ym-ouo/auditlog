package model

import "time"

type ArchiveBatch struct {
	BatchNo     int64     `json:"batch_no"`
	StartSeq    int64     `json:"start_seq"`
	EndSeq      int64     `json:"end_seq"`
	PrevHash    string    `json:"prev_hash"`
	HeadHash    string    `json:"head_hash"`
	ItemCount   int       `json:"item_count"`
	PayloadHash string    `json:"payload_hash"`
	ArchivedAt  time.Time `json:"archived_at"`
}

type ArchiveExport struct {
	Batch   ArchiveBatch `json:"batch"`
	Entries []Entry      `json:"entries"`
}

type ArchiveStatus struct {
	Running     bool      `json:"running"`
	LastRunAt   time.Time `json:"last_run_at,omitempty"`
	LastBatchNo int64     `json:"last_batch_no"`
	LastError   string    `json:"last_error"`
	LastStatus  string    `json:"last_status"`
	Threshold   int       `json:"threshold"`
	KeepMin     int       `json:"keep_min"`
	MaxAgeDays  int       `json:"max_age_days"`
}

// Archive run outcomes recorded in ArchiveStatus.LastStatus. Keeping these as
// constants (rather than ad-hoc string literals) makes the success / failure /
// canceled branches in ArchiveService.Trigger exhaustive and stable across the
// store and HTTP layers.
const (
	ArchiveStatusOK       = "ok"       // a batch was written
	ArchiveStatusNoop     = "noop"     // nothing to archive, run completed cleanly
	ArchiveStatusCanceled = "canceled" // the run was aborted via context cancellation
	ArchiveStatusError    = "error"    // the store returned a non-cancellation error
)

type BatchPage struct {
	Items    []ArchiveBatch `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
}
