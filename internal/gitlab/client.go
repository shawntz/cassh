package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client represents a GitLab API client
type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

// SSHKey represents a GitLab SSH key
type SSHKey struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// NewClient creates a new GitLab API client
// baseURL should be the GitLab instance URL (e.g., "https://gitlab.com" or "https://gitlab.company.com")
// token is a personal access token with `api` scope
func NewClient(baseURL, token string) (*Client, error) {
	// Validate token is not empty
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	// Remove trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("PRIVATE-TOKEN", c.token)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// ListSSHKeys retrieves all SSH keys for the authenticated user
func (c *Client) ListSSHKeys() ([]SSHKey, error) {
	resp, err := c.doRequest("GET", "/api/v4/user/keys", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list SSH keys: unexpected status code %d", resp.StatusCode)
	}
	var keys []SSHKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return keys, nil
}

// GetSSHKeyByTitle finds an SSH key by its title
func (c *Client) GetSSHKeyByTitle(title string) (*SSHKey, error) {
	keys, err := c.ListSSHKeys()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if key.Title == title {
			return &key, nil
		}
	}

	return nil, nil // Not found
}

// CreateSSHKey adds a new SSH key to the authenticated user's account
func (c *Client) CreateSSHKey(title, publicKey string, expiresAt *time.Time) (*SSHKey, error) {
	payload := map[string]interface{}{
		"title": title,
		"key":   strings.TrimSpace(publicKey),
	}

	if expiresAt != nil {
		payload["expires_at"] = expiresAt.Format("2006-01-02")
	}

	resp, err := c.doRequest("POST", "/api/v4/user/keys", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to create SSH key: couldn't read response body: %w", readErr)
	}

	if resp.StatusCode != http.StatusCreated {
		// Check if key already exists (handle only relevant error status codes)
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusConflict {
			bodyStr := strings.ToLower(string(body))
			if strings.Contains(bodyStr, "has already been taken") || strings.Contains(bodyStr, "already exists") {
				// Try to find the existing key
				existingKey, err := c.GetSSHKeyByTitle(title)
				if err != nil {
					return nil, fmt.Errorf("key already exists but failed to retrieve it: %w", err)
				}
				if existingKey != nil {
					return existingKey, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to create SSH key: %s (status: %d)", string(body), resp.StatusCode)
	}

	var key SSHKey
	if err := json.Unmarshal(body, &key); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &key, nil
}

// DeleteSSHKey removes an SSH key by ID
func (c *Client) DeleteSSHKey(keyID int) error {
	endpoint := fmt.Sprintf("/api/v4/user/keys/%d", keyID)
	resp, err := c.doRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		// Key might already be deleted
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete SSH key: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// GetCurrentUser retrieves information about the authenticated user
func (c *Client) GetCurrentUser() (map[string]interface{}, error) {
	resp, err := c.doRequest("GET", "/api/v4/user", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s (status: %d)", string(body), resp.StatusCode)
	}

	var user map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return user, nil
}

// ValidateToken checks if the token is valid by attempting to get current user
func (c *Client) ValidateToken() error {
	_, err := c.GetCurrentUser()
	return err
}

// ExtractHostFromURL extracts the hostname from a GitLab URL
// Example: "https://gitlab.company.com" -> "gitlab.company.com"
func ExtractHostFromURL(gitlabURL string) string {
	parsed, err := url.Parse(gitlabURL)
	if err == nil && parsed.Hostname() != "" {
		// Use the parsed hostname, which excludes any port.
		return parsed.Hostname()
	}

	// If parsing fails or no hostname is found, try to extract manually.
	if err != nil {
		fmt.Printf("warning: failed to parse GitLab URL %q: %v\n", gitlabURL, err)
	}

	host := strings.TrimPrefix(gitlabURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "//")
	host = strings.TrimSpace(host)
	host = strings.TrimSuffix(host, "/")

	// Remove any path component.
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	// Remove any port component.
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	return host
}
