package database

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"
)

var sqliteBusyErrors uint64
var sqliteLockedErrors uint64

// classifySQLiteError classifies an error as indicating an SQLite "busy" or "locked" condition.
// It returns two booleans: busy is true when the error message suggests a busy state (for example contains "sqlite_busy", "database is locked", or "busy timeout"); locked is true when the message suggests a locked table (for example contains "sqlite_locked" or "database table is locked").
// nil errors and context cancellation/deadline errors are treated as neither busy nor locked.
func classifySQLiteError(err error) (busy bool, locked bool) {
	if err == nil {
		return false, false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, false
	}

	msg := strings.ToLower(err.Error())

	if strings.Contains(msg, "sqlite_busy") || strings.Contains(msg, "database is locked") || strings.Contains(msg, "busy timeout") {
		busy = true
	}
	if strings.Contains(msg, "sqlite_locked") || strings.Contains(msg, "database table is locked") {
		locked = true
	}

	return busy, locked
}

// recordSQLiteError inspects err and atomically increments package counters that track
// SQLite "busy" and "locked" error occurrences when the error indicates those conditions.
// If the error does not indicate either condition, the counters are not modified.
func recordSQLiteError(err error) {
	busy, locked := classifySQLiteError(err)
	if busy {
		atomic.AddUint64(&sqliteBusyErrors, 1)
	}
	if locked {
		atomic.AddUint64(&sqliteLockedErrors, 1)
	}
}

// SQLiteBusyErrorsTotal returns the total number of observed SQLite "busy" errors.
// The value is read atomically and reflects errors classified as SQLite busy conditions.
func SQLiteBusyErrorsTotal() uint64 {
	return atomic.LoadUint64(&sqliteBusyErrors)
}

// SQLiteLockedErrorsTotal returns the total number of observed SQLite "locked" errors.
// The count is read atomically and is safe for concurrent use.
func SQLiteLockedErrorsTotal() uint64 {
	return atomic.LoadUint64(&sqliteLockedErrors)
}

// SQLiteUp reports whether the package-level database is reachable by pinging its underlying sql.DB.
// It returns `true` if the database responds to a ping, `false` otherwise. If the global DB is nil
// or obtaining the underlying *sql.DB fails it returns `false`. If the provided context has no
// deadline or the deadline has already expired, a 200ms timeout is used for the ping.
func SQLiteUp(ctx context.Context) bool {
	if DB == nil {
		return false
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return false
	}

	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
	}

	return sqlDB.PingContext(ctx) == nil
}
