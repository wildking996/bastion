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

func recordSQLiteError(err error) {
	busy, locked := classifySQLiteError(err)
	if busy {
		atomic.AddUint64(&sqliteBusyErrors, 1)
	}
	if locked {
		atomic.AddUint64(&sqliteLockedErrors, 1)
	}
}

func SQLiteBusyErrorsTotal() uint64 {
	return atomic.LoadUint64(&sqliteBusyErrors)
}

func SQLiteLockedErrorsTotal() uint64 {
	return atomic.LoadUint64(&sqliteLockedErrors)
}

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
