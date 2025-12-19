package database

import (
	"context"
	"time"

	"gorm.io/gorm/logger"
)

type sqliteMetricsLogger struct {
	inner logger.Interface
}

func (l sqliteMetricsLogger) LogMode(level logger.LogLevel) logger.Interface {
	return sqliteMetricsLogger{inner: l.inner.LogMode(level)}
}

func (l sqliteMetricsLogger) Info(ctx context.Context, s string, args ...interface{}) {
	l.inner.Info(ctx, s, args...)
}

func (l sqliteMetricsLogger) Warn(ctx context.Context, s string, args ...interface{}) {
	l.inner.Warn(ctx, s, args...)
}

func (l sqliteMetricsLogger) Error(ctx context.Context, s string, args ...interface{}) {
	l.inner.Error(ctx, s, args...)
}

func (l sqliteMetricsLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if err != nil {
		recordSQLiteError(err)
	}
	l.inner.Trace(ctx, begin, fc, err)
}
