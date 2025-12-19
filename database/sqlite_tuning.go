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

func currentSQLitePoolConfig(settings *config.Config) sqlitePoolConfig {
	return sanitizeSQLitePoolConfig(sqlitePoolConfig{
		maxOpenConns: settings.SQLiteMaxOpenConns,
		maxIdleConns: settings.SQLiteMaxIdleConns,
		maxIdleSec:   settings.SQLiteConnMaxIdleSec,
		maxLifeSec:   settings.SQLiteConnMaxLifeSec,
	})
}

func normalizeSQLiteJournalMode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "WAL", "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
		return value
	default:
		return ""
	}
}

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
