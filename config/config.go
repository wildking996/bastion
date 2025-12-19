package config

import (
	"bastion/version"
	"flag"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	LogLevel             string
	LogFilePath          string
	Port                 int
	DatabaseURL          string
	SQLitePragmasEnabled bool
	SQLiteBusyTimeoutMS  int
	SQLiteJournalMode    string
	SQLiteSynchronous    string
	SQLiteForeignKeys    bool
	SQLiteMaxOpenConns   int
	SQLiteMaxIdleConns   int
	SQLiteConnMaxIdleSec int
	SQLiteConnMaxLifeSec int
	SSHConnectTimeout    int
	SSHKeepaliveInterval int
	AuditEnabled         bool
	CLIMode              bool
	CLIServer            string // Server URL for CLI mode

	// Tunable limits and timeouts
	MaxSessionConnections           int
	ForwardBufferSize               int
	AuditQueueSize                  int
	MaxHTTPLogs                     int
	HTTPPairCleanupIntervalMinutes  int
	HTTPPairMaxAgeMinutes           int
	GoroutineMonitorIntervalSeconds int
	GoroutineWarnThreshold          int
	Socks5HandshakeTimeoutSeconds   int
	SessionIdleTimeoutHours         int
	SSHConnectMaxRetries            int
	SSHConnectRetryDelaySeconds     int

	// HTTP audit log gzip decode (on-demand)
	HTTPGzipDecodeMaxBytes     int
	HTTPGzipDecodeTimeoutMS    int
	HTTPGzipDecodeCacheSeconds int
}

var Settings *Config

// init initializes the package-level Settings with default configuration values sourced from environment variables.
// It sets logging, server, SQLite pragmas and connection parameters, SSH timeouts, audit/CLI flags, and various tunable limits using environment overrides or sensible defaults.
func init() {
	Settings = &Config{
		LogLevel:             getEnv("LOG_LEVEL", "INFO"),
		LogFilePath:          getEnv("LOG_FILE", "./bastion.log"),
		Port:                 getEnvInt("PORT", 7788),
		DatabaseURL:          getEnv("DATABASE_URL", "bastion.db"),
		SQLitePragmasEnabled: getEnvBool("SQLITE_PRAGMAS_ENABLED", true),
		SQLiteBusyTimeoutMS:  getEnvInt("SQLITE_BUSY_TIMEOUT_MS", 5000),
		SQLiteJournalMode:    getEnv("SQLITE_JOURNAL_MODE", "WAL"),
		SQLiteSynchronous:    getEnv("SQLITE_SYNCHRONOUS", "NORMAL"),
		SQLiteForeignKeys:    getEnvBool("SQLITE_FOREIGN_KEYS", true),
		SQLiteMaxOpenConns:   getEnvInt("SQLITE_MAX_OPEN_CONNS", 1),
		SQLiteMaxIdleConns:   getEnvInt("SQLITE_MAX_IDLE_CONNS", 1),
		SQLiteConnMaxIdleSec: getEnvInt("SQLITE_CONN_MAX_IDLE_SECONDS", 300),
		SQLiteConnMaxLifeSec: getEnvInt("SQLITE_CONN_MAX_LIFETIME_SECONDS", 0),
		SSHConnectTimeout:    getEnvInt("SSH_CONNECT_TIMEOUT", 15),
		SSHKeepaliveInterval: getEnvInt("SSH_KEEPALIVE_INTERVAL", 30),
		AuditEnabled:         getEnvBool("AUDIT_ENABLED", true),
		CLIMode:              getEnvBool("CLI_MODE", false),

		MaxSessionConnections:           getEnvInt("MAX_SESSION_CONNECTIONS", 1000),
		ForwardBufferSize:               getEnvInt("FORWARD_BUFFER_SIZE", 32768),
		AuditQueueSize:                  getEnvInt("AUDIT_QUEUE_SIZE", 1000),
		MaxHTTPLogs:                     getEnvInt("MAX_HTTP_LOGS", 1000),
		HTTPPairCleanupIntervalMinutes:  getEnvInt("HTTP_PAIR_CLEANUP_INTERVAL_MINUTES", 5),
		HTTPPairMaxAgeMinutes:           getEnvInt("HTTP_PAIR_MAX_AGE_MINUTES", 10),
		GoroutineMonitorIntervalSeconds: getEnvInt("GOROUTINE_MONITOR_INTERVAL_SECONDS", 30),
		GoroutineWarnThreshold:          getEnvInt("GOROUTINE_WARN_THRESHOLD", 1000),
		Socks5HandshakeTimeoutSeconds:   getEnvInt("SOCKS5_HANDSHAKE_TIMEOUT_SECONDS", 30),
		SessionIdleTimeoutHours:         getEnvInt("SESSION_IDLE_TIMEOUT_HOURS", 24),
		SSHConnectMaxRetries:            getEnvInt("SSH_CONNECT_MAX_RETRIES", 3),
		SSHConnectRetryDelaySeconds:     getEnvInt("SSH_CONNECT_RETRY_DELAY_SECONDS", 2),

		HTTPGzipDecodeMaxBytes:     getEnvInt("HTTP_GZIP_DECODE_MAX_BYTES", 1048576),
		HTTPGzipDecodeTimeoutMS:    getEnvInt("HTTP_GZIP_DECODE_TIMEOUT_MS", 500),
		HTTPGzipDecodeCacheSeconds: getEnvInt("HTTP_GZIP_DECODE_CACHE_SECONDS", 60),
	}
}

// ParseFlags parses command-line flags, applies any overrides to the package-level Settings, and updates configuration accordingly.
// It also provides a custom usage message and handles --help (prints usage and exits) and --version (prints build info and exits).
func ParseFlags() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Bastion V3 - Go implementation\n\n")
		fmt.Fprintf(out, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(out, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(out, "\nEnvironment variables:")
		fmt.Fprintln(out, "  LOG_LEVEL                         Log level (DEBUG, INFO, WARN, ERROR)")
		fmt.Fprintln(out, "  PORT                              HTTP server port (default 7788)")
		fmt.Fprintln(out, "  DATABASE_URL                      SQLite database path (default bastion.db)")
		fmt.Fprintln(out, "  SQLITE_PRAGMAS_ENABLED            Enable SQLite PRAGMAs (true/false, default true)")
		fmt.Fprintln(out, "  SQLITE_BUSY_TIMEOUT_MS            SQLite busy_timeout in milliseconds (default 5000)")
		fmt.Fprintln(out, "  SQLITE_JOURNAL_MODE               SQLite journal_mode (default WAL)")
		fmt.Fprintln(out, "  SQLITE_SYNCHRONOUS                SQLite synchronous (default NORMAL)")
		fmt.Fprintln(out, "  SQLITE_FOREIGN_KEYS               Enable SQLite foreign_keys (true/false, default true)")
		fmt.Fprintln(out, "  SQLITE_MAX_OPEN_CONNS             SQLite MaxOpenConns (default 1)")
		fmt.Fprintln(out, "  SQLITE_MAX_IDLE_CONNS             SQLite MaxIdleConns (default 1)")
		fmt.Fprintln(out, "  SQLITE_CONN_MAX_IDLE_SECONDS      SQLite ConnMaxIdleTime in seconds (default 300)")
		fmt.Fprintln(out, "  SQLITE_CONN_MAX_LIFETIME_SECONDS  SQLite ConnMaxLifetime in seconds (default 0)")
		fmt.Fprintln(out, "  SSH_CONNECT_TIMEOUT               SSH connect timeout in seconds (default 15)")
		fmt.Fprintln(out, "  SSH_KEEPALIVE_INTERVAL            SSH keepalive interval in seconds (default 30)")
		fmt.Fprintln(out, "  AUDIT_ENABLED                     Enable HTTP audit logging (true/false, default true)")
		fmt.Fprintln(out, "  MAX_SESSION_CONNECTIONS           Maximum concurrent connections per session (default 1000)")
		fmt.Fprintln(out, "  FORWARD_BUFFER_SIZE               TCP forward buffer size in bytes (default 32768)")
		fmt.Fprintln(out, "  AUDIT_QUEUE_SIZE                  HTTP audit queue size (default 1000)")
		fmt.Fprintln(out, "  MAX_HTTP_LOGS                     Maximum in-memory HTTP logs (default 1000)")
		fmt.Fprintln(out, "  HTTP_PAIR_CLEANUP_INTERVAL_MINUTES  Interval minutes to cleanup stale HTTP pairs (default 5)")
		fmt.Fprintln(out, "  HTTP_PAIR_MAX_AGE_MINUTES        Max age minutes before HTTP pair is considered stale (default 10)")
		fmt.Fprintln(out, "  GOROUTINE_MONITOR_INTERVAL_SECONDS Interval seconds for goroutine monitor (default 30)")
		fmt.Fprintln(out, "  GOROUTINE_WARN_THRESHOLD         Goroutine count warning threshold (default 1000)")
		fmt.Fprintln(out, "  SOCKS5_HANDSHAKE_TIMEOUT_SECONDS SOCKS5 handshake timeout in seconds (default 30)")
		fmt.Fprintln(out, "  SESSION_IDLE_TIMEOUT_HOURS       Session idle timeout in hours (default 24)")
		fmt.Fprintln(out, "  SSH_CONNECT_MAX_RETRIES          Max SSH connect retries per hop (default 3)")
		fmt.Fprintln(out, "  SSH_CONNECT_RETRY_DELAY_SECONDS  Delay between SSH connect retries in seconds (default 2)")
		fmt.Fprintln(out, "  HTTP_GZIP_DECODE_MAX_BYTES       Max decompressed bytes for on-demand gzip decode (default 1048576)")
		fmt.Fprintln(out, "  HTTP_GZIP_DECODE_TIMEOUT_MS      Timeout for on-demand gzip decode in ms (default 500)")
		fmt.Fprintln(out, "  HTTP_GZIP_DECODE_CACHE_SECONDS   Sliding cache TTL seconds for decoded results (default 60)")
	}

	port := flag.Int("port", Settings.Port, "HTTP server port (overrides PORT)")
	db := flag.String("db", Settings.DatabaseURL, "SQLite database path (overrides DATABASE_URL)")
	sqlitePragmasEnabled := flag.Bool("sqlite-pragmas", Settings.SQLitePragmasEnabled, "Enable SQLite PRAGMAs (overrides SQLITE_PRAGMAS_ENABLED)")
	sqliteBusyTimeoutMS := flag.Int("sqlite-busy-timeout-ms", Settings.SQLiteBusyTimeoutMS, "SQLite busy_timeout in milliseconds (overrides SQLITE_BUSY_TIMEOUT_MS)")
	sqliteJournalMode := flag.String("sqlite-journal-mode", Settings.SQLiteJournalMode, "SQLite journal_mode (overrides SQLITE_JOURNAL_MODE)")
	sqliteSynchronous := flag.String("sqlite-synchronous", Settings.SQLiteSynchronous, "SQLite synchronous (overrides SQLITE_SYNCHRONOUS)")
	sqliteForeignKeys := flag.Bool("sqlite-foreign-keys", Settings.SQLiteForeignKeys, "Enable SQLite foreign_keys PRAGMA (overrides SQLITE_FOREIGN_KEYS)")
	sqliteMaxOpenConns := flag.Int("sqlite-max-open-conns", Settings.SQLiteMaxOpenConns, "SQLite MaxOpenConns (overrides SQLITE_MAX_OPEN_CONNS)")
	sqliteMaxIdleConns := flag.Int("sqlite-max-idle-conns", Settings.SQLiteMaxIdleConns, "SQLite MaxIdleConns (overrides SQLITE_MAX_IDLE_CONNS)")
	sqliteConnMaxIdleSec := flag.Int("sqlite-conn-max-idle-seconds", Settings.SQLiteConnMaxIdleSec, "SQLite ConnMaxIdleTime in seconds (overrides SQLITE_CONN_MAX_IDLE_SECONDS)")
	sqliteConnMaxLifeSec := flag.Int("sqlite-conn-max-lifetime-seconds", Settings.SQLiteConnMaxLifeSec, "SQLite ConnMaxLifetime in seconds (overrides SQLITE_CONN_MAX_LIFETIME_SECONDS)")
	logLevel := flag.String("log-level", Settings.LogLevel, "Log level: DEBUG, INFO, WARN, ERROR (overrides LOG_LEVEL)")
	logFile := flag.String("log-file", Settings.LogFilePath, "Log file path (overrides LOG_FILE)")
	auditEnabled := flag.Bool("audit", Settings.AuditEnabled, "Enable HTTP traffic auditing (overrides AUDIT_ENABLED)")
	cliMode := flag.Bool("cli", Settings.CLIMode, "Run in CLI mode (HTTP client only, no database)")
	cliServer := flag.String("server", "http://localhost:7788", "Server URL for CLI mode")

	maxSessionConns := flag.Int("max-session-connections", Settings.MaxSessionConnections, "Maximum concurrent connections per mapping session")
	maxHTTPLogs := flag.Int("max-http-logs", Settings.MaxHTTPLogs, "Maximum number of HTTP logs kept in memory")

	showHelp := flag.Bool("help", false, "Show help and exit")
	showVersion := flag.Bool("version", false, "Show version and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println(version.GetBuildInfo())
		os.Exit(0)
	}

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	Settings.Port = *port
	Settings.DatabaseURL = *db
	Settings.SQLitePragmasEnabled = *sqlitePragmasEnabled
	Settings.SQLiteBusyTimeoutMS = *sqliteBusyTimeoutMS
	Settings.SQLiteJournalMode = *sqliteJournalMode
	Settings.SQLiteSynchronous = *sqliteSynchronous
	Settings.SQLiteForeignKeys = *sqliteForeignKeys
	Settings.SQLiteMaxOpenConns = *sqliteMaxOpenConns
	Settings.SQLiteMaxIdleConns = *sqliteMaxIdleConns
	Settings.SQLiteConnMaxIdleSec = *sqliteConnMaxIdleSec
	Settings.SQLiteConnMaxLifeSec = *sqliteConnMaxLifeSec
	Settings.LogLevel = *logLevel
	Settings.LogFilePath = *logFile
	Settings.AuditEnabled = *auditEnabled
	Settings.CLIMode = *cliMode
	Settings.CLIServer = *cliServer
	Settings.MaxSessionConnections = *maxSessionConns
	Settings.MaxHTTPLogs = *maxHTTPLogs
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
