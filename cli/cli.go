package cli

import (
	"bastion/config"
	"bastion/core"
	"bastion/models"
	"bastion/service"
	"bastion/state"
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// CLI command line interactive interface
type CLI struct {
	scanner *bufio.Scanner
	running bool
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{
		scanner: bufio.NewScanner(os.Stdin),
		running: true,
	}
}

// Start starts the CLI main loop
func (c *CLI) Start() {
	c.printWelcome()

	for c.running {
		fmt.Print("\n> ")
		if !c.scanner.Scan() {
			break
		}

		input := strings.TrimSpace(c.scanner.Text())
		if input == "" {
			continue
		}

		c.handleCommand(input)
	}
}

// printWelcome prints welcome message
func (c *CLI) printWelcome() {
	PrintBanner("Bastion V3 - CLI Mode (Headless)")
	fmt.Println("\nType 'help' for available commands")
}

// handleCommand handles user command
func (c *CLI) handleCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "help", "h", "?":
		c.showHelp()
	case "bastion":
		c.handleBastionCommand(args)
	case "mapping", "map":
		c.handleMappingCommand(args)
	case "start":
		c.handleStartCommand(args)
	case "stop":
		c.handleStopCommand(args)
	case "status", "st":
		c.handleStatusCommand()
	case "stats":
		c.handleStatsCommand()
	case "http", "logs":
		c.handleHTTPCommand(args)
	case "clear":
		c.clearScreen()
	case "exit", "quit", "q":
		c.handleExit()
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", cmd)
	}
}

// showHelp displays help information
func (c *CLI) showHelp() {
	fmt.Println()
	PrintBanner("Available Commands")
	fmt.Println()

	commands := [][]string{
		{"help, h, ?", "Show this help message"},
		{"", ""},
		{"BASTION MANAGEMENT:", ""},
		{"bastion list", "List all bastions"},
		{"bastion add", "Add a new bastion (interactive)"},
		{"bastion delete <id>", "Delete a bastion by ID"},
		{"bastion show <id>", "Show bastion details"},
		{"", ""},
		{"MAPPING MANAGEMENT:", ""},
		{"mapping list", "List all mappings"},
		{"mapping add", "Add a new mapping (interactive)"},
		{"mapping delete <id>", "Delete a mapping by ID"},
		{"mapping show <id>", "Show mapping details"},
		{"", ""},
		{"SESSION CONTROL:", ""},
		{"start <mapping_id>", "Start a mapping session"},
		{"stop <mapping_id>", "Stop a mapping session"},
		{"status", "Show all sessions status"},
		{"stats", "Show traffic statistics"},
		{"", ""},
		{"HTTP AUDIT:", ""},
		{"http list [page]", "List HTTP logs (paginated)"},
		{"http search [keyword] [--local-port <port>] [--bastion <name>] [--url <url>] [page]", "Search HTTP logs (multi-dimensional filters)"},
		{"http show <id>", "Show HTTP request/response details"},
		{"http clear", "Clear all HTTP logs"},
		{"", ""},
		{"SYSTEM:", ""},
		{"clear", "Clear screen"},
		{"exit, quit, q", "Exit the program"},
	}

	// Simple formatted output instead of table
	for _, cmd := range commands {
		if len(cmd) == 2 && cmd[0] != "" {
			fmt.Printf("  %-30s %s\n", cmd[0], cmd[1])
		} else {
			fmt.Println()
		}
	}
}

// handleBastionCommand handles bastion-related commands
func (c *CLI) handleBastionCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: bastion <list|add|delete|show> [args]")
		return
	}

	switch args[0] {
	case "list", "ls":
		c.listBastions()
	case "add", "create":
		c.addBastion()
	case "delete", "del", "rm":
		if len(args) < 2 {
			fmt.Println("Usage: bastion delete <id>")
			return
		}
		c.deleteBastion(args[1])
	case "show", "get":
		if len(args) < 2 {
			fmt.Println("Usage: bastion show <id>")
			return
		}
		c.showBastion(args[1])
	default:
		fmt.Printf("Unknown bastion command: %s\n", args[0])
	}
}

// listBastions lists all bastions
func (c *CLI) listBastions() {
	bastions, err := service.GlobalServices.Bastion.List()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(bastions) == 0 {
		fmt.Println("No bastions configured.")
		return
	}

	fmt.Println()
	PrintBanner(fmt.Sprintf("Total Bastions: %d", len(bastions)))
	fmt.Println()

	fmt.Printf("%-4s %-20s %-20s %-6s %-15s %-10s\n", "ID", "Name", "Host", "Port", "Username", "Auth")
	fmt.Println(strings.Repeat("-", 80))

	for _, b := range bastions {
		auth := "Password"
		if b.PkeyPath != "" {
			auth = "Key"
		}
		fmt.Printf("%-4d %-20s %-20s %-6d %-15s %-10s\n",
			b.ID,
			truncate(b.Name, 20),
			truncate(b.Host, 20),
			b.Port,
			truncate(b.Username, 15),
			auth,
		)
	}
}

// addBastion adds a bastion interactively
func (c *CLI) addBastion() {
	fmt.Println()
	PrintBanner("Add New Bastion (Interactive)")
	fmt.Println("\nNote: In headless mode, use Ctrl+C to cancel the entire operation")
	fmt.Println("      Type 'cancel' at any prompt to abort")

	bastion := models.BastionCreate{}

	// Name
	bastion.Name = c.readInput("Name (optional, will auto-generate if empty)", "")
	if bastion.Name == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}

	// Host
	bastion.Host = c.readInput("Host (required)", "")
	if bastion.Host == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	for bastion.Host == "" {
		fmt.Println("Host is required!")
		bastion.Host = c.readInput("Host (required)", "")
		if bastion.Host == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
	}

	// Port
	portStr := c.readInput("Port", "22")
	if portStr == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 22
	}
	bastion.Port = port

	// Username
	bastion.Username = c.readInput("Username (required)", "")
	if bastion.Username == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	for bastion.Username == "" {
		fmt.Println("Username is required!")
		bastion.Username = c.readInput("Username (required)", "")
		if bastion.Username == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
	}

	// Auth method
	authMethod := c.readInput("Auth method (1=Password, 2=SSH Key)", "1")
	if authMethod == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	if authMethod == "2" {
		bastion.PkeyPath = c.readInput("SSH Key Path", "")
		if bastion.PkeyPath == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		bastion.PkeyPassphrase = c.readInput("Key Passphrase (optional)", "")
		if bastion.PkeyPassphrase == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
	} else {
		bastion.Password = c.readInputPassword("Password")
		if bastion.Password == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
	}

	// Normalize and create
	bastion.Normalize()

	createdBastion, err := service.GlobalServices.Bastion.Create(bastion)
	if err != nil {
		fmt.Printf("Error creating bastion: %v\n", err)
		return
	}

	fmt.Printf("\n✓ Bastion created successfully! ID: %d\n", createdBastion.ID)
}

// deleteBastion removes a bastion
func (c *CLI) deleteBastion(idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", idStr)
		return
	}

	bastion, err := service.GlobalServices.Bastion.Get(uint(id))
	if err != nil {
		fmt.Printf("Bastion not found: %d\n", id)
		return
	}

	// Check if in use - get all mappings to check
	mappings, _ := service.GlobalServices.Mapping.List()
	runningMap := make(map[string]bool)
	for _, m := range mappings {
		if m.Running {
			runningMap[m.ID] = true
		}
	}

	inUse, runningMappings, totalMappings, err := service.GlobalServices.Bastion.CheckInUse(bastion.Name, runningMap)
	if err != nil {
		fmt.Printf("Error checking bastion usage: %v\n", err)
		return
	}

	if inUse {
		fmt.Printf("Cannot delete bastion '%s': used by %d mapping(s)\n", bastion.Name, totalMappings)
		if len(runningMappings) > 0 {
			fmt.Printf("Active mappings: %s\n", strings.Join(runningMappings, ", "))
		}
		return
	}

	confirm := c.readInput(fmt.Sprintf("Delete bastion '%s'? (yes/no)", bastion.Name), "no")
	if strings.ToLower(confirm) != "yes" && strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return
	}

	if err := service.GlobalServices.Bastion.Delete(uint(id)); err != nil {
		fmt.Printf("Error deleting bastion: %v\n", err)
		return
	}

	fmt.Println("✓ Bastion deleted successfully!")
}

// showBastion displays bastion details
func (c *CLI) showBastion(idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", idStr)
		return
	}

	bastion, err := service.GlobalServices.Bastion.Get(uint(id))
	if err != nil {
		fmt.Printf("Bastion not found: %d\n", id)
		return
	}

	fmt.Println()
	PrintBanner(fmt.Sprintf("Bastion Details: %s", bastion.Name))
	fmt.Println()

	fmt.Printf("ID:         %d\n", bastion.ID)
	fmt.Printf("Name:       %s\n", bastion.Name)
	fmt.Printf("Host:       %s\n", bastion.Host)
	fmt.Printf("Port:       %d\n", bastion.Port)
	fmt.Printf("Username:   %s\n", bastion.Username)

	if bastion.PkeyPath != "" {
		fmt.Printf("Auth:       SSH Key (%s)\n", bastion.PkeyPath)
	} else {
		fmt.Printf("Auth:       Password\n")
	}
}

// handleMappingCommand handles mapping-related commands
func (c *CLI) handleMappingCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mapping <list|add|delete|show> [args]")
		return
	}

	switch args[0] {
	case "list", "ls":
		c.listMappings()
	case "add", "create":
		c.addMapping()
	case "delete", "del", "rm":
		if len(args) < 2 {
			fmt.Println("Usage: mapping delete <id>")
			return
		}
		c.deleteMapping(args[1])
	case "show", "get":
		if len(args) < 2 {
			fmt.Println("Usage: mapping show <id>")
			return
		}
		c.showMapping(args[1])
	default:
		fmt.Printf("Unknown mapping command: %s\n", args[0])
	}
}

// listMappings lists all mappings
func (c *CLI) listMappings() {
	mappingsWithStatus, err := service.GlobalServices.Mapping.List()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(mappingsWithStatus) == 0 {
		fmt.Println("No mappings configured.")
		return
	}

	fmt.Println()
	PrintBanner(fmt.Sprintf("Total Mappings: %d", len(mappingsWithStatus)))
	fmt.Println()

	fmt.Printf("%-20s %-18s %-18s %-8s %-25s %-10s\n", "ID", "Local", "Remote", "Type", "Chain", "Status")
	fmt.Println(strings.Repeat("-", 110))

	for _, mws := range mappingsWithStatus {
		status := "Stopped"
		if mws.Running {
			status = "Running"
		}

		chain := strings.Join(mws.Chain, " → ")

		fmt.Printf("%-20s %-18s %-18s %-8s %-25s %-10s\n",
			truncate(mws.ID, 20),
			fmt.Sprintf("%s:%d", mws.LocalHost, mws.LocalPort),
			fmt.Sprintf("%s:%d", mws.RemoteHost, mws.RemotePort),
			mws.Type,
			truncate(chain, 25),
			status,
		)
	}
}

// addMapping adds a mapping interactively
func (c *CLI) addMapping() {
	fmt.Println()
	PrintBanner("Add New Mapping (Interactive)")
	fmt.Println("\nNote: In headless mode, use Ctrl+C to cancel the entire operation")
	fmt.Println("      Type 'cancel' at any prompt to abort")

	mapping := models.MappingCreate{}

	// Local Host
	mapping.LocalHost = c.readInput("Local Host", "127.0.0.1")
	if mapping.LocalHost == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	if mapping.LocalHost == "" {
		mapping.LocalHost = "127.0.0.1"
	}

	// Local Port
	localPortStr := c.readInput("Local Port (required)", "")
	if localPortStr == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	for localPortStr == "" {
		fmt.Println("Local Port is required!")
		localPortStr = c.readInput("Local Port (required)", "")
		if localPortStr == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
	}
	localPort, _ := strconv.Atoi(localPortStr)
	if localPort == 0 {
		fmt.Println("Invalid port number!")
		return
	}
	mapping.LocalPort = localPort

	// Type
	typeStr := c.readInput("Type (1=TCP, 2=SOCKS5)", "1")
	if typeStr == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	if typeStr == "2" {
		mapping.Type = "socks5"
	} else {
		mapping.Type = "tcp"
	}

	// Remote (only for TCP)
	if mapping.Type == "tcp" {
		mapping.RemoteHost = c.readInput("Remote Host (required)", "")
		if mapping.RemoteHost == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		for mapping.RemoteHost == "" {
			fmt.Println("Remote Host is required for TCP mapping!")
			mapping.RemoteHost = c.readInput("Remote Host (required)", "")
			if mapping.RemoteHost == "cancel" {
				fmt.Println("\n❌ Operation cancelled")
				return
			}
		}

		remotePortStr := c.readInput("Remote Port (required)", "")
		if remotePortStr == "cancel" {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		for remotePortStr == "" {
			fmt.Println("Remote Port is required!")
			remotePortStr = c.readInput("Remote Port (required)", "")
			if remotePortStr == "cancel" {
				fmt.Println("\n❌ Operation cancelled")
				return
			}
		}
		remotePort, _ := strconv.Atoi(remotePortStr)
		if remotePort == 0 {
			fmt.Println("Invalid port number!")
			return
		}
		mapping.RemotePort = remotePort
	}

	// Chain
	fmt.Println("\nAvailable Bastions:")
	bastions, _ := service.GlobalServices.Bastion.List()
	for i, b := range bastions {
		fmt.Printf("  %d. %s (%s:%d)\n", i+1, b.Name, b.Host, b.Port)
	}

	chainStr := c.readInput("Bastion chain (comma-separated names or numbers)", "")
	if chainStr == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	if chainStr != "" {
		chainParts := strings.Split(chainStr, ",")
		for _, part := range chainParts {
			part = strings.TrimSpace(part)
			// Try as number first
			if idx, err := strconv.Atoi(part); err == nil && idx > 0 && idx <= len(bastions) {
				mapping.Chain = append(mapping.Chain, bastions[idx-1].Name)
			} else {
				// Use as name
				mapping.Chain = append(mapping.Chain, part)
			}
		}
	}

	// ID
	mapping.ID = c.readInput("Mapping ID (optional, will auto-generate)", "")
	if mapping.ID == "cancel" {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	if mapping.ID == "" {
		mapping.ID = fmt.Sprintf("%s:%d", mapping.LocalHost, mapping.LocalPort)
	}

	// Normalize and create
	mapping.Normalize()

	createdMapping, err := service.GlobalServices.Mapping.Create(mapping)
	if err != nil {
		fmt.Printf("Error creating mapping: %v\n", err)
		return
	}

	fmt.Printf("\n✓ Mapping created successfully! ID: %s\n", createdMapping.ID)
}

// deleteMapping removes a mapping
func (c *CLI) deleteMapping(id string) {
	if err := service.GlobalServices.Mapping.Delete(id); err != nil {
		fmt.Printf("Error deleting mapping: %v\n", err)
		return
	}

	fmt.Println("✓ Mapping deleted successfully!")
}

// showMapping displays mapping details
func (c *CLI) showMapping(id string) {
	mapping, err := service.GlobalServices.Mapping.Get(id)
	if err != nil {
		fmt.Printf("Mapping not found: %s\n", id)
		return
	}

	running := state.Global.SessionExists(id)

	fmt.Println()
	PrintBanner(fmt.Sprintf("Mapping Details: %s", mapping.ID))
	fmt.Println()

	fmt.Printf("ID:          %s\n", mapping.ID)
	fmt.Printf("Type:        %s\n", mapping.Type)
	fmt.Printf("Local:       %s:%d\n", mapping.LocalHost, mapping.LocalPort)

	if mapping.Type == "tcp" {
		fmt.Printf("Remote:      %s:%d\n", mapping.RemoteHost, mapping.RemotePort)
	}

	chain := mapping.GetChain()
	if len(chain) > 0 {
		fmt.Printf("Chain:       %s\n", strings.Join(chain, " → "))
	}

	if running {
		fmt.Printf("Status:      Running ✓\n")

		if session, exists := state.Global.GetSession(id); exists {
			stats := session.GetStats()
			fmt.Printf("\nStatistics:\n")
			fmt.Printf("  Active Connections: %d\n", stats.ActiveConns)
			fmt.Printf("  Bytes Up:           %s\n", formatBytes(stats.BytesUp))
			fmt.Printf("  Bytes Down:         %s\n", formatBytes(stats.BytesDown))
		}
	} else {
		fmt.Printf("Status:      Stopped\n")
	}
}

// handleStartCommand starts a mapping
func (c *CLI) handleStartCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: start <mapping_id>")
		return
	}

	id := args[0]

	fmt.Printf("Starting mapping %s...\n", id)
	if err := service.GlobalServices.Mapping.Start(id); err != nil {
		fmt.Printf("Error starting mapping: %v\n", err)
		return
	}

	// Get mapping details to display
	mapping, _ := service.GlobalServices.Mapping.Get(id)

	fmt.Printf("✓ Mapping started successfully!\n")
	fmt.Printf("  Local endpoint: %s:%d\n", mapping.LocalHost, mapping.LocalPort)
}

// handleStopCommand stops a mapping
func (c *CLI) handleStopCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: stop <mapping_id>")
		return
	}

	id := args[0]

	fmt.Printf("Stopping mapping %s...\n", id)
	if err := service.GlobalServices.Mapping.Stop(id); err != nil {
		fmt.Printf("Error stopping mapping: %v\n", err)
		return
	}
	fmt.Println("✓ Mapping stopped successfully!")
}

// handleStatusCommand shows all session states
func (c *CLI) handleStatusCommand() {
	state.Global.RLock()
	sessionCount := len(state.Global.Sessions)
	sessionIDs := make([]string, 0, sessionCount)
	for id := range state.Global.Sessions {
		sessionIDs = append(sessionIDs, id)
	}
	state.Global.RUnlock()

	fmt.Println()
	PrintBanner(fmt.Sprintf("Active Sessions: %d", sessionCount))
	fmt.Println()

	if sessionCount == 0 {
		fmt.Println("No active sessions.")
		return
	}

	fmt.Printf("%-20s %-15s %-15s %-15s\n", "Mapping ID", "Connections", "Bytes Up", "Bytes Down")
	fmt.Println(strings.Repeat("-", 70))

	for _, id := range sessionIDs {
		if session, exists := state.Global.GetSession(id); exists {
			stats := session.GetStats()
			fmt.Printf("%-20s %-15d %-15s %-15s\n",
				truncate(id, 20),
				stats.ActiveConns,
				formatBytes(stats.BytesUp),
				formatBytes(stats.BytesDown),
			)
		}
	}
}

// handleStatsCommand shows traffic statistics
func (c *CLI) handleStatsCommand() {
	statsMap := service.GlobalServices.Mapping.GetStats()

	var totalConns int32
	var totalUp, totalDown int64

	for _, stat := range statsMap {
		totalConns += stat.ActiveConns
		totalUp += stat.BytesUp
		totalDown += stat.BytesDown
	}

	fmt.Println()
	PrintBanner("Traffic Statistics")
	fmt.Println()

	fmt.Printf("Active Sessions:     %d\n", len(statsMap))
	fmt.Printf("Total Connections:   %d\n", totalConns)
	fmt.Printf("Total Bytes Up:      %s\n", formatBytes(totalUp))
	fmt.Printf("Total Bytes Down:    %s\n", formatBytes(totalDown))
	fmt.Printf("Total Traffic:       %s\n", formatBytes(totalUp+totalDown))
}

// handleHTTPCommand handles HTTP log commands
func (c *CLI) handleHTTPCommand(args []string) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list", "ls":
		page := 1
		if len(args) > 1 {
			if p, err := strconv.Atoi(args[1]); err == nil {
				page = p
			}
		}
		c.listHTTPLogs(page)
	case "search", "find":
		page, values, err := parseHTTPLogSearchArgs(args[1:])
		if err != nil {
			fmt.Println("Usage: http search [keyword] [--local-port <port>] [--bastion <name>] [--url <url>] [page]")
			return
		}
		c.searchHTTPLogs(values, page)
	case "show", "get":
		if len(args) < 2 {
			fmt.Println("Usage: http show <id>")
			return
		}
		c.showHTTPLog(args[1])
	case "clear":
		c.clearHTTPLogs()
	default:
		fmt.Printf("Unknown http command: %s\n", args[0])
	}
}

// listHTTPLogs lists HTTP logs
func (c *CLI) listHTTPLogs(page int) {
	if !config.Settings.AuditEnabled {
		fmt.Println("HTTP audit is disabled. Enable with --audit flag.")
		return
	}

	pageSize := 20
	logs, total := service.GlobalServices.Audit.GetHTTPLogs(page, pageSize)

	if total == 0 {
		fmt.Println("No HTTP logs available.")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	fmt.Println()
	PrintBanner(fmt.Sprintf("HTTP Logs (Page %d/%d, Total: %d)", page, totalPages, total))
	fmt.Println()

	fmt.Printf("%-6s %-6s %-8s %-25s %-35s %-10s\n", "ID", "Code", "Method", "Host", "URL", "Time")
	fmt.Println(strings.Repeat("-", 100))

	for _, log := range logs {
		timestamp := log.Timestamp.Format("15:04:05")

		fmt.Printf("%-6d %-6d %-8s %-25s %-35s %-10s\n",
			log.ID,
			log.StatusCode,
			log.Method,
			truncate(log.Host, 25),
			truncate(log.URL, 35),
			timestamp,
		)
	}

	fmt.Printf("\nUse 'http show <id>' to view details\n")
}

func (c *CLI) searchHTTPLogs(values url.Values, page int) {
	if !config.Settings.AuditEnabled {
		fmt.Println("HTTP audit is disabled. Enable with --audit flag.")
		return
	}

	filter := core.HTTPLogFilter{
		Query:   values.Get("q"),
		URL:     values.Get("url"),
		Bastion: values.Get("bastion"),
	}
	if lp := strings.TrimSpace(values.Get("local_port")); lp != "" {
		if p, err := strconv.Atoi(lp); err == nil && p > 0 {
			filter.LocalPort = &p
		}
	}

	pageSize := 20
	logs, total := service.GlobalServices.Audit.QueryHTTPLogs(filter, page, pageSize)
	if total == 0 {
		fmt.Println("No HTTP logs available.")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	fmt.Println()
	PrintBanner(fmt.Sprintf("HTTP Logs Search (Page %d/%d, Total: %d)", page, totalPages, total))
	fmt.Println()

	fmt.Printf("%-6s %-6s %-8s %-25s %-35s %-10s\n", "ID", "Code", "Method", "Host", "URL", "Time")
	fmt.Println(strings.Repeat("-", 100))

	for _, log := range logs {
		timestamp := log.Timestamp.Format("15:04:05")
		fmt.Printf("%-6d %-6d %-8s %-25s %-35s %-10s\n",
			log.ID,
			log.StatusCode,
			log.Method,
			truncate(log.Host, 25),
			truncate(log.URL, 35),
			timestamp,
		)
	}

	fmt.Printf("\nUse 'http show <id>' to view details\n")
}

// showHTTPLog shows HTTP log details
func (c *CLI) showHTTPLog(idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", idStr)
		return
	}

	log := service.GlobalServices.Audit.GetHTTPLogByID(id)
	if log == nil {
		fmt.Printf("HTTP log not found: %d\n", id)
		return
	}

	fmt.Println()
	PrintBanner(fmt.Sprintf("HTTP Log #%d", log.ID))
	fmt.Println()

	fmt.Printf("Time:        %s\n", log.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Method:      %s\n", log.Method)
	fmt.Printf("Host:        %s\n", log.Host)
	fmt.Printf("URL:         %s\n", log.URL)
	fmt.Printf("Protocol:    %s\n", log.Protocol)
	fmt.Printf("Req Size:    %d bytes\n", log.ReqSize)
	fmt.Printf("Resp Size:   %d bytes\n", log.RespSize)

	if log.Request != "" {
		fmt.Printf("\nRequest:\n")
		fmt.Println(truncate(log.Request, 1000))
	}

	if log.Response != "" {
		fmt.Printf("\nResponse:\n")
		if log.IsGzipped && log.ResponseDecoded != "" {
			fmt.Println("(Decompressed from gzip)")
			fmt.Println(truncate(log.ResponseDecoded, 1000))
		} else {
			fmt.Println(truncate(log.Response, 1000))
		}
	}
}

// clearHTTPLogs clears HTTP logs
func (c *CLI) clearHTTPLogs() {
	confirm := c.readInput("Clear all HTTP logs? (yes/no)", "no")
	if strings.ToLower(confirm) != "yes" && strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return
	}

	service.GlobalServices.Audit.ClearHTTPLogs()
	fmt.Println("✓ HTTP logs cleared successfully!")
}

// clearScreen clears the console
func (c *CLI) clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// handleExit exits the CLI
func (c *CLI) handleExit() {
	fmt.Println("\nShutting down...")
	c.running = false
}

// readInput reads user input with an optional default
func (c *CLI) readInput(prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	c.scanner.Scan()
	input := strings.TrimSpace(c.scanner.Text())

	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

// readInputPassword reads a password (not echoed)
func (c *CLI) readInputPassword(prompt string) string {
	fmt.Printf("%s: ", prompt)
	// Note: For true password hiding, use golang.org/x/term
	// For simplicity, using regular input here
	c.scanner.Scan()
	return strings.TrimSpace(c.scanner.Text())
}

// formatBytes formats a byte count
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncate shortens a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// MakeHTTPRequest sends an HTTP request (for testing)
func (c *CLI) MakeHTTPRequest(method, url string, body io.Reader) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	fmt.Printf("\nSending %s request to %s...\n", method, url)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("\nResponse Status: %s\n", resp.Status)
	fmt.Printf("Headers:\n")
	for k, v := range resp.Header {
		fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if len(bodyBytes) > 0 {
		fmt.Printf("\nBody:\n%s\n", string(bodyBytes))
	}
}
