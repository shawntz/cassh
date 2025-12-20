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
