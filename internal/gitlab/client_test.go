package gitlab

import (
	"testing"
)

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "JSON with message field",
			body:     []byte(`{"message": "Invalid credentials"}`),
			expected: "Invalid credentials",
		},
		{
			name:     "JSON with error field",
			body:     []byte(`{"error": "Not found"}`),
			expected: "Not found",
		},
		{
			name:     "JSON with both message and error fields (message takes precedence)",
			body:     []byte(`{"message": "Primary error", "error": "Secondary error"}`),
			expected: "Primary error",
		},
		{
			name:     "JSON with sensitive data should be omitted",
			body:     []byte(`{"token": "secret-token-123", "details": "Internal server error with stack trace"}`),
			expected: "API request failed (response body omitted for security)",
		},
		{
			name:     "Invalid JSON should be omitted",
			body:     []byte(`This is not JSON`),
			expected: "API request failed (response body omitted for security)",
		},
		{
			name:     "Empty body should be omitted",
			body:     []byte(``),
			expected: "API request failed (response body omitted for security)",
		},
		{
			name:     "JSON with no message or error field should be omitted",
			body:     []byte(`{"foo": "bar"}`),
			expected: "API request failed (response body omitted for security)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorMessage(tt.body)
			if result != tt.expected {
				t.Errorf("sanitizeErrorMessage() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestExtractHostFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "HTTPS URL",
			url:      "https://gitlab.com",
			expected: "gitlab.com",
		},
		{
			name:     "HTTP URL",
			url:      "http://gitlab.company.com",
			expected: "gitlab.company.com",
		},
		{
			name:     "URL with trailing slash",
			url:      "https://gitlab.com/",
			expected: "gitlab.com",
		},
		{
			name:     "URL with port",
			url:      "https://gitlab.com:443",
			expected: "gitlab.com",
		},
		{
			name:     "URL with custom port",
			url:      "https://gitlab.company.com:8443",
			expected: "gitlab.company.com",
		},
		{
			name:     "Plain hostname",
			url:      "gitlab.com",
			expected: "gitlab.com",
		},
		{
			name:     "Plain hostname with trailing slash",
			url:      "gitlab.com/",
			expected: "gitlab.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractHostFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractHostFromURL(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
