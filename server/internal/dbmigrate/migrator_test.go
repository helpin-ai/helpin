package dbmigrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateUsesTimestampVersion(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 4, 29, 10, 11, 12, 345678000, time.UTC)

	path, err := createAt(dir, "Add Billing Events!", now)
	if err != nil {
		t.Fatalf("create migration: %v", err)
	}

	const wantName = "20260429101112345678_add_billing_events.sql"
	if filepath.Base(path) != wantName {
		t.Fatalf("migration filename = %q, want %q", filepath.Base(path), wantName)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if string(contents) != "-- Migration: add_billing_events\n" {
		t.Fatalf("migration contents = %q", string(contents))
	}
}

func TestCreateBumpsTimestampVersionOnExactCollision(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 4, 29, 10, 11, 12, 345678000, time.UTC)

	if _, err := createAt(dir, "first", now); err != nil {
		t.Fatalf("create first migration: %v", err)
	}
	path, err := createAt(dir, "second", now)
	if err != nil {
		t.Fatalf("create second migration: %v", err)
	}

	const wantPrefix = "20260429101112345679_second.sql"
	if filepath.Base(path) != wantPrefix {
		t.Fatalf("migration filename = %q, want %q", filepath.Base(path), wantPrefix)
	}
}

func TestCreateKeepsLegacySequentialVersionsAsExistingVersions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "202604290001_existing.sql"), []byte("SELECT 1;\n"), 0644); err != nil {
		t.Fatalf("write legacy migration: %v", err)
	}

	path, err := createAt(dir, "new migration", time.Date(2026, 4, 29, 0, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatalf("create migration: %v", err)
	}

	if !strings.HasPrefix(filepath.Base(path), "20260429000001000000_") {
		t.Fatalf("migration filename = %q, want timestamp prefix", filepath.Base(path))
	}
}
