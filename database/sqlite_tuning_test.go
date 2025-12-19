package database

import (
	"bastion/config"
	"strings"
	"testing"
)

func TestBuildSQLiteDSN_PragmaParams(t *testing.T) {
	cfg := &config.Config{
		SQLitePragmasEnabled: true,
		SQLiteBusyTimeoutMS:  5000,
		SQLiteJournalMode:    "WAL",
		SQLiteSynchronous:    "NORMAL",
		SQLiteForeignKeys:    true,
	}

	dsn := buildSQLiteDSN("test.db", cfg)
	if dsn == "test.db" {
		t.Fatalf("expected DSN to include pragma params, got %q", dsn)
	}
	if want := "_pragma=busy_timeout%285000%29"; !strings.Contains(dsn, want) {
		t.Fatalf("expected DSN to contain %q, got %q", want, dsn)
	}
	if want := "_pragma=journal_mode%28WAL%29"; !strings.Contains(dsn, want) {
		t.Fatalf("expected DSN to contain %q, got %q", want, dsn)
	}
	if want := "_pragma=synchronous%28NORMAL%29"; !strings.Contains(dsn, want) {
		t.Fatalf("expected DSN to contain %q, got %q", want, dsn)
	}
	if want := "_pragma=foreign_keys%281%29"; !strings.Contains(dsn, want) {
		t.Fatalf("expected DSN to contain %q, got %q", want, dsn)
	}
}

func TestBuildSQLiteDSN_PreservesExistingQuery(t *testing.T) {
	cfg := &config.Config{
		SQLitePragmasEnabled: true,
		SQLiteForeignKeys:    true,
	}
	dsn := buildSQLiteDSN("test.db?cache=shared", cfg)
	if !strings.Contains(dsn, "cache=shared") {
		t.Fatalf("expected existing query to be preserved, got %q", dsn)
	}
	if !strings.Contains(dsn, "_pragma=") {
		t.Fatalf("expected pragma params, got %q", dsn)
	}
}
