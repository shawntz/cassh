package gitlab

import (
	"testing"
)

func TestNewClient_EmptyToken(t *testing.T) {
	// Test that NewClient returns an error when token is empty
	_, err := NewClient("https://gitlab.com", "")
	if err == nil {
		t.Error("Expected error when creating client with empty token, got nil")
	}
	
	expectedMsg := "token cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewClient_ValidToken(t *testing.T) {
	// Test that NewClient succeeds with a non-empty token
	client, err := NewClient("https://gitlab.com", "valid-token")
	if err != nil {
		t.Errorf("Expected no error with valid token, got: %v", err)
	}
	if client == nil {
		t.Error("Expected client to be non-nil")
	}
	if client.token != "valid-token" {
		t.Errorf("Expected token to be 'valid-token', got %q", client.token)
	}
	if client.baseURL != "https://gitlab.com" {
		t.Errorf("Expected baseURL to be 'https://gitlab.com', got %q", client.baseURL)
	}
}

func TestNewClient_TrailingSlash(t *testing.T) {
	// Test that trailing slashes are removed from baseURL
	client, err := NewClient("https://gitlab.com/", "valid-token")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if client.baseURL != "https://gitlab.com" {
		t.Errorf("Expected baseURL to have trailing slash removed, got %q", client.baseURL)
	}
}
