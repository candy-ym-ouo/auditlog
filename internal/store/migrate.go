package store

import (
	"database/sql"
	"fmt"
	"time"
)

const LatestSchema = 3

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var version int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	steps := []string{
		`CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY,value TEXT NOT NULL); CREATE TABLE IF NOT EXISTS audit_entries(id INTEGER PRIMARY KEY AUTOINCREMENT,seq INTEGER NOT NULL UNIQUE,prev_hash TEXT NOT NULL,hash TEXT NOT NULL,actor TEXT NOT NULL,action TEXT NOT NULL,target TEXT NOT NULL,detail TEXT NOT NULL,event_time TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS archive_batches(batch_no INTEGER PRIMARY KEY,start_seq INTEGER NOT NULL,end_seq INTEGER NOT NULL,prev_hash TEXT NOT NULL,head_hash TEXT NOT NULL,item_count INTEGER NOT NULL,payload_hash TEXT NOT NULL,archived_at TEXT NOT NULL); CREATE TABLE IF NOT EXISTS archive_entries(id INTEGER PRIMARY KEY AUTOINCREMENT,seq INTEGER NOT NULL UNIQUE,prev_hash TEXT NOT NULL,hash TEXT NOT NULL,actor TEXT NOT NULL,action TEXT NOT NULL,target TEXT NOT NULL,detail TEXT NOT NULL,event_time TEXT NOT NULL,batch_no INTEGER NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_actor ON audit_entries(actor); CREATE INDEX IF NOT EXISTS idx_entries_action ON audit_entries(action); CREATE INDEX IF NOT EXISTS idx_entries_time ON audit_entries(event_time); CREATE INDEX IF NOT EXISTS idx_arch_entries_batch ON archive_entries(batch_no);`,
	}
	for i := version; i < LatestSchema; i++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(steps[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)`, i+1, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
