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
	Threshold   int       `json:"threshold"`
	KeepMin     int       `json:"keep_min"`
	MaxAgeDays  int       `json:"max_age_days"`
}

type BatchPage struct {
	Items    []ArchiveBatch `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
}
