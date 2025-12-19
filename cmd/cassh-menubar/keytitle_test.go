//go:build darwin

package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shawntz/cassh/internal/config"
)

func TestGetKeyTitle(t *testing.T) {
	hostname, _ := os.Hostname()
	expectedHostname := strings.ReplaceAll(hostname, ".", "-")
	if len(expectedHostname) > 30 {
		expectedHostname = expectedHostname[:30]
	}

	tests := []struct {
		name     string
		connID   string
		contains []string
	}{
		{
			name:     "personal connection",
			connID:   "personal-1234567890",
			contains: []string{"cassh-personal-1234567890@", expectedHostname},
		},
		{
			name:     "enterprise connection",
			connID:   "enterprise-9876543210",
			contains: []string{"cassh-enterprise-9876543210@", expectedHostname},
		},
		{
			name:     "short ID",
			connID:   "test",
			contains: []string{"cassh-test@", expectedHostname},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getKeyTitle(tt.connID)

			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("getKeyTitle(%q) = %q, should contain %q", tt.connID, result, substr)
				}
			}

			// Verify format: cassh-{connID}@{hostname}
			if !strings.HasPrefix(result, "cassh-") {
				t.Errorf("getKeyTitle(%q) = %q, should start with 'cassh-'", tt.connID, result)
			}

			if !strings.Contains(result, "@") {
				t.Errorf("getKeyTitle(%q) = %q, should contain '@'", tt.connID, result)
			}
		})
	}
}

func TestGetLegacyKeyTitle(t *testing.T) {
	tests := []struct {
		name     string
		connID   string
		expected string
	}{
		{
			name:     "personal connection",
			connID:   "personal-1234567890",
			expected: "cassh-personal-1234567890",
		},
		{
			name:     "enterprise connection",
			connID:   "enterprise-9876543210",
			expected: "cassh-enterprise-9876543210",
		},
		{
			name:     "short ID",
			connID:   "test",
			expected: "cassh-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLegacyKeyTitle(tt.connID)
			if result != tt.expected {
				t.Errorf("getLegacyKeyTitle(%q) = %q, want %q", tt.connID, result, tt.expected)
			}
		})
	}
}

func TestGetKeyTitleHostnameTruncation(t *testing.T) {
	// Test that the function doesn't panic with any input
	// and produces reasonable output
	result := getKeyTitle("test-connection")

	// Should not be empty
	if result == "" {
		t.Error("getKeyTitle() returned empty string")
	}

	// Should not exceed reasonable length (cassh- + connID + @ + 30 char hostname)
	maxExpectedLen := len("cassh-test-connection@") + 30
	if len(result) > maxExpectedLen+10 { // Allow some margin
		t.Errorf("getKeyTitle() result too long: %d chars, expected max ~%d", len(result), maxExpectedLen)
	}
}

func TestNeedsKeyRotation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		conn     *config.Connection
		expected bool
	}{
		{
			name: "enterprise connection - never needs rotation",
			conn: &config.Connection{
				Type:             config.ConnectionTypeEnterprise,
				KeyRotationHours: 4,
				KeyCreatedAt:     now.Add(-5 * time.Hour).Unix(),
			},
			expected: false,
		},
		{
			name: "personal - no rotation configured",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 0,
				KeyCreatedAt:     now.Add(-100 * time.Hour).Unix(),
			},
			expected: false,
		},
		{
			name: "personal - no creation time recorded",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 4,
				KeyCreatedAt:     0,
			},
			expected: false,
		},
		{
			name: "personal - key is fresh (created 1 hour ago, 4h rotation)",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 4,
				KeyCreatedAt:     now.Add(-1 * time.Hour).Unix(),
			},
			expected: false,
		},
		{
			name: "personal - key needs rotation (created 5 hours ago, 4h rotation)",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 4,
				KeyCreatedAt:     now.Add(-5 * time.Hour).Unix(),
			},
			expected: true,
		},
		{
			name: "personal - key exactly at rotation boundary",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 4,
				KeyCreatedAt:     now.Add(-4 * time.Hour).Unix(),
			},
			expected: true,
		},
		{
			name: "personal - key created 1 day ago with 24h rotation",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 24,
				KeyCreatedAt:     now.Add(-24 * time.Hour).Unix(),
			},
			expected: true,
		},
		{
			name: "personal - key created 23 hours ago with 24h rotation",
			conn: &config.Connection{
				Type:             config.ConnectionTypePersonal,
				KeyRotationHours: 24,
				KeyCreatedAt:     now.Add(-23 * time.Hour).Unix(),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := needsKeyRotation(tt.conn)
			if result != tt.expected {
				t.Errorf("needsKeyRotation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestKeyTitleConsistency(t *testing.T) {
	// Test that calling getKeyTitle multiple times returns the same result
	connID := "personal-1234567890"

	first := getKeyTitle(connID)
	second := getKeyTitle(connID)

	if first != second {
		t.Errorf("getKeyTitle() not consistent: first=%q, second=%q", first, second)
	}
}

func TestLegacyAndNewTitlesDifferent(t *testing.T) {
	connID := "personal-1234567890"

	newTitle := getKeyTitle(connID)
	legacyTitle := getLegacyKeyTitle(connID)

	if newTitle == legacyTitle {
		t.Errorf("new and legacy titles should be different: new=%q, legacy=%q", newTitle, legacyTitle)
	}

	// Legacy should be a prefix of new (without the @hostname part)
	if !strings.HasPrefix(newTitle, legacyTitle) {
		t.Errorf("new title %q should start with legacy title %q", newTitle, legacyTitle)
	}
}
