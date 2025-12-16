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

// InitDB initializes the database
func InitDB() error {
	var err error

	// Configure GORM log level
	logLevel := logger.Silent
	if config.Settings.LogLevel == "DEBUG" {
		logLevel = logger.Info
	}

	logWriter := log.Writer()

	DB, err = gorm.Open(sqlite.Open(config.Settings.DatabaseURL), &gorm.Config{
		Logger: logger.New(
			log.New(logWriter, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel: logLevel,
			},
		),
	})
	if err != nil {
		return err
	}

	// Get underlying SQL DB and configure the connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	// Tune pool parameters for concurrency and resource usage
	sqlDB.SetMaxIdleConns(10)                  // Max idle connections
	sqlDB.SetMaxOpenConns(100)                 // Max open connections
	sqlDB.SetConnMaxLifetime(time.Hour)        // Connection max lifetime (1 hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Idle connection max lifetime (10 minutes)

	// Auto-migrate database tables
	err = DB.AutoMigrate(&models.Bastion{}, &models.Mapping{})
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
