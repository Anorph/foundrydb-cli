package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the FoundryDB REST API
type Client struct {
	BaseURL    string
	Username   string
	Password   string
	OrgID      string
	HTTPClient *http.Client
}

// NewClient creates a new API client
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with Basic Auth
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send org scoping header when an org ID or slug is provided
	if c.OrgID != "" {
		req.Header.Set("X-Active-Org-ID", c.OrgID)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	return resp, nil
}

// decodeJSON reads response body and decodes JSON into target
func decodeJSON(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	if target != nil {
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// ListServices returns all managed services
func (c *Client) ListServices() (*ServiceListResponse, error) {
	resp, err := c.doRequest("GET", "/managed-services/", nil)
	if err != nil {
		return nil, err
	}

	var result ServiceListResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetService returns a single service by ID
func (c *Client) GetService(id string) (*Service, error) {
	resp, err := c.doRequest("GET", "/managed-services/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result Service
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateService creates a new managed service
func (c *Client) CreateService(req CreateServiceRequest) (*Service, error) {
	resp, err := c.doRequest("POST", "/managed-services/", req)
	if err != nil {
		return nil, err
	}

	var result Service
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteService deletes a managed service
func (c *Client) DeleteService(id string) error {
	resp, err := c.doRequest("DELETE", "/managed-services/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

// ListUsers returns database users for a service
func (c *Client) ListUsers(serviceID string) (*UserListResponse, error) {
	resp, err := c.doRequest("GET", "/managed-services/"+serviceID+"/database-users", nil)
	if err != nil {
		return nil, err
	}

	var result UserListResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RevealPassword reveals the password for a database user
func (c *Client) RevealPassword(serviceID, username string) (*RevealPasswordResponse, error) {
	path := fmt.Sprintf("/managed-services/%s/database-users/%s/reveal-password", serviceID, username)
	resp, err := c.doRequest("POST", path, nil)
	if err != nil {
		return nil, err
	}

	var result RevealPasswordResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBackups returns backups for a service
func (c *Client) ListBackups(serviceID string) (*BackupListResponse, error) {
	resp, err := c.doRequest("GET", "/managed-services/"+serviceID+"/backups", nil)
	if err != nil {
		return nil, err
	}

	var result BackupListResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TriggerBackup triggers a manual backup for a service
func (c *Client) TriggerBackup(serviceID string) (*TriggerBackupResponse, error) {
	resp, err := c.doRequest("POST", "/managed-services/"+serviceID+"/backups", nil)
	if err != nil {
		return nil, err
	}

	var result TriggerBackupResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMetrics returns current metrics for a service
func (c *Client) GetMetrics(serviceID string) (*Metrics, error) {
	resp, err := c.doRequest("GET", "/managed-services/"+serviceID+"/metrics/current", nil)
	if err != nil {
		return nil, err
	}

	var result Metrics
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RequestLogs requests log retrieval for a service
func (c *Client) RequestLogs(serviceID string, lines int) (*RequestLogsResponse, error) {
	path := fmt.Sprintf("/managed-services/%s/logs?lines=%d", serviceID, lines)
	resp, err := c.doRequest("POST", path, nil)
	if err != nil {
		return nil, err
	}

	var result RequestLogsResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListOrganizations returns all organizations the authenticated user belongs to
func (c *Client) ListOrganizations() (*OrganizationListResponse, error) {
	resp, err := c.doRequest("GET", "/organizations/", nil)
	if err != nil {
		return nil, err
	}

	var result OrganizationListResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PollLogs polls for log retrieval results
func (c *Client) PollLogs(serviceID, taskID string) (*LogsResponse, error) {
	path := fmt.Sprintf("/managed-services/%s/logs?task_id=%s", serviceID, taskID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result LogsResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
