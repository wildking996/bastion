package database

import (
	"bastion/config"
	"fmt"
	"net/url"
	"strings"
)

type sqlitePoolConfig struct {
	maxOpenConns int
	maxIdleConns int
	maxIdleSec   int
	maxLifeSec   int
}

// sanitizeSQLitePoolConfig normalizes a sqlitePoolConfig, enforcing sensible bounds on its fields.
// It ensures maxOpenConns is at least 1, clamps maxIdleConns to the range [0, maxOpenConns],
// and forces maxIdleSec and maxLifeSec to be at least 0. The sanitized config is returned.
func sanitizeSQLitePoolConfig(cfg sqlitePoolConfig) sqlitePoolConfig {
	if cfg.maxOpenConns < 1 {
		cfg.maxOpenConns = 1
	}
	if cfg.maxIdleConns < 0 {
		cfg.maxIdleConns = 0
	}
	if cfg.maxIdleConns > cfg.maxOpenConns {
		cfg.maxIdleConns = cfg.maxOpenConns
	}
	if cfg.maxIdleSec < 0 {
		cfg.maxIdleSec = 0
	}
	if cfg.maxLifeSec < 0 {
		cfg.maxLifeSec = 0
	}
	return cfg
}

// buildSQLiteDSN constructs a SQLite DSN from dbPath and settings.
// If settings.SQLitePragmasEnabled is true, it appends SQLite PRAGMA parameters
// (busy_timeout, journal_mode, synchronous, foreign_keys) to the query portion,
// preserving any existing query parameters. If no query parameters are present
// after processing, the base path is returned without a trailing "?".
func buildSQLiteDSN(dbPath string, settings *config.Config) string {
	base, rawQuery, hasQuery := strings.Cut(dbPath, "?")

	query, _ := url.ParseQuery(rawQuery)

	if settings.SQLitePragmasEnabled {
		if settings.SQLiteBusyTimeoutMS > 0 {
			query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", settings.SQLiteBusyTimeoutMS))
		}
		if journalMode := normalizeSQLiteJournalMode(settings.SQLiteJournalMode); journalMode != "" {
			query.Add("_pragma", fmt.Sprintf("journal_mode(%s)", journalMode))
		}
		if synchronous := normalizeSQLiteSynchronous(settings.SQLiteSynchronous); synchronous != "" {
			query.Add("_pragma", fmt.Sprintf("synchronous(%s)", synchronous))
		}
		if settings.SQLiteForeignKeys {
			query.Add("_pragma", "foreign_keys(1)")
		} else {
			query.Add("_pragma", "foreign_keys(0)")
		}
	}

	if len(query) == 0 {
		return base
	}

	encoded := query.Encode()
	if !hasQuery && encoded != "" {
		return base + "?" + encoded
	}
	return base + "?" + encoded
}

// currentSQLitePoolConfig builds a sqlitePoolConfig from the provided Config's SQLite
// connection and pool settings and enforces sane bounds.
//
// It reads SQLiteMaxOpenConns, SQLiteMaxIdleConns, SQLiteConnMaxIdleSec and
// SQLiteConnMaxLifeSec from settings and returns the sanitized configuration.
func currentSQLitePoolConfig(settings *config.Config) sqlitePoolConfig {
	return sanitizeSQLitePoolConfig(sqlitePoolConfig{
		maxOpenConns: settings.SQLiteMaxOpenConns,
		maxIdleConns: settings.SQLiteMaxIdleConns,
		maxIdleSec:   settings.SQLiteConnMaxIdleSec,
		maxLifeSec:   settings.SQLiteConnMaxLifeSec,
	})
}

// normalizeSQLiteJournalMode converts the input to an accepted uppercase SQLite journal mode or returns an empty string if the value is invalid.
// Accepted modes: "WAL", "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF".
func normalizeSQLiteJournalMode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "WAL", "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
		return value
	default:
		return ""
	}
}

// normalizeSQLiteSynchronous normalizes and validates a SQLite `synchronous` pragma value.
// It returns the trimmed, uppercased value if it is one of `OFF`, `NORMAL`, `FULL`, `EXTRA` or one of the numeric strings `0`, `1`, `2`, `3`; otherwise it returns an empty string.
func normalizeSQLiteSynchronous(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "OFF", "NORMAL", "FULL", "EXTRA":
		return value
	case "0", "1", "2", "3":
		return value
	default:
		return ""
	}
}