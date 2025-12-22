package database

import (
	"bastion/config"
	"bastion/models"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes and configures the package-level GORM SQLite database according to config.Settings, applies connection pool settings and optional SQLite PRAGMAs, runs automigrations for models.Bastion and models.Mapping, and assigns the resulting *gorm.DB to the package DB.
// It returns an error if opening the database, obtaining the underlying sql.DB, or running the migrations fails.
func InitDB() error {
	var err error

	// Configure GORM log level
	logLevel := logger.Silent
	if config.Settings.LogLevel == "DEBUG" {
		logLevel = logger.Info
	}

	logWriter := log.Writer()

	dsn := buildSQLiteDSN(config.Settings.DatabaseURL, config.Settings)
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: sqliteMetricsLogger{inner: logger.New(
			log.New(logWriter, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel: logLevel,
			},
		)},
	})
	if err != nil {
		return err
	}

	// Get underlying SQL DB and configure the connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	pool := currentSQLitePoolConfig(config.Settings)
	sqlDB.SetMaxIdleConns(pool.maxIdleConns)
	sqlDB.SetMaxOpenConns(pool.maxOpenConns)
	sqlDB.SetConnMaxIdleTime(time.Duration(pool.maxIdleSec) * time.Second)
	sqlDB.SetConnMaxLifetime(time.Duration(pool.maxLifeSec) * time.Second)

	// Apply PRAGMAs again as a best-effort startup initialization (useful for existing DB files).
	// Connection URL parameters ensure PRAGMAs are applied for new connections too.
	if config.Settings.SQLitePragmasEnabled {
		if config.Settings.SQLiteBusyTimeoutMS > 0 {
			DB.Exec("PRAGMA busy_timeout = ?", config.Settings.SQLiteBusyTimeoutMS)
		}
		if journalMode := normalizeSQLiteJournalMode(config.Settings.SQLiteJournalMode); journalMode != "" {
			DB.Exec("PRAGMA journal_mode = " + journalMode)
		}
		if synchronous := normalizeSQLiteSynchronous(config.Settings.SQLiteSynchronous); synchronous != "" {
			DB.Exec("PRAGMA synchronous = " + synchronous)
		}
		if config.Settings.SQLiteForeignKeys {
			DB.Exec("PRAGMA foreign_keys = ON")
		} else {
			DB.Exec("PRAGMA foreign_keys = OFF")
		}
	}

	// Auto-migrate database tables
	err = DB.AutoMigrate(&models.Bastion{}, &models.Mapping{}, &models.AppSetting{})
	if err != nil {
		return err
	}

	log.Println("Database initialized successfully")
	return nil
}

// CloseDB closes the database connection and releases resources
func CloseDB() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	log.Println("Closing database connection...")
	return sqlDB.Close()
}
