package main

import (
	"bastion/cli"
	"bastion/config"
	"bastion/core"
	"bastion/database"
	"bastion/handlers"
	"bastion/service"
	"bastion/state"
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/creativeprojects/go-selfupdate/update"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// Self-update helper mode must run before flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "--self-update-helper" {
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		runSelfUpdateHelper(os.Args[2:])
		return
	}

	// Load environment variables and parse CLI flags
	config.ParseFlags()

	logFile, err := setupLogging(config.Settings.LogFilePath)
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	// Check if CLI mode is requested
	if config.Settings.CLIMode {
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		mainCLI()
		return
	}

	// Configure log format
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("System starting up...")

	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Start auditor
	core.AuditorInstance.Start()

	// Start SSH pool housekeeping (keepalive probes and idle reclamation).
	core.Pool.StartHousekeeping()

	// Initialize services
	service.InitServices(database.DB, state.Global, core.AuditorInstance)

	// Auto-start mappings marked for autostart
	if err := service.GlobalServices.Mapping.StartAutoStartMappings(); err != nil {
		log.Printf("Warning: Failed to start auto-start mappings: %v", err)
	}

	// Start goroutine monitor
	go monitorGoroutines()

	// Set Gin mode
	if config.Settings.LogLevel != "DEBUG" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Direct Gin logs to the configured log file
	gin.DefaultWriter = log.Writer()
	gin.DefaultErrorWriter = log.Writer()

	// Disable Gin color logs to avoid ANSI issues on Windows terminals
	gin.DisableConsoleColor()

	// Create router
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Static file service using embedded FS
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static file system: %v", err)
	}
	r.StaticFS("/web", http.FS(staticFS))

	// Root path redirect
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web/index.html")
	})

	// Prometheus metrics (exposition format)
	r.GET("/metrics", handlers.GetPrometheusMetrics)

	// API routes
	api := r.Group("/api")
	{
		// Bastion routes
		api.GET("/bastions", handlers.ListBastions)
		api.POST("/bastions", handlers.CreateBastion)
		api.PUT("/bastions/:id", handlers.UpdateBastion)
		api.DELETE("/bastions/:id", handlers.DeleteBastion)

		// Mapping routes
		api.GET("/mappings", handlers.ListMappings)
		api.POST("/mappings", handlers.CreateMapping)
		api.PUT("/mappings/:id", handlers.UpdateMapping)
		api.DELETE("/mappings/:id", handlers.DeleteMapping)
		api.POST("/mappings/:id/start", handlers.StartMapping)
		api.POST("/mappings/:id/stop", handlers.StopMapping)

		// Stats routes
		api.GET("/stats", handlers.GetStats)

		// HTTP log routes
		api.GET("/http-logs", handlers.GetHTTPLogs)
		api.GET("/http-logs/:id", handlers.GetHTTPLogDetail)
		api.DELETE("/http-logs", handlers.ClearHTTPLogs)

		// Error log routes
		api.GET("/error-logs", handlers.GetErrorLogs)
		api.DELETE("/error-logs", handlers.ClearErrorLogs)

		// System shutdown routes
		api.POST("/shutdown/generate-code", handlers.GenerateShutdownCode)
		api.POST("/shutdown/verify", handlers.VerifyAndShutdown)

		// Health and metrics routes
		api.GET("/health", handlers.HealthCheck)
		api.GET("/metrics", handlers.GetMetrics)

		// Self-update routes
		api.GET("/update/check", handlers.CheckUpdate)
		api.GET("/update/proxy", handlers.GetUpdateProxy)
		api.POST("/update/proxy", handlers.SetUpdateProxy)
		api.POST("/update/generate-code", handlers.GenerateUpdateCode)
		api.POST("/update/apply", handlers.ApplyUpdate)
	}

	// Find an available port
	port := findAvailablePort(config.Settings.Port)
	if port != config.Settings.Port {
		log.Printf("Default port %d is busy. Switched to %d", config.Settings.Port, port)
	}

	// Create HTTP server
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on http://127.0.0.1:%d", port)
		log.Printf("Open browser at: http://127.0.0.1:%d/", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Optionally open browser automatically
	go func() {
		time.Sleep(1500 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://127.0.0.1:%d/", port))
	}()

	// Create shutdown channel and expose to handlers
	shutdownChan := make(chan bool, 1)
	handlers.SetShutdownChannel(shutdownChan)

	// Wait for OS interrupt or API-triggered shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Received interrupt signal")
	case <-shutdownChan:
		log.Println("Shutdown triggered via API")
	}

	log.Println("System shutting down...")

	// Stop auditor
	core.AuditorInstance.Stop()

	// Stop all sessions
	state.Global.Lock()
	sessionIDs := make([]string, 0, len(state.Global.Sessions))
	for id := range state.Global.Sessions {
		sessionIDs = append(sessionIDs, id)
	}
	state.Global.Unlock()

	// Stop sessions outside the lock to avoid deadlocks
	for _, id := range sessionIDs {
		log.Printf("Stopping session: %s", id)
		state.Global.RemoveAndStopSession(id)
	}

	// Close all SSH connections
	core.Pool.CloseAll()

	// Close database connection
	if err := database.CloseDB(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	// Gracefully shut down HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func runSelfUpdateHelper(args []string) {
	target := ""
	source := ""
	cleanup := ""
	helperLog := ""
	parentPID := 0
	restart := false
	var restartArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--target":
			if i+1 < len(args) {
				target = args[i+1]
				i++
			}
		case "--source":
			if i+1 < len(args) {
				source = args[i+1]
				i++
			}
		case "--cleanup":
			if i+1 < len(args) {
				cleanup = args[i+1]
				i++
			}
		case "--helper-log":
			if i+1 < len(args) {
				helperLog = args[i+1]
				i++
			}
		case "--parent-pid":
			if i+1 < len(args) {
				parentPID = parseInt(args[i+1])
				i++
			}
		case "--restart":
			restart = true
		case "--":
			restartArgs = append(restartArgs, args[i+1:]...)
			i = len(args)
		}
	}

	if target == "" || source == "" {
		log.Printf("update-helper: missing --target/--source, exiting")
		return
	}

	closeLog := setupHelperLogging(helperLog, target)
	if closeLog != nil {
		defer closeLog()
	}

	log.Printf(
		"update-helper: start target=%s source=%s cleanup=%s restart=%v",
		target, source, cleanup, restart,
	)

	// Retry replacement for up to 5 minutes (needed on Windows where the binary is locked while running).
	deadline := time.Now().Add(5 * time.Minute)
	lastLog := time.Time{}
	for {
		if err := applyExecutableUpdate(target, source); err == nil {
			log.Printf("update-helper: replaced successfully")
			break
		} else if time.Since(lastLog) > 2*time.Second {
			log.Printf("update-helper: replace failed (will retry): %v", err)
			lastLog = time.Now()
		}
		if time.Now().After(deadline) {
			log.Printf("update-helper: replace deadline reached, exiting")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	if cleanup != "" {
		log.Printf("update-helper: cleaning up %s", cleanup)
		_ = os.RemoveAll(cleanup)
	}

	if restart {
		waitForParentExit(parentPID, 30*time.Second)
		log.Printf("update-helper: restarting %s args=%v", target, restartArgs)
		cmd := exec.Command(target, restartArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Printf("update-helper: restart failed: %v", err)
			return
		}
		log.Printf("update-helper: restart started (pid=%d)", cmd.Process.Pid)
	}
}

func setupHelperLogging(explicitPath, target string) func() {
	candidates := []string{}
	if strings.TrimSpace(explicitPath) != "" {
		candidates = append(candidates, explicitPath)
	}
	if v := strings.TrimSpace(os.Getenv("LOG_FILE")); v != "" {
		candidates = append(candidates, v)
	}
	candidates = append(candidates, filepath.Join(filepath.Dir(target), "bastion-update.log"))
	candidates = append(candidates, "bastion-update.log")
	candidates = append(candidates, filepath.Join(os.TempDir(), "bastion-update.log"))

	for _, path := range candidates {
		if strings.TrimSpace(path) == "" {
			continue
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			continue
		}
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		log.Printf("update-helper: logging to %s", path)
		return func() { _ = f.Close() }
	}

	log.Printf("update-helper: logging to stderr only (failed to open log file)")
	return nil
}

func applyExecutableUpdate(target, source string) error {
	fp, err := os.Open(source)
	if err != nil {
		return err
	}
	defer fp.Close()

	mode := os.FileMode(0o755)
	if fi, err := os.Stat(target); err == nil {
		mode = fi.Mode()
	}

	err = update.Apply(fp, update.Options{
		TargetPath: target,
		TargetMode: mode,
	})
	if err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			log.Printf("update-helper: rollback failed: %v", rerr)
		}
		return err
	}

	_ = os.Remove(source)
	return nil
}

func parseInt(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// findAvailablePort searches for an available port
func findAvailablePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port
		}
	}
	log.Fatal("No available ports found")
	return startPort
}

// openBrowser opens the default browser
func openBrowser(url string) {
	var err error
	switch {
	case fileExists("/usr/bin/xdg-open"):
		err = runCommand("xdg-open", url)
	case fileExists("/usr/bin/open"):
		err = runCommand("open", url)
	default:
		// Windows
		err = runCommand("cmd", "/c", "start", url)
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please manually open: %s", url)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait asynchronously to avoid zombie processes
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Browser process exited with error: %v", err)
		}
	}()

	return nil
}

// monitorGoroutines tracks goroutine count to prevent leaks
func monitorGoroutines() {
	ticker := time.NewTicker(time.Duration(config.Settings.GoroutineMonitorIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		count := runtime.NumGoroutine()
		if count > config.Settings.GoroutineWarnThreshold {
			log.Printf("WARNING: High goroutine count detected: %d", count)
		} else if config.Settings.LogLevel == "DEBUG" {
			log.Printf("Current goroutine count: %d", count)
		}
	}
}

// mainCLI entrypoint for CLI (HTTP client mode)
func mainCLI() {
	// CLI mode skips DB load; acts as HTTP client
	log.SetFlags(log.Ldate | log.Ltime)

	// Fetch server address
	serverURL := config.Settings.CLIServer

	fmt.Printf("Bastion V3 CLI - Connecting to %s\n", serverURL)

	// Create HTTP client CLI instance
	cliInstance, err := cli.NewCLIHttp(serverURL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("\nTips:")
		fmt.Println("  1. Make sure the Bastion server is running:")
		fmt.Println("     ./bastion")
		fmt.Println("  2. Or specify a different server:")
		fmt.Printf("     ./bastion --cli --server http://your-server:7788\n")
		os.Exit(1)
	}

	// Start CLI loop (readline handles Ctrl+C automatically)
	cliInstance.Start()
}
