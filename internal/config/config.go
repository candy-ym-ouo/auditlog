package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr              string
	DatabasePath      string
	Token             string
	ArchiveInterval   time.Duration
	ArchiveTimeout    time.Duration
	ArchiveThreshold  int
	ArchiveKeepMin    int
	ArchiveMaxAgeDays int
}

func Defaults() Config {
	return Config{Addr: "127.0.0.1:8080", DatabasePath: "audit.db", ArchiveInterval: time.Hour, ArchiveTimeout: 30 * time.Second, ArchiveThreshold: 10000, ArchiveKeepMin: 1000, ArchiveMaxAgeDays: 90}
}

func Load(args []string) (Config, error) {
	c := Defaults()
	setString(&c.Addr, "AUDIT_ADDR")
	setString(&c.DatabasePath, "AUDIT_DB")
	setString(&c.Token, "AUDIT_TOKEN")
	if err := setDuration(&c.ArchiveInterval, "AUDIT_ARCHIVE_INTERVAL"); err != nil {
		return c, err
	}
	if err := setDuration(&c.ArchiveTimeout, "AUDIT_ARCHIVE_TIMEOUT"); err != nil {
		return c, err
	}
	if err := setInt(&c.ArchiveThreshold, "AUDIT_ARCHIVE_THRESHOLD"); err != nil {
		return c, err
	}
	if err := setInt(&c.ArchiveKeepMin, "AUDIT_ARCHIVE_KEEP_MIN"); err != nil {
		return c, err
	}
	if err := setInt(&c.ArchiveMaxAgeDays, "AUDIT_ARCHIVE_MAX_AGE_DAYS"); err != nil {
		return c, err
	}
	fs := flag.NewFlagSet("auditlog-server", flag.ContinueOnError)
	fs.StringVar(&c.Addr, "addr", c.Addr, "HTTP listen address")
	fs.StringVar(&c.DatabasePath, "db", c.DatabasePath, "SQLite database path")
	fs.StringVar(&c.Token, "token", c.Token, "optional bearer token")
	fs.DurationVar(&c.ArchiveInterval, "archive-interval", c.ArchiveInterval, "archive schedule interval")
	fs.DurationVar(&c.ArchiveTimeout, "archive-timeout", c.ArchiveTimeout, "per-run timeout for archive operations")
	fs.IntVar(&c.ArchiveThreshold, "archive-threshold", c.ArchiveThreshold, "active entry threshold")
	fs.IntVar(&c.ArchiveKeepMin, "archive-keep-min", c.ArchiveKeepMin, "minimum active entries")
	fs.IntVar(&c.ArchiveMaxAgeDays, "archive-max-age-days", c.ArchiveMaxAgeDays, "maximum active age")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if c.Addr == "" || c.DatabasePath == "" {
		return errors.New("addr and db must not be empty")
	}
	if c.ArchiveInterval <= 0 {
		return errors.New("archive interval must be positive")
	}
	if c.ArchiveTimeout <= 0 {
		return errors.New("archive timeout must be positive")
	}
	if c.ArchiveThreshold < 1 || c.ArchiveKeepMin < 1 {
		return errors.New("archive threshold and keep-min must be positive")
	}
	if c.ArchiveKeepMin >= c.ArchiveThreshold {
		return errors.New("archive keep-min must be lower than threshold")
	}
	if c.ArchiveMaxAgeDays < 0 {
		return errors.New("archive max-age-days cannot be negative")
	}
	return nil
}

func setString(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}
func setInt(dst *int, key string) error {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		*dst = n
	}
	return nil
}
func setDuration(dst *time.Duration, key string) error {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		*dst = d
	}
	return nil
}
