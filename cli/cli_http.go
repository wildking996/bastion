package cli

import (
	"bastion/core"
	"bastion/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
)

// CLIHttp is the CLI for HTTP client mode
type CLIHttp struct {
	rl      *readline.Instance
	running bool
	client  *Client
}

// NewCLIHttp creates a new HTTP client CLI instance
func NewCLIHttp(serverURL string) (*CLIHttp, error) {
	// Create HTTP client
	client := NewClient(serverURL)

	// Test connectivity
	if err := client.HealthCheck(); err != nil {
		return nil, fmt.Errorf("cannot connect to server: %v", err)
	}

	// Create readline instance; ignore Ctrl+C
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create readline: %v", err)
	}

	return &CLIHttp{
		rl:      rl,
		running: true,
		client:  client,
	}, nil
}

// Start runs the CLI loop
func (c *CLIHttp) Start() {
	defer c.rl.Close()
	c.printWelcome()

	for c.running {
		line, err := c.rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C pressed
				fmt.Println("\n⚠ Ctrl+C detected. Please use 'exit' or 'quit' command to exit gracefully.")
				continue
			}
			// EOF or other error; exit
			break
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		c.handleCommand(input)
	}
}

// printWelcome prints initial banner
func (c *CLIHttp) printWelcome() {
	PrintBanner("Bastion - CLI Mode (HTTP Client)")
	fmt.Printf("\nConnected to: %s\n", c.client.baseURL)
	fmt.Println("Type 'help' for available commands")
}

// handleCommand routes user commands
func (c *CLIHttp) handleCommand(input string) {
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

// showHelp prints available commands
func (c *CLIHttp) showHelp() {
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
		{"http show <id>", "Show HTTP request/response details"},
		{"http clear", "Clear all HTTP logs"},
		{"", ""},
		{"SYSTEM:", ""},
		{"clear", "Clear screen"},
		{"exit, quit, q", "Exit the program"},
	}

	for _, cmd := range commands {
		if len(cmd) == 2 && cmd[0] != "" {
			fmt.Printf("  %-30s %s\n", cmd[0], cmd[1])
		} else {
			fmt.Println()
		}
	}
}

// handleBastionCommand handles bastion-related commands
func (c *CLIHttp) handleBastionCommand(args []string) {
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
func (c *CLIHttp) listBastions() {
	bastions, err := c.client.ListBastions()
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
func (c *CLIHttp) addBastion() {
	fmt.Println()
	PrintBanner("Add New Bastion (Interactive)")
	fmt.Println("\nTip: Don't worry about mistakes, you can review and modify at the confirmation step")
	fmt.Println("     Press Ctrl+C anytime to cancel")

	bastion := models.BastionCreate{}
	var authMethod string // 1=Password, 2=SSH Key

	// Step 1: Collect all inputs
	input, cancelled := c.readInputWithCancel("Name (optional, will auto-generate if empty)", "")
	if cancelled {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	bastion.Name = input

	for {
		input, cancelled := c.readInputWithCancel("Host (required)", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		if !validateHost(input) {
			fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address (e.g., 192.168.1.1) or domain name (e.g., example.com).")
			continue
		}
		bastion.Host = input
		break
	}

	for {
		input, cancelled := c.readInputWithCancel("Port (1-65535)", "22")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		port, _ := strconv.Atoi(input)
		if port == 0 {
			port = 22
		}
		if !validatePort(port) {
			fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
			continue
		}
		bastion.Port = port
		break
	}

	for {
		input, cancelled := c.readInputWithCancel("Username (required)", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		if !validateUsername(input) {
			fmt.Println("❌ Invalid username! Username cannot be empty, contain spaces or '@', and must be 1-32 characters.")
			continue
		}
		bastion.Username = input
		break
	}

	input, cancelled = c.readInputWithCancel("Auth method (1=Password, 2=SSH Key)", "1")
	if cancelled {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	authMethod = input

	if authMethod == "2" {
		input, cancelled = c.readInputWithCancel("SSH Key Path", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		bastion.PkeyPath = input

		input, cancelled = c.readInputWithCancel("Key Passphrase (optional)", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		bastion.PkeyPassphrase = input
	} else {
		input, cancelled = c.readInputPasswordWithCancel("Password")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		bastion.Password = input
	}

	// Step 2: Confirmation and modification loop
	for {
		fmt.Println()
		PrintBanner("Review Your Input")
		fmt.Printf("\n1. Name:       %s\n", bastion.Name)
		fmt.Printf("2. Host:       %s\n", bastion.Host)
		fmt.Printf("3. Port:       %d\n", bastion.Port)
		fmt.Printf("4. Username:   %s\n", bastion.Username)
		if authMethod == "2" {
			fmt.Printf("5. Auth:       SSH Key (%s)\n", bastion.PkeyPath)
			if bastion.PkeyPassphrase != "" {
				fmt.Printf("6. Passphrase: ****\n")
			}
		} else {
			fmt.Printf("5. Auth:       Password (****)\n")
		}

		fmt.Println("\nOptions:")
		fmt.Println("  - Press Enter to confirm and create")
		fmt.Println("  - Enter field number (1-6) to modify")
		fmt.Println("  - Press Ctrl+C to abort")

		choice, cancelled := c.readInputWithCancel("Your choice", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}

		choice = strings.TrimSpace(strings.ToLower(choice))

		if choice == "" {
			// Confirm and create
			bastion.Normalize()
			createdBastion, err := c.client.CreateBastion(bastion)
			if err != nil {
				fmt.Printf("\n❌ Error creating bastion: %v\n", err)
				return
			}
			fmt.Printf("\n✓ Bastion created successfully! ID: %d\n", createdBastion.ID)
			return
		}

		// Modify specific field
		switch choice {
		case "1":
			input, cancelled := c.readInputWithCancel("Name", bastion.Name)
			if !cancelled {
				bastion.Name = input
			}
		case "2":
			for {
				input, cancelled := c.readInputWithCancel("Host", bastion.Host)
				if cancelled {
					break
				}
				if !validateHost(input) {
					fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address or domain name.")
					continue
				}
				bastion.Host = input
				break
			}
		case "3":
			for {
				input, cancelled := c.readInputWithCancel("Port (1-65535)", strconv.Itoa(bastion.Port))
				if cancelled {
					break
				}
				port, _ := strconv.Atoi(input)
				if !validatePort(port) {
					fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
					continue
				}
				bastion.Port = port
				break
			}
		case "4":
			for {
				input, cancelled := c.readInputWithCancel("Username", bastion.Username)
				if cancelled {
					break
				}
				if !validateUsername(input) {
					fmt.Println("❌ Invalid username!")
					continue
				}
				bastion.Username = input
				break
			}
		case "5":
			newAuth, cancelled := c.readInputWithCancel("Auth method (1=Password, 2=SSH Key)", authMethod)
			if !cancelled && newAuth != authMethod {
				authMethod = newAuth
				// Clear previous auth data
				bastion.Password = ""
				bastion.PkeyPath = ""
				bastion.PkeyPassphrase = ""

				if authMethod == "2" {
					input, cancelled := c.readInputWithCancel("SSH Key Path", "")
					if !cancelled {
						bastion.PkeyPath = input
					}
					input, cancelled = c.readInputWithCancel("Key Passphrase (optional)", "")
					if !cancelled {
						bastion.PkeyPassphrase = input
					}
				} else {
					input, cancelled := c.readInputPasswordWithCancel("Password")
					if !cancelled {
						bastion.Password = input
					}
				}
			}
		case "6":
			if authMethod == "2" {
				input, cancelled := c.readInputWithCancel("Key Passphrase", "")
				if !cancelled {
					bastion.PkeyPassphrase = input
				}
			}
		default:
			fmt.Println("❌ Invalid choice. Please try again.")
		}
	}
}

// deleteBastion removes a bastion
func (c *CLIHttp) deleteBastion(idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", idStr)
		return
	}

	bastion, err := c.client.GetBastion(uint(id))
	if err != nil {
		fmt.Printf("Bastion not found: %d\n", id)
		return
	}

	confirm := c.readInput(fmt.Sprintf("Delete bastion '%s'? (yes/no)", bastion.Name), "no")
	if strings.ToLower(confirm) != "yes" && strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return
	}

	if err := c.client.DeleteBastion(uint(id)); err != nil {
		fmt.Printf("Error deleting bastion: %v\n", err)
		return
	}

	fmt.Println("✓ Bastion deleted successfully!")
}

// showBastion prints bastion details
func (c *CLIHttp) showBastion(idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", idStr)
		return
	}

	bastion, err := c.client.GetBastion(uint(id))
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
func (c *CLIHttp) handleMappingCommand(args []string) {
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
func (c *CLIHttp) listMappings() {
	mappings, err := c.client.ListMappings()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(mappings) == 0 {
		fmt.Println("No mappings configured.")
		return
	}

	fmt.Println()
	PrintBanner(fmt.Sprintf("Total Mappings: %d", len(mappings)))
	fmt.Println()

	fmt.Printf("%-20s %-18s %-18s %-8s %-25s %-10s\n", "ID", "Local", "Remote", "Type", "Chain", "Status")
	fmt.Println(strings.Repeat("-", 110))

	for _, m := range mappings {
		status := "Stopped"
		if m.Running {
			status = "Running"
		}

		chain := strings.Join(m.Chain, " → ")

		fmt.Printf("%-20s %-18s %-18s %-8s %-25s %-10s\n",
			truncate(m.ID, 20),
			fmt.Sprintf("%s:%d", m.LocalHost, m.LocalPort),
			fmt.Sprintf("%s:%d", m.RemoteHost, m.RemotePort),
			m.Type,
			truncate(chain, 25),
			status,
		)
	}
}

// addMapping adds a mapping interactively
func (c *CLIHttp) addMapping() {
	fmt.Println()
	PrintBanner("Add New Mapping (Interactive)")
	fmt.Println("\nTip: Don't worry about mistakes, you can review and modify at the confirmation step")
	fmt.Println("     Press Ctrl+C anytime to cancel")

	mapping := models.MappingCreate{}
	var bastions []models.Bastion

	// Step 1: Collect all inputs
	for {
		input, cancelled := c.readInputWithCancel("Local Host", "127.0.0.1")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		if input == "" {
			input = "127.0.0.1"
		}
		if !validateHost(input) {
			fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address or domain name.")
			continue
		}
		mapping.LocalHost = input
		break
	}

	for {
		input, cancelled := c.readInputWithCancel("Local Port (1-65535, required)", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		if input == "" {
			fmt.Println("❌ Local Port is required!")
			continue
		}
		localPort, _ := strconv.Atoi(input)
		if !validatePort(localPort) {
			fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
			continue
		}
		mapping.LocalPort = localPort
		break
	}

	input, cancelled := c.readInputWithCancel("Type (1=TCP, 2=SOCKS5, 3=HTTP)", "1")
	if cancelled {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	switch strings.TrimSpace(input) {
	case "2":
		mapping.Type = "socks5"
	case "3":
		mapping.Type = "http"
	default:
		mapping.Type = "tcp"
	}

	if mapping.Type == "tcp" {
		for {
			input, cancelled := c.readInputWithCancel("Remote Host (required)", "")
			if cancelled {
				fmt.Println("\n❌ Operation cancelled")
				return
			}
			if !validateHost(input) {
				fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address or domain name.")
				continue
			}
			mapping.RemoteHost = input
			break
		}

		for {
			input, cancelled := c.readInputWithCancel("Remote Port (1-65535, required)", "")
			if cancelled {
				fmt.Println("\n❌ Operation cancelled")
				return
			}
			if input == "" {
				fmt.Println("❌ Remote Port is required!")
				continue
			}
			remotePort, _ := strconv.Atoi(input)
			if !validatePort(remotePort) {
				fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
				continue
			}
			mapping.RemotePort = remotePort
			break
		}
	}

	// Bastion chain
	fmt.Println("\nAvailable Bastions:")
	bastions, _ = c.client.ListBastions()
	for i, b := range bastions {
		fmt.Printf("  %d. %s (%s:%d)\n", i+1, b.Name, b.Host, b.Port)
	}

	for {
		input, cancelled := c.readInputWithCancel("Bastion chain (comma-separated names or numbers, optional)", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}
		validChain, ok := validateBastionChain(input, bastions)
		if !ok {
			fmt.Println("❌ Invalid bastion chain! Please use valid bastion numbers (1-" + strconv.Itoa(len(bastions)) + ") or names from the list above.")
			continue
		}
		mapping.Chain = validChain
		break
	}

	input, cancelled = c.readInputWithCancel("Mapping ID (optional, will auto-generate)", "")
	if cancelled {
		fmt.Println("\n❌ Operation cancelled")
		return
	}
	if input == "" {
		input = fmt.Sprintf("%s:%d", mapping.LocalHost, mapping.LocalPort)
	}
	mapping.ID = input

	// Step 2: Confirmation and modification loop
	for {
		fmt.Println()
		PrintBanner("Review Your Input")
		fmt.Printf("\n1. ID:          %s\n", mapping.ID)
		fmt.Printf("2. Local:       %s:%d\n", mapping.LocalHost, mapping.LocalPort)
		fmt.Printf("3. Type:        %s\n", mapping.Type)
		if mapping.Type == "tcp" {
			fmt.Printf("4. Remote:      %s:%d\n", mapping.RemoteHost, mapping.RemotePort)
		}
		if len(mapping.Chain) > 0 {
			fmt.Printf("5. Chain:       %s\n", strings.Join(mapping.Chain, " → "))
		} else {
			fmt.Printf("5. Chain:       (none)\n")
		}

		fmt.Println("\nOptions:")
		fmt.Println("  - Press Enter to confirm and create")
		fmt.Println("  - Enter field number (1-5) to modify")
		fmt.Println("  - Press Ctrl+C to abort")

		choice, cancelled := c.readInputWithCancel("Your choice", "")
		if cancelled {
			fmt.Println("\n❌ Operation cancelled")
			return
		}

		choice = strings.TrimSpace(strings.ToLower(choice))

		if choice == "" {
			// Confirm and create
			mapping.Normalize()
			createdMapping, err := c.client.CreateMapping(mapping)
			if err != nil {
				fmt.Printf("\n❌ Error creating mapping: %v\n", err)
				return
			}
			fmt.Printf("\n✓ Mapping created successfully! ID: %s\n", createdMapping.ID)
			return
		}

		// Modify specific field
		switch choice {
		case "1":
			input, cancelled := c.readInputWithCancel("Mapping ID", mapping.ID)
			if !cancelled && input != "" {
				mapping.ID = input
			}
		case "2":
			for {
				host, cancelled := c.readInputWithCancel("Local Host", mapping.LocalHost)
				if cancelled {
					break
				}
				if !validateHost(host) {
					fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address or domain name.")
					continue
				}
				mapping.LocalHost = host
				break
			}
			for {
				portStr, cancelled := c.readInputWithCancel("Local Port (1-65535)", strconv.Itoa(mapping.LocalPort))
				if cancelled {
					break
				}
				port, _ := strconv.Atoi(portStr)
				if !validatePort(port) {
					fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
					continue
				}
				mapping.LocalPort = port
				break
			}
		case "3":
			newType, cancelled := c.readInputWithCancel("Type (1=TCP, 2=SOCKS5)", "")
			if !cancelled {
				oldType := mapping.Type
				if newType == "2" {
					mapping.Type = "socks5"
				} else {
					mapping.Type = "tcp"
				}

				// If changed from SOCKS5 to TCP, need to collect remote host/port
				if oldType == "socks5" && mapping.Type == "tcp" {
					for {
						host, cancelled := c.readInputWithCancel("Remote Host", "")
						if cancelled {
							break
						}
						if !validateHost(host) {
							fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address or domain name.")
							continue
						}
						mapping.RemoteHost = host
						break
					}
					for {
						portStr, cancelled := c.readInputWithCancel("Remote Port (1-65535)", "")
						if cancelled {
							break
						}
						port, _ := strconv.Atoi(portStr)
						if !validatePort(port) {
							fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
							continue
						}
						mapping.RemotePort = port
						break
					}
				}
			}
		case "4":
			if mapping.Type == "tcp" {
				for {
					host, cancelled := c.readInputWithCancel("Remote Host", mapping.RemoteHost)
					if cancelled {
						break
					}
					if !validateHost(host) {
						fmt.Println("❌ Invalid host format! Please enter a valid IPv4 address or domain name.")
						continue
					}
					mapping.RemoteHost = host
					break
				}
				for {
					portStr, cancelled := c.readInputWithCancel("Remote Port (1-65535)", strconv.Itoa(mapping.RemotePort))
					if cancelled {
						break
					}
					port, _ := strconv.Atoi(portStr)
					if !validatePort(port) {
						fmt.Println("❌ Invalid port! Port must be between 1 and 65535.")
						continue
					}
					mapping.RemotePort = port
					break
				}
			}
		case "5":
			fmt.Println("\nAvailable Bastions:")
			bastions, _ = c.client.ListBastions()
			for i, b := range bastions {
				fmt.Printf("  %d. %s (%s:%d)\n", i+1, b.Name, b.Host, b.Port)
			}
			currentChain := strings.Join(mapping.Chain, ",")
			for {
				input, cancelled := c.readInputWithCancel("Bastion chain (comma-separated, empty to clear)", currentChain)
				if cancelled {
					break
				}
				validChain, ok := validateBastionChain(input, bastions)
				if !ok {
					fmt.Println("❌ Invalid bastion chain! Please use valid bastion numbers (1-" + strconv.Itoa(len(bastions)) + ") or names from the list above.")
					continue
				}
				mapping.Chain = validChain
				break
			}
		default:
			fmt.Println("❌ Invalid choice. Please try again.")
		}
	}
}

// deleteMapping deletes a mapping
func (c *CLIHttp) deleteMapping(id string) {
	if err := c.client.DeleteMapping(id); err != nil {
		fmt.Printf("Error deleting mapping: %v\n", err)
		return
	}

	fmt.Println("✓ Mapping deleted successfully!")
}

// showMapping displays mapping details
func (c *CLIHttp) showMapping(id string) {
	mapping, err := c.client.GetMapping(id)
	if err != nil {
		fmt.Printf("Mapping not found: %s\n", id)
		return
	}

	// Get running status from list
	mappings, _ := c.client.ListMappings()
	running := false
	for _, m := range mappings {
		if m.ID == id {
			running = m.Running
			break
		}
	}

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
	} else {
		fmt.Printf("Status:      Stopped\n")
	}
}

// handleStartCommand starts a mapping
func (c *CLIHttp) handleStartCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: start <mapping_id>")
		return
	}

	id := args[0]

	fmt.Printf("Starting mapping %s...\n", id)
	if err := c.client.StartMapping(id); err != nil {
		fmt.Printf("Error starting mapping: %v\n", err)
		return
	}

	fmt.Println("✓ Mapping started successfully!")
}

// handleStopCommand stops a mapping
func (c *CLIHttp) handleStopCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: stop <mapping_id>")
		return
	}

	id := args[0]

	fmt.Printf("Stopping mapping %s...\n", id)
	if err := c.client.StopMapping(id); err != nil {
		fmt.Printf("Error stopping mapping: %v\n", err)
		return
	}
	fmt.Println("✓ Mapping stopped successfully!")
}

// handleStatusCommand shows all session states
func (c *CLIHttp) handleStatusCommand() {
	stats, err := c.client.GetStats()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println()
	PrintBanner(fmt.Sprintf("Active Sessions: %d", len(stats)))
	fmt.Println()

	if len(stats) == 0 {
		fmt.Println("No active sessions.")
		return
	}

	fmt.Printf("%-20s %-15s %-15s %-15s\n", "Mapping ID", "Connections", "Bytes Up", "Bytes Down")
	fmt.Println(strings.Repeat("-", 70))

	for id, stat := range stats {
		fmt.Printf("%-20s %-15d %-15s %-15s\n",
			truncate(id, 20),
			stat.ActiveConns,
			formatBytes(stat.BytesUp),
			formatBytes(stat.BytesDown),
		)
	}
}

// handleStatsCommand shows traffic statistics
func (c *CLIHttp) handleStatsCommand() {
	statsMap, err := c.client.GetStats()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

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

// handleHTTPCommand routes HTTP log commands
func (c *CLIHttp) handleHTTPCommand(args []string) {
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
func (c *CLIHttp) listHTTPLogs(page int) {
	pageSize := 20
	logs, total, err := c.client.GetHTTPLogs(page, pageSize)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if total == 0 {
		fmt.Println("No HTTP logs available.")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	fmt.Println()
	PrintBanner(fmt.Sprintf("HTTP Logs (Page %d/%d, Total: %d)", page, totalPages, total))
	fmt.Println()

	fmt.Printf("%-6s %-8s %-25s %-35s %-10s\n", "ID", "Method", "Host", "URL", "Time")
	fmt.Println(strings.Repeat("-", 90))

	for _, log := range logs {
		timestamp := log.Timestamp.Format("15:04:05")

		fmt.Printf("%-6d %-8s %-25s %-35s %-10s\n",
			log.ID,
			log.Method,
			truncate(log.Host, 25),
			truncate(log.URL, 35),
			timestamp,
		)
	}

	fmt.Printf("\nUse 'http show <id>' to view details\n")
}

// showHTTPLog shows HTTP log details
func (c *CLIHttp) showHTTPLog(idStr string) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Printf("Invalid ID: %s\n", idStr)
		return
	}

	log, err := c.client.GetHTTPLogByID(id)
	if err != nil {
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
func (c *CLIHttp) clearHTTPLogs() {
	confirm := c.readInput("Clear all HTTP logs? (yes/no)", "no")
	if strings.ToLower(confirm) != "yes" && strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return
	}

	if err := c.client.ClearHTTPLogs(); err != nil {
		fmt.Printf("Error clearing logs: %v\n", err)
		return
	}
	fmt.Println("✓ HTTP logs cleared successfully!")
}

// clearScreen clears the console
func (c *CLIHttp) clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// handleExit exits the CLI
func (c *CLIHttp) handleExit() {
	// Fetch running sessions
	stats, err := c.client.GetStats()
	if err != nil {
		fmt.Printf("Error getting sessions: %v\n", err)
		c.running = false
		return
	}

	if len(stats) == 0 {
		fmt.Println("\nGoodbye!")
		c.running = false
		return
	}

	// Active sessions exist; ask the user what to do
	fmt.Printf("\n⚠ You have %d active session(s).\n", len(stats))
	fmt.Println("\nOptions:")
	fmt.Println("  1. Exit directly (keep sessions running)")
	fmt.Println("  2. Stop all sessions and exit")
	fmt.Println("  3. Stop sessions individually")
	fmt.Println("  0. Cancel (return to CLI)")

	choice := c.readInput("\nYour choice", "1")

	switch choice {
	case "0":
		fmt.Println("Exit cancelled.")
		return
	case "1":
		fmt.Println("\nGoodbye! (Sessions still running)")
		c.running = false
	case "2":
		c.stopAllSessions(stats)
		fmt.Println("\nGoodbye!")
		c.running = false
	case "3":
		c.stopSessionsInteractively(stats)
		fmt.Println("\nGoodbye!")
		c.running = false
	default:
		fmt.Println("Invalid choice. Exit cancelled.")
	}
}

// stopAllSessions stops every session
func (c *CLIHttp) stopAllSessions(stats map[string]core.SessionStats) {
	fmt.Println("\nStopping all sessions...")
	for id := range stats {
		if err := c.client.StopMapping(id); err != nil {
			fmt.Printf("❌ Failed to stop %s: %v\n", id, err)
		} else {
			fmt.Printf("✓ Stopped %s\n", id)
		}
	}
}

// stopSessionsInteractively lets the user stop sessions interactively
func (c *CLIHttp) stopSessionsInteractively(stats map[string]core.SessionStats) {
	// Convert to slice and sort by port
	type sessionInfo struct {
		id   string
		port int
	}

	var sessions []sessionInfo
	for id := range stats {
		port := extractPort(id)
		sessions = append(sessions, sessionInfo{id: id, port: port})
	}

	// Sort by port ascending
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].port > sessions[j].port {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	page := 0
	pageSize := 4

	for {
		// Calculate current page
		start := page * pageSize
		end := start + pageSize
		if end > len(sessions) {
			end = len(sessions)
		}

		if start >= len(sessions) {
			fmt.Println("\nAll sessions reviewed.")
			break
		}

		// Display current page
		fmt.Println()
		PrintBanner(fmt.Sprintf("Active Sessions (Page %d/%d)", page+1, (len(sessions)+pageSize-1)/pageSize))
		fmt.Println()

		for i := start; i < end; i++ {
			fmt.Printf("  %d. %s\n", i-start+1, sessions[i].id)
		}
		fmt.Printf("  %d. Finish and exit\n", end-start+1)

		// Read user selection
		input := c.readInput("\nSelect session to stop (number or port)", "")
		if input == "" {
			continue
		}

		// Parse input
		if num, err := strconv.Atoi(input); err == nil {
			// Input is an index
			if num == end-start+1 {
				// Done selecting
				return
			}
			if num >= 1 && num <= end-start {
				idx := start + num - 1
				if err := c.client.StopMapping(sessions[idx].id); err != nil {
					fmt.Printf("❌ Failed to stop %s: %v\n", sessions[idx].id, err)
				} else {
					fmt.Printf("✓ Stopped %s\n", sessions[idx].id)
					// Remove from list
					sessions = append(sessions[:idx], sessions[idx+1:]...)
					if idx >= start+pageSize {
						page++
					}
				}
			} else {
				fmt.Println("Invalid selection. Please try again.")
			}
		} else {
			// Input might be a port
			found := false
			for i, s := range sessions {
				if strings.Contains(s.id, input) {
					if err := c.client.StopMapping(s.id); err != nil {
						fmt.Printf("❌ Failed to stop %s: %v\n", s.id, err)
					} else {
						fmt.Printf("✓ Stopped %s\n", s.id)
						sessions = append(sessions[:i], sessions[i+1:]...)
						found = true
					}
					break
				}
			}
			if !found {
				fmt.Println("Session not found. Please try again.")
			}
		}
	}
}

// extractPort pulls the port from a mapping ID
func extractPort(id string) int {
	// ID format is typically "127.0.0.1:8080"
	parts := strings.Split(id, ":")
	if len(parts) == 2 {
		if port, err := strconv.Atoi(parts[1]); err == nil {
			return port
		}
	}
	return 0
}

// validateBastionChain validates bastion chain input
func validateBastionChain(input string, bastions []models.Bastion) ([]string, bool) {
	if input == "" {
		return []string{}, true // Empty chain is allowed
	}

	chainParts := strings.Split(input, ",")
	validChain := []string{}

	for _, part := range chainParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		found := false
		// Try parsing as an index
		if idx, err := strconv.Atoi(part); err == nil {
			if idx > 0 && idx <= len(bastions) {
				validChain = append(validChain, bastions[idx-1].Name)
				found = true
			}
		} else {
			// Otherwise try name lookup
			for _, b := range bastions {
				if b.Name == part {
					validChain = append(validChain, part)
					found = true
					break
				}
			}
		}

		if !found {
			return nil, false
		}
	}

	return validChain, true
}

// readInput reads user input with an optional default
func (c *CLIHttp) readInput(prompt, defaultValue string) string {
	if defaultValue != "" {
		c.rl.SetPrompt(fmt.Sprintf("%s [%s]: ", prompt, defaultValue))
	} else {
		c.rl.SetPrompt(fmt.Sprintf("%s: ", prompt))
	}

	line, err := c.rl.Readline()
	c.rl.SetPrompt("> ") // Restore default prompt

	if err != nil {
		return defaultValue
	}

	input := strings.TrimSpace(line)
	if input == "" && defaultValue != "" {
		return defaultValue
	}
	return input
}

// readInputWithCancel reads input and supports cancellation
func (c *CLIHttp) readInputWithCancel(prompt, defaultValue string) (string, bool) {
	if defaultValue != "" {
		c.rl.SetPrompt(fmt.Sprintf("%s [%s]: ", prompt, defaultValue))
	} else {
		c.rl.SetPrompt(fmt.Sprintf("%s: ", prompt))
	}

	line, err := c.rl.Readline()
	c.rl.SetPrompt("> ") // Restore default prompt

	if err != nil {
		if err == readline.ErrInterrupt {
			// Ctrl+C pressed, cancel
			return "", true
		}
		// Other errors: use default
		return defaultValue, false
	}

	input := strings.TrimSpace(line)
	if input == "" && defaultValue != "" {
		return defaultValue, false
	}
	return input, false
}

// readInputPasswordWithCancel reads a password without echo and supports cancel
func (c *CLIHttp) readInputPasswordWithCancel(prompt string) (string, bool) {
	c.rl.SetPrompt(fmt.Sprintf("%s: ", prompt))
	line, err := c.rl.ReadPassword("")
	c.rl.SetPrompt("> ") // Restore default prompt

	if err != nil {
		if err == readline.ErrInterrupt {
			// Ctrl+C pressed, cancel
			return "", true
		}
		return "", false
	}
	return string(line), false
}

// validatePort ensures the port is within a valid range
func validatePort(port int) bool {
	return port > 0 && port <= 65535
}

// validateHost checks host or IP format
func validateHost(host string) bool {
	if host == "" {
		return false
	}

	// Must not contain spaces and must be reasonable length
	if strings.Contains(host, " ") || len(host) > 255 {
		return false
	}

	// Check for valid IPv4
	if isValidIPv4(host) {
		return true
	}

	// Check for valid domain format
	if isValidDomain(host) {
		return true
	}

	return false
}

// isValidIPv4 validates IPv4 address format
func isValidIPv4(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}

		// Ensure all digits
		num, err := strconv.Atoi(part)
		if err != nil {
			return false
		}

		// Range 0-255
		if num < 0 || num > 255 {
			return false
		}

		// No leading zeros except for "0"
		if len(part) > 1 && part[0] == '0' {
			return false
		}
	}

	return true
}

// isValidDomain validates domain format
func isValidDomain(domain string) bool {
	// Domain length check
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// Cannot start or end with a dot
	if domain[0] == '.' || domain[len(domain)-1] == '.' {
		return false
	}

	// Split domain labels
	labels := strings.Split(domain, ".")
	if len(labels) < 1 {
		return false
	}

	// Must contain at least one dot (two labels) or be localhost
	if len(labels) == 1 && domain != "localhost" {
		return false
	}

	for _, label := range labels {
		// Each label length 1-63
		if len(label) == 0 || len(label) > 63 {
			return false
		}

		// Label cannot start or end with hyphen
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		// Validate characters (letters, digits, hyphen)
		hasLetter := false
		for _, ch := range label {
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				hasLetter = true
			}
			isAllowed := (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') ||
				ch == '-'
			if !isAllowed {
				return false
			}
		}

		// Each label must include at least one letter (numeric-only not allowed unless localhost)
		if !hasLetter && domain != "localhost" {
			return false
		}
	}

	return true
}

// validateUsername checks username rules
func validateUsername(username string) bool {
	if username == "" {
		return false
	}
	// Username cannot contain spaces or special characters
	if strings.Contains(username, " ") || strings.Contains(username, "@") {
		return false
	}
	return len(username) > 0 && len(username) <= 32
}
