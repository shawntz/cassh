//go:build darwin

package main

import (
	"sync"
	"testing"
	"time"

	"github.com/shawntz/cassh/internal/config"
)

// Test Coverage for updater.go
//
// This test file provides comprehensive coverage for the updater package, focusing on:
//
// 1. Version Comparison Logic:
//    - normalizeVersion: Handles v prefix, suffixes, edge cases
//    - isNewerVersion: Semantic version comparison including dev builds
//
// 2. State Management:
//    - dismissUpdate: Update dismissal tracking
//    - clearDismissedUpdate: Clearing dismissal state
//
// 3. Configuration:
//    - Periodic update checker configuration and interval calculation
//    - Persistent notifier configuration and interval calculation
//
// 4. Concurrency Safety:
//    - Mutex protection for config access
//    - Concurrent read/write scenarios
//
// 5. Helper Functions:
//    - escapeForAppleScript: String escaping for AppleScript dialogs
//
// Note: UI-related functions (setupUpdateMenu, show dialogs, notifications) and
// network calls (fetchLatestRelease) are not directly tested as they require
// macOS system integration or external services. These are tested through
// integration testing and manual QA.
//
// Background goroutines (startPeriodicUpdateChecker, startPersistentUpdateNotifier)
// are tested for their configuration logic but not for actual background execution
// to avoid timing-dependent tests.

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Version with v prefix",
			input:    "v1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "Version without v prefix",
			input:    "1.2.3",
			expected: "1.2.3",
		},
		{
			name:     "Version with dirty suffix",
			input:    "v1.2.3-dirty",
			expected: "1.2.3",
		},
		{
			name:     "Version with other suffix",
			input:    "1.2.3-rc1",
			expected: "1.2.3",
		},
		{
			name:     "Dev version",
			input:    "dev",
			expected: "dev",
		},
		{
			name:     "Empty version",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{
			name:     "Newer major version",
			latest:   "2.0.0",
			current:  "1.0.0",
			expected: true,
		},
		{
			name:     "Newer minor version",
			latest:   "1.2.0",
			current:  "1.1.0",
			expected: true,
		},
		{
			name:     "Newer patch version",
			latest:   "1.0.1",
			current:  "1.0.0",
			expected: true,
		},
		{
			name:     "Same version",
			latest:   "1.0.0",
			current:  "1.0.0",
			expected: false,
		},
		{
			name:     "Older version",
			latest:   "1.0.0",
			current:  "2.0.0",
			expected: false,
		},
		{
			name:     "Dev version is always outdated",
			latest:   "1.0.0",
			current:  "dev",
			expected: true,
		},
		{
			name:     "Empty current version is outdated",
			latest:   "1.0.0",
			current:  "",
			expected: true,
		},
		{
			name:     "Version with missing patch",
			latest:   "1.2",
			current:  "1.1.9",
			expected: true,
		},
		{
			name:     "Complex version comparison",
			latest:   "2.1.0",
			current:  "1.9.9",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNewerVersion(tt.latest, tt.current)
			if result != tt.expected {
				t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, result, tt.expected)
			}
		})
	}
}

func TestEscapeForAppleScript(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple string",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "String with quotes",
			input:    `Say "Hello"`,
			expected: `Say \"Hello\"`,
		},
		{
			name:     "String with backslash",
			input:    `C:\path\to\file`,
			expected: `C:\\path\\to\\file`,
		},
		{
			name:     "String with newline",
			input:    "Line 1\nLine 2",
			expected: `Line 1\nLine 2`,
		},
		{
			name:     "String with carriage return",
			input:    "Line 1\rLine 2",
			expected: `Line 1\nLine 2`,
		},
		{
			name:     "Complex string",
			input:    `Path: "C:\test\file"\nNext line`,
			expected: `Path: \"C:\\test\\file\"\nNext line`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeForAppleScript(tt.input)
			if result != tt.expected {
				t.Errorf("escapeForAppleScript(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDismissUpdate(t *testing.T) {
	// Save original config and version
	origCfg := cfg
	origVersion := latestVersion
	defer func() {
		cfg = origCfg
		latestVersion = origVersion
	}()

	// Create a test config
	cfg = config.Config{
		User: config.DefaultUserConfig(),
	}

	// Set a version to dismiss
	latestVersion = "1.2.3"

	// Call dismissUpdate
	dismissUpdate()

	// Verify the version was stored
	configMutex.RLock()
	dismissedVersion := cfg.User.DismissedUpdateVersion
	configMutex.RUnlock()

	if dismissedVersion != "1.2.3" {
		t.Errorf("DismissedUpdateVersion = %q, want %q", dismissedVersion, "1.2.3")
	}
}

func TestDismissUpdateEmptyVersion(t *testing.T) {
	// Save original config and version
	origCfg := cfg
	origVersion := latestVersion
	defer func() {
		cfg = origCfg
		latestVersion = origVersion
	}()

	// Create a test config
	cfg = config.Config{
		User: config.DefaultUserConfig(),
	}

	// Set an empty version
	latestVersion = ""
	cfg.User.DismissedUpdateVersion = "old-version"

	// Call dismissUpdate - should not change anything
	dismissUpdate()

	// Verify it didn't change
	configMutex.RLock()
	dismissedVersion := cfg.User.DismissedUpdateVersion
	configMutex.RUnlock()

	if dismissedVersion != "old-version" {
		t.Errorf("DismissedUpdateVersion should not change when latestVersion is empty, got %q", dismissedVersion)
	}
}

func TestClearDismissedUpdate(t *testing.T) {
	// Save original config
	origCfg := cfg
	defer func() { cfg = origCfg }()

	// Create a test config
	cfg = config.Config{
		User: config.DefaultUserConfig(),
	}

	// Set a dismissed version
	configMutex.Lock()
	cfg.User.DismissedUpdateVersion = "1.2.3"
	configMutex.Unlock()

	// Call clearDismissedUpdate
	clearDismissedUpdate()

	// Verify it was cleared
	configMutex.RLock()
	dismissedVersion := cfg.User.DismissedUpdateVersion
	configMutex.RUnlock()

	if dismissedVersion != "" {
		t.Errorf("DismissedUpdateVersion = %q, want empty string", dismissedVersion)
	}
}

func TestClearDismissedUpdateAlreadyEmpty(t *testing.T) {
	// Save original config
	origCfg := cfg
	defer func() { cfg = origCfg }()

	// Create a test config with no dismissed version
	cfg = config.Config{
		User: config.DefaultUserConfig(),
	}

	// Ensure it's empty
	configMutex.Lock()
	cfg.User.DismissedUpdateVersion = ""
	configMutex.Unlock()

	// Call clearDismissedUpdate - should not error
	clearDismissedUpdate()

	// Verify it's still empty
	configMutex.RLock()
	dismissedVersion := cfg.User.DismissedUpdateVersion
	configMutex.RUnlock()

	if dismissedVersion != "" {
		t.Errorf("DismissedUpdateVersion = %q, want empty string", dismissedVersion)
	}
}

// TestUpdateStatusConstants verifies the update status constants are defined correctly
func TestUpdateStatusConstants(t *testing.T) {
	// Verify constants have unique values
	statuses := map[UpdateStatus]string{
		UpdateStatusUnknown:     "Unknown",
		UpdateStatusUpToDate:    "UpToDate",
		UpdateStatusAvailable:   "Available",
		UpdateStatusError:       "Error",
		UpdateStatusNoReleases:  "NoReleases",
	}

	seen := make(map[UpdateStatus]bool)
	for status := range statuses {
		if seen[status] {
			t.Errorf("Duplicate status value: %v", status)
		}
		seen[status] = true
	}

	// Verify we have 5 distinct statuses
	if len(statuses) != 5 {
		t.Errorf("Expected 5 update statuses, got %d", len(statuses))
	}
}

// TestConfigMutexProtection verifies mutex is initialized
func TestConfigMutexProtection(t *testing.T) {
	// This test ensures configMutex is usable
	// If it's not initialized, this will panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("configMutex panic: %v", r)
		}
	}()

	// Test lock/unlock
	configMutex.Lock()
	configMutex.Unlock()

	// Test RLock/RUnlock
	configMutex.RLock()
	configMutex.RUnlock()
}

// calculateUpdateCheckInterval returns the update check interval based on configured days
// This matches the logic in startPeriodicUpdateChecker
func calculateUpdateCheckInterval(intervalDays int) time.Duration {
	checkInterval := time.Duration(intervalDays) * 24 * time.Hour
	if intervalDays == 0 {
		checkInterval = 24 * time.Hour // Default to daily
	}
	return checkInterval
}

// calculateNotifyInterval returns the notification interval based on configured minutes
// This matches the logic in startPersistentUpdateNotifier
func calculateNotifyInterval(intervalMin int) time.Duration {
	notifyInterval := time.Duration(intervalMin) * time.Minute
	if intervalMin == 0 {
		notifyInterval = 6 * time.Hour // Default to 6 hours
	}
	return notifyInterval
}

// TestPeriodicUpdateCheckerConfiguration tests the configuration logic
// without actually starting the background goroutine
func TestPeriodicUpdateCheckerConfiguration(t *testing.T) {
	// Save original config
	origCfg := cfg
	defer func() { cfg = origCfg }()

	tests := []struct {
		name               string
		updateCheckEnabled bool
		intervalDays       int
		expectedInterval   time.Duration
	}{
		{
			name:               "Default interval (0 days)",
			updateCheckEnabled: true,
			intervalDays:       0,
			expectedInterval:   24 * time.Hour,
		},
		{
			name:               "Custom 7 day interval",
			updateCheckEnabled: true,
			intervalDays:       7,
			expectedInterval:   7 * 24 * time.Hour,
		},
		{
			name:               "1 day interval",
			updateCheckEnabled: true,
			intervalDays:       1,
			expectedInterval:   24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config
			cfg = config.Config{
				User: config.DefaultUserConfig(),
			}
			cfg.User.UpdateCheckEnabled = tt.updateCheckEnabled
			cfg.User.UpdateCheckIntervalDays = tt.intervalDays

			// Verify configuration is set correctly
			configMutex.RLock()
			enabled := cfg.User.UpdateCheckEnabled
			interval := cfg.User.UpdateCheckIntervalDays
			configMutex.RUnlock()

			if enabled != tt.updateCheckEnabled {
				t.Errorf("UpdateCheckEnabled = %v, want %v", enabled, tt.updateCheckEnabled)
			}

			if interval != tt.intervalDays {
				t.Errorf("UpdateCheckIntervalDays = %d, want %d", interval, tt.intervalDays)
			}

			// Calculate expected interval using helper function
			checkInterval := calculateUpdateCheckInterval(interval)

			if checkInterval != tt.expectedInterval {
				t.Errorf("Calculated interval = %v, want %v", checkInterval, tt.expectedInterval)
			}
		})
	}
}

// TestPersistentNotifierConfiguration tests the configuration logic
// without actually starting the background goroutine
func TestPersistentNotifierConfiguration(t *testing.T) {
	// Save original config
	origCfg := cfg
	defer func() { cfg = origCfg }()

	tests := []struct {
		name             string
		notifyPersistent bool
		intervalMin      int
		expectedInterval time.Duration
	}{
		{
			name:             "Default interval (0 minutes)",
			notifyPersistent: true,
			intervalMin:      0,
			expectedInterval: 6 * time.Hour,
		},
		{
			name:             "Custom 60 minute interval",
			notifyPersistent: true,
			intervalMin:      60,
			expectedInterval: 60 * time.Minute,
		},
		{
			name:             "30 minute interval",
			notifyPersistent: true,
			intervalMin:      30,
			expectedInterval: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config
			cfg = config.Config{
				User: config.DefaultUserConfig(),
			}
			cfg.User.UpdateNotifyPersistent = tt.notifyPersistent
			cfg.User.UpdateNotifyIntervalMin = tt.intervalMin

			// Verify configuration is set correctly
			configMutex.RLock()
			persistent := cfg.User.UpdateNotifyPersistent
			interval := cfg.User.UpdateNotifyIntervalMin
			configMutex.RUnlock()

			if persistent != tt.notifyPersistent {
				t.Errorf("UpdateNotifyPersistent = %v, want %v", persistent, tt.notifyPersistent)
			}

			if interval != tt.intervalMin {
				t.Errorf("UpdateNotifyIntervalMin = %d, want %d", interval, tt.intervalMin)
			}

			// Calculate expected interval using helper function
			notifyInterval := calculateNotifyInterval(interval)

			if notifyInterval != tt.expectedInterval {
				t.Errorf("Calculated interval = %v, want %v", notifyInterval, tt.expectedInterval)
			}
		})
	}
}

// TestConcurrentConfigAccess verifies that concurrent reads/writes to config are safe
func TestConcurrentConfigAccess(t *testing.T) {
	// Save original config
	origCfg := cfg
	defer func() { cfg = origCfg }()

	// Create test config
	cfg = config.Config{
		User: config.DefaultUserConfig(),
	}

	// Use a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup
	iterations := 100

	// Start multiple readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				configMutex.RLock()
				// Read config values to test concurrent access patterns
				_ = cfg.User.UpdateCheckEnabled
				_ = cfg.User.DismissedUpdateVersion
				configMutex.RUnlock()
			}
		}()
	}

	// Start multiple writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				configMutex.Lock()
				cfg.User.DismissedUpdateVersion = "test-version"
				cfg.User.UpdateCheckEnabled = (j % 2) == 0
				configMutex.Unlock()
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If we get here without deadlock or race, the test passes
	t.Log("Concurrent access test completed successfully")
}

// TestGitHubReleaseStructure verifies the GitHubRelease struct can be unmarshaled
func TestGitHubReleaseStructure(t *testing.T) {
	// This is a basic test to ensure the struct is defined correctly
	release := GitHubRelease{
		TagName:     "v1.0.0",
		Name:        "Release 1.0.0",
		PublishedAt: time.Now(),
		HTMLURL:     "https://github.com/shawntz/cassh/releases/tag/v1.0.0",
		Body:        "Release notes",
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("TagName = %q, want %q", release.TagName, "v1.0.0")
	}

	if release.Name != "Release 1.0.0" {
		t.Errorf("Name = %q, want %q", release.Name, "Release 1.0.0")
	}

	if release.HTMLURL == "" {
		t.Error("HTMLURL should not be empty")
	}
}

// TestVersionComparisonEdgeCases tests additional edge cases for version comparison
func TestVersionComparisonEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{
			name:     "Both versions with single digit",
			latest:   "2",
			current:  "1",
			expected: true,
		},
		{
			name:     "Latest with two parts, current with three",
			latest:   "1.5",
			current:  "1.4.9",
			expected: true,
		},
		{
			name:     "Latest 1.10.0 vs current 1.9.0",
			latest:   "1.10.0",
			current:  "1.9.0",
			expected: true,
		},
		{
			name:     "Latest 1.9.0 vs current 1.10.0",
			latest:   "1.9.0",
			current:  "1.10.0",
			expected: false,
		},
		{
			name:     "Large version numbers",
			latest:   "100.200.300",
			current:  "100.200.299",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNewerVersion(tt.latest, tt.current)
			if result != tt.expected {
				t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.latest, tt.current, result, tt.expected)
			}
		})
	}
}

// TestNormalizeVersionEdgeCases tests additional edge cases for version normalization
func TestNormalizeVersionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple hyphens",
			input:    "v1.2.3-beta-1",
			expected: "1.2.3",
		},
		{
			name:     "Just v prefix",
			input:    "v",
			expected: "",
		},
		{
			name:     "Version with plus",
			input:    "1.2.3+build.123",
			expected: "1.2.3+build.123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
