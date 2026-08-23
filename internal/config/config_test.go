package config

import (
	"testing"
	"time"
)

func TestDefaultsArchiveTimeout(t *testing.T) {
	d := Defaults()
	if d.ArchiveTimeout != 30*time.Second {
		t.Fatalf("default archive timeout = %v, want 30s", d.ArchiveTimeout)
	}
}

func TestLoadArchiveTimeoutFromEnv(t *testing.T) {
	t.Setenv("AUDIT_ARCHIVE_TIMEOUT", "42s")
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ArchiveTimeout != 42*time.Second {
		t.Fatalf("archive timeout = %v, want 42s", c.ArchiveTimeout)
	}
}

func TestLoadRejectsNonPositiveTimeout(t *testing.T) {
	for _, v := range []string{"0s", "-1s"} {
		t.Setenv("AUDIT_ARCHIVE_TIMEOUT", v)
		if _, err := Load(nil); err == nil {
			t.Fatalf("Load with AUDIT_ARCHIVE_TIMEOUT=%q should have errored", v)
		}
	}
}
