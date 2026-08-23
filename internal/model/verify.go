package model

import "time"

const (
	VerifyOK           = "ok"
	VerifyHashMismatch = "hash_mismatch"
	VerifyLinkBroken   = "link_broken"
	VerifyHeadMismatch = "head_mismatch"
)

type VerifyRequest struct {
	Mode       string `json:"mode"`
	StartSeq   int64  `json:"start_seq"`
	EndSeq     int64  `json:"end_seq"`
	SampleStep int    `json:"sample_step"`
}

type ArchiveVerification struct {
	BatchNo  int64  `json:"batch_no"`
	StartSeq int64  `json:"start_seq"`
	EndSeq   int64  `json:"end_seq"`
	HeadHash string `json:"head_hash"`
	Linked   bool   `json:"linked"`
}

type VerifyReport struct {
	Mode            string                `json:"mode"`
	Status          string                `json:"status"`
	CheckedEntries  int                   `json:"checked_entries"`
	StartSeq        int64                 `json:"start_seq"`
	EndSeq          int64                 `json:"end_seq"`
	BreakSeq        int64                 `json:"break_seq"`
	BreakReason     string                `json:"break_reason"`
	HeadHash        string                `json:"head_hash"`
	ArchivesChecked int                   `json:"archives_checked"`
	Archives        []ArchiveVerification `json:"archives"`
	DurationMS      int64                 `json:"duration_ms"`
	VerifiedAt      time.Time             `json:"verified_at"`
}

type Stats struct {
	TotalEntries     int64     `json:"total_entries"`
	ArchivedEntries  int64     `json:"archived_entries"`
	ActiveEntries    int64     `json:"active_entries"`
	HeadSeq          int64     `json:"head_seq"`
	HeadHash         string    `json:"head_hash"`
	BatchCount       int64     `json:"batch_count"`
	LastArchiveAt    time.Time `json:"last_archive_at,omitempty"`
	LastVerifyAt     time.Time `json:"last_verify_at,omitempty"`
	LastVerifyStatus string    `json:"last_verify_status,omitempty"`
}
