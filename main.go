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
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
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
