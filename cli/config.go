package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ServerConfig server configuration
type ServerConfig struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
	Token       string `yaml:"token,omitempty"`
}

// Config CLI configuration
type Config struct {
	DefaultServer string                  `yaml:"default_server"`
	Servers       map[string]ServerConfig `yaml:"servers"`
	configPath    string
}

// getConfigPath gets the configuration file path
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".bastion")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.yaml"), nil
}

// LoadConfig loads the configuration
func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	config := &Config{
		configPath: configPath,
		Servers:    make(map[string]ServerConfig),
	}

	// If config file doesn't exist, create default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config.DefaultServer = "local"
		config.Servers["local"] = ServerConfig{
			URL:         "http://localhost:8080",
			Description: "Local Bastion service",
		}
		if err := config.Save(); err != nil {
			return nil, err
		}
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	config.configPath = configPath
	return config, nil
}

// Save saves the configuration
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(c.configPath, data, 0600)
}

// AddServer adds a server
func (c *Config) AddServer(name, url, description string) error {
	if name == "" {
		return fmt.Errorf("server name cannot be empty")
	}
	if url == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	c.Servers[name] = ServerConfig{
		URL:         url,
		Description: description,
	}

	// If this is the first server, set it as default
	if c.DefaultServer == "" {
		c.DefaultServer = name
	}

	return c.Save()
}

// RemoveServer removes a server
func (c *Config) RemoveServer(name string) error {
	if _, exists := c.Servers[name]; !exists {
		return fmt.Errorf("server '%s' not found", name)
	}

	delete(c.Servers, name)

	// If the deleted server was the default, select another as default
	if c.DefaultServer == name {
		c.DefaultServer = ""
		for serverName := range c.Servers {
			c.DefaultServer = serverName
			break
		}
	}

	return c.Save()
}

// SetDefault sets the default server
func (c *Config) SetDefault(name string) error {
	if _, exists := c.Servers[name]; !exists {
		return fmt.Errorf("server '%s' not found", name)
	}

	c.DefaultServer = name
	return c.Save()
}

// GetServer gets server configuration
func (c *Config) GetServer(name string) (*ServerConfig, error) {
	if name == "" {
		name = c.DefaultServer
	}

	server, exists := c.Servers[name]
	if !exists {
		return nil, fmt.Errorf("server '%s' not found", name)
	}

	return &server, nil
}

// GetDefaultServer gets the default server configuration
func (c *Config) GetDefaultServer() (*ServerConfig, error) {
	return c.GetServer(c.DefaultServer)
}

// ListServers lists all servers
func (c *Config) ListServers() map[string]ServerConfig {
	return c.Servers
}
