package cli

import (
	"bastion/core"
	"bastion/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the HTTP client for talking to the Bastion server
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new HTTP client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest executes an HTTP request
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %v", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	return resp, nil
}

// handleResponse handles an HTTP response
func (c *Client) handleResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}
	}

	return nil
}

// HealthCheck pings the health endpoint
func (c *Client) HealthCheck() error {
	resp, err := c.doRequest("GET", "/api/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server unhealthy: HTTP %d", resp.StatusCode)
	}

	return nil
}

// Bastion management API

// ListBastions lists all bastions
func (c *Client) ListBastions() ([]models.Bastion, error) {
	resp, err := c.doRequest("GET", "/api/bastions", nil)
	if err != nil {
		return nil, err
	}

	var bastions []models.Bastion
	if err := c.handleResponse(resp, &bastions); err != nil {
		return nil, err
	}

	return bastions, nil
}

// GetBastion fetches a single bastion
func (c *Client) GetBastion(id uint) (*models.Bastion, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/bastions/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var bastion models.Bastion
	if err := c.handleResponse(resp, &bastion); err != nil {
		return nil, err
	}

	return &bastion, nil
}

// CreateBastion creates a bastion
func (c *Client) CreateBastion(req models.BastionCreate) (*models.Bastion, error) {
	resp, err := c.doRequest("POST", "/api/bastions", req)
	if err != nil {
		return nil, err
	}

	var bastion models.Bastion
	if err := c.handleResponse(resp, &bastion); err != nil {
		return nil, err
	}

	return &bastion, nil
}

// UpdateBastion updates a bastion
func (c *Client) UpdateBastion(id uint, req models.BastionCreate) (*models.Bastion, error) {
	resp, err := c.doRequest("PUT", fmt.Sprintf("/api/bastions/%d", id), req)
	if err != nil {
		return nil, err
	}

	var bastion models.Bastion
	if err := c.handleResponse(resp, &bastion); err != nil {
		return nil, err
	}

	return &bastion, nil
}

// DeleteBastion deletes a bastion
func (c *Client) DeleteBastion(id uint) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/api/bastions/%d", id), nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// Mapping management API

// ListMappings lists all mappings
func (c *Client) ListMappings() ([]models.MappingRead, error) {
	resp, err := c.doRequest("GET", "/api/mappings", nil)
	if err != nil {
		return nil, err
	}

	var mappings []models.MappingRead
	if err := c.handleResponse(resp, &mappings); err != nil {
		return nil, err
	}

	return mappings, nil
}

// GetMapping fetches a single mapping
func (c *Client) GetMapping(id string) (*models.Mapping, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/mappings/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var mapping models.Mapping
	if err := c.handleResponse(resp, &mapping); err != nil {
		return nil, err
	}

	return &mapping, nil
}

// CreateMapping creates a mapping
func (c *Client) CreateMapping(req models.MappingCreate) (*models.Mapping, error) {
	resp, err := c.doRequest("POST", "/api/mappings", req)
	if err != nil {
		return nil, err
	}

	var mapping models.Mapping
	if err := c.handleResponse(resp, &mapping); err != nil {
		return nil, err
	}

	return &mapping, nil
}

// DeleteMapping deletes a mapping
func (c *Client) DeleteMapping(id string) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/api/mappings/%s", id), nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// StartMapping starts a mapping
func (c *Client) StartMapping(id string) error {
	resp, err := c.doRequest("POST", fmt.Sprintf("/api/mappings/%s/start", id), nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// StopMapping stops a mapping
func (c *Client) StopMapping(id string) error {
	resp, err := c.doRequest("POST", fmt.Sprintf("/api/mappings/%s/stop", id), nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}

// Stats API

// GetStats fetches mapping statistics
func (c *Client) GetStats() (map[string]core.SessionStats, error) {
	resp, err := c.doRequest("GET", "/api/stats", nil)
	if err != nil {
		return nil, err
	}

	var stats map[string]core.SessionStats
	if err := c.handleResponse(resp, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

// HTTP Logs API

// HTTPLogsResponse HTTP log response structure
type HTTPLogsResponse struct {
	Data     []*core.HTTPLog `json:"data"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
}

// GetHTTPLogs fetches paginated HTTP logs
func (c *Client) GetHTTPLogs(page, pageSize int) ([]*core.HTTPLog, int, error) {
	path := fmt.Sprintf("/api/http-logs?page=%d&page_size=%d", page, pageSize)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, 0, err
	}

	var result HTTPLogsResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, 0, err
	}

	return result.Data, result.Total, nil
}

// GetHTTPLogByID fetches a single HTTP log entry
func (c *Client) GetHTTPLogByID(id int) (*core.HTTPLog, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/http-logs/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var log core.HTTPLog
	if err := c.handleResponse(resp, &log); err != nil {
		return nil, err
	}

	return &log, nil
}

// ClearHTTPLogs deletes all HTTP logs
func (c *Client) ClearHTTPLogs() error {
	resp, err := c.doRequest("DELETE", "/api/http-logs", nil)
	if err != nil {
		return err
	}

	return c.handleResponse(resp, nil)
}
