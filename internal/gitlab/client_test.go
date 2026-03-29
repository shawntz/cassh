package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		token    string
		wantURL  string
	}{
		{
			name:    "URL with trailing slash",
			baseURL: "https://gitlab.com/",
			token:   "test-token",
			wantURL: "https://gitlab.com",
		},
		{
			name:    "URL without trailing slash",
			baseURL: "https://gitlab.company.com",
			token:   "test-token",
			wantURL: "https://gitlab.company.com",
		},
		{
			name:    "URL with multiple trailing slashes",
			baseURL: "https://gitlab.example.com///",
			token:   "test-token",
			wantURL: "https://gitlab.example.com//",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.token)
			if client == nil {
				t.Fatal("NewClient() returned nil")
			}
			if client.baseURL != tt.wantURL {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.wantURL)
			}
			if client.token != tt.token {
				t.Errorf("token = %q, want %q", client.token, tt.token)
			}
			if client.client == nil {
				t.Error("HTTP client is nil")
			}
		})
	}
}

func TestDoRequest_Authentication(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		wantAuthHeader string
	}{
		{
			name:           "Valid token",
			token:          "test-token-123",
			wantAuthHeader: "test-token-123",
		},
		{
			name:           "Empty token",
			token:          "",
			wantAuthHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("PRIVATE-TOKEN")
				if authHeader != tt.wantAuthHeader {
					t.Errorf("PRIVATE-TOKEN header = %q, want %q", authHeader, tt.wantAuthHeader)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(server.URL, tt.token)
			_, err := client.doRequest("GET", "/test", nil)
			if err != nil {
				t.Errorf("doRequest() error = %v", err)
			}
		})
	}
}

func TestDoRequest_HTTPStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		method     string
	}{
		{"200 OK", http.StatusOK, "GET"},
		{"201 Created", http.StatusCreated, "POST"},
		{"204 No Content", http.StatusNoContent, "DELETE"},
		{"400 Bad Request", http.StatusBadRequest, "POST"},
		{"401 Unauthorized", http.StatusUnauthorized, "GET"},
		{"403 Forbidden", http.StatusForbidden, "GET"},
		{"404 Not Found", http.StatusNotFound, "GET"},
		{"500 Internal Server Error", http.StatusInternalServerError, "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Request method = %q, want %q", r.Method, tt.method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			resp, err := client.doRequest(tt.method, "/test", nil)
			if err != nil {
				t.Errorf("doRequest() error = %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestListSSHKeys(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
		wantCount  int
	}{
		{
			name:       "Success with keys",
			statusCode: http.StatusOK,
			response: []SSHKey{
				{ID: 1, Title: "key1", Key: "ssh-ed25519 AAAA... user@host"},
				{ID: 2, Title: "key2", Key: "ssh-ed25519 BBBB... user@host"},
			},
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:       "Success with empty list",
			statusCode: http.StatusOK,
			response:   []SSHKey{},
			wantErr:    false,
			wantCount:  0,
		},
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			response:   map[string]string{"message": "Unauthorized"},
			wantErr:    true,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"error": "Internal error"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v4/user/keys" {
					t.Errorf("Request path = %q, want %q", r.URL.Path, "/api/v4/user/keys")
				}
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			keys, err := client.ListSSHKeys()

			if (err != nil) != tt.wantErr {
				t.Errorf("ListSSHKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(keys) != tt.wantCount {
				t.Errorf("ListSSHKeys() returned %d keys, want %d", len(keys), tt.wantCount)
			}
		})
	}
}

func TestGetSSHKeyByTitle(t *testing.T) {
	keys := []SSHKey{
		{ID: 1, Title: "work-laptop", Key: "ssh-ed25519 AAAA..."},
		{ID: 2, Title: "home-desktop", Key: "ssh-ed25519 BBBB..."},
		{ID: 3, Title: "ci-server", Key: "ssh-ed25519 CCCC..."},
	}

	tests := []struct {
		name      string
		title     string
		wantFound bool
		wantID    int
	}{
		{
			name:      "Find existing key",
			title:     "work-laptop",
			wantFound: true,
			wantID:    1,
		},
		{
			name:      "Find another existing key",
			title:     "ci-server",
			wantFound: true,
			wantID:    3,
		},
		{
			name:      "Key not found",
			title:     "nonexistent",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(keys)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			key, err := client.GetSSHKeyByTitle(tt.title)

			if err != nil {
				t.Errorf("GetSSHKeyByTitle() error = %v", err)
				return
			}

			if tt.wantFound {
				if key == nil {
					t.Error("GetSSHKeyByTitle() returned nil, want key")
					return
				}
				if key.ID != tt.wantID {
					t.Errorf("Key ID = %d, want %d", key.ID, tt.wantID)
				}
			} else {
				if key != nil {
					t.Errorf("GetSSHKeyByTitle() returned key, want nil")
				}
			}
		})
	}
}

func TestCreateSSHKey(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		publicKey  string
		expiresAt  *time.Time
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "Success without expiry",
			title:      "test-key",
			publicKey:  "ssh-ed25519 AAAA... user@host",
			expiresAt:  nil,
			statusCode: http.StatusCreated,
			response:   SSHKey{ID: 1, Title: "test-key", Key: "ssh-ed25519 AAAA... user@host"},
			wantErr:    false,
		},
		{
			name:       "Success with expiry",
			title:      "test-key-expiry",
			publicKey:  "ssh-ed25519 BBBB... user@host",
			expiresAt:  func() *time.Time { t := time.Now().Add(30 * 24 * time.Hour); return &t }(),
			statusCode: http.StatusCreated,
			response:   SSHKey{ID: 2, Title: "test-key-expiry", Key: "ssh-ed25519 BBBB... user@host"},
			wantErr:    false,
		},
		{
			name:       "400 Bad Request",
			title:      "invalid-key",
			publicKey:  "not-a-valid-key",
			expiresAt:  nil,
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"message": "Invalid key"},
			wantErr:    true,
		},
		{
			name:       "401 Unauthorized",
			title:      "test-key",
			publicKey:  "ssh-ed25519 CCCC... user@host",
			expiresAt:  nil,
			statusCode: http.StatusUnauthorized,
			response:   map[string]string{"message": "Unauthorized"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v4/user/keys" {
					t.Errorf("Request path = %q, want %q", r.URL.Path, "/api/v4/user/keys")
				}
				if r.Method != "POST" {
					t.Errorf("Request method = %q, want POST", r.Method)
				}

				var payload map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				if payload["title"] != tt.title {
					t.Errorf("Request title = %q, want %q", payload["title"], tt.title)
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			key, err := client.CreateSSHKey(tt.title, tt.publicKey, tt.expiresAt)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSSHKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && key == nil {
				t.Error("CreateSSHKey() returned nil key")
			}
		})
	}
}

func TestCreateSSHKey_DuplicateHandling(t *testing.T) {
	existingKey := SSHKey{ID: 1, Title: "duplicate-key", Key: "ssh-ed25519 AAAA..."}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v4/user/keys" {
			// Simulate duplicate key error
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": map[string][]string{
					"key": {"has already been taken"},
				},
			})
		} else if r.Method == "GET" && r.URL.Path == "/api/v4/user/keys" {
			// Return the existing key
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]SSHKey{existingKey})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	key, err := client.CreateSSHKey("duplicate-key", "ssh-ed25519 AAAA...", nil)

	if err != nil {
		t.Errorf("CreateSSHKey() error = %v, want nil (should return existing key)", err)
		return
	}

	if key == nil {
		t.Fatal("CreateSSHKey() returned nil key")
	}

	if key.ID != existingKey.ID {
		t.Errorf("Key ID = %d, want %d", key.ID, existingKey.ID)
	}
}

func TestDeleteSSHKey(t *testing.T) {
	tests := []struct {
		name       string
		keyID      int
		statusCode int
		wantErr    bool
	}{
		{
			name:       "Success with 204 No Content",
			keyID:      1,
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "Success with 200 OK",
			keyID:      2,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "404 Not Found (already deleted)",
			keyID:      999,
			statusCode: http.StatusNotFound,
			wantErr:    false, // Should not error on 404
		},
		{
			name:       "401 Unauthorized",
			keyID:      3,
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "500 Internal Server Error",
			keyID:      4,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("Request method = %q, want DELETE", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			err := client.DeleteSSHKey(tt.keyID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteSSHKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetCurrentUser(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "Success",
			statusCode: http.StatusOK,
			response: map[string]interface{}{
				"id":       123,
				"username": "testuser",
				"email":    "test@example.com",
			},
			wantErr: false,
		},
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			response:   map[string]string{"message": "Unauthorized"},
			wantErr:    true,
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			response:   map[string]string{"message": "Forbidden"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v4/user" {
					t.Errorf("Request path = %q, want %q", r.URL.Path, "/api/v4/user")
				}
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			user, err := client.GetCurrentUser()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && user == nil {
				t.Error("GetCurrentUser() returned nil user")
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "Valid token",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "Invalid token",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":       1,
						"username": "testuser",
					})
				} else {
					json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			err := client.ValidateToken()

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractHostFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
	}{
		{
			name:     "HTTPS URL",
			url:      "https://gitlab.com",
			wantHost: "gitlab.com",
		},
		{
			name:     "HTTP URL",
			url:      "http://gitlab.company.com",
			wantHost: "gitlab.company.com",
		},
		{
			name:     "URL with trailing slash",
			url:      "https://gitlab.example.com/",
			wantHost: "gitlab.example.com",
		},
		{
			name:     "URL with port",
			url:      "https://gitlab.example.com:8080",
			wantHost: "gitlab.example.com:8080",
		},
		{
			name:     "URL with path",
			url:      "https://gitlab.example.com/group/project",
			wantHost: "gitlab.example.com",
		},
		{
			name:     "Just hostname (no scheme, parsed as path)",
			url:      "gitlab.com",
			wantHost: "", // url.Parse treats this as a path, not host
		},
		{
			name:     "Just hostname with trailing slash (no scheme)",
			url:      "gitlab.com/",
			wantHost: "", // url.Parse treats this as a path, not host
		},
		{
			name:     "Invalid URL with special chars",
			url:      "ht!tp://invalid url",
			wantHost: "ht!tp://invalid url",
		},
		{
			name:     "URL without protocol with trailing slash",
			url:      "gitlab.example.com/",
			wantHost: "", // url.Parse treats this as a path, not host
		},
		{
			name:     "Empty string",
			url:      "",
			wantHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractHostFromURL(tt.url)
			if got != tt.wantHost {
				t.Errorf("ExtractHostFromURL(%q) = %q, want %q", tt.url, got, tt.wantHost)
			}
		})
	}
}

func TestDoRequest_ContentType(t *testing.T) {
	tests := []struct {
		name            string
		body            interface{}
		wantContentType string
	}{
		{
			name:            "With body",
			body:            map[string]string{"key": "value"},
			wantContentType: "application/json",
		},
		{
			name:            "Without body",
			body:            nil,
			wantContentType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contentType := r.Header.Get("Content-Type")
				if contentType != tt.wantContentType {
					t.Errorf("Content-Type header = %q, want %q", contentType, tt.wantContentType)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			_, err := client.doRequest("POST", "/test", tt.body)
			if err != nil {
				t.Errorf("doRequest() error = %v", err)
			}
		})
	}
}

func TestListSSHKeys_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.ListSSHKeys()

	if err == nil {
		t.Error("ListSSHKeys() expected error for invalid JSON")
	}
}

func TestCreateSSHKey_TrimWhitespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		key := payload["key"].(string)
		if key != "ssh-ed25519 AAAA... user@host" {
			t.Errorf("Key not trimmed: %q", key)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(SSHKey{ID: 1, Title: "test", Key: key})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	// Test with leading and trailing whitespace
	_, err := client.CreateSSHKey("test", "  ssh-ed25519 AAAA... user@host  \n", nil)
	if err != nil {
		t.Errorf("CreateSSHKey() error = %v", err)
	}
}
