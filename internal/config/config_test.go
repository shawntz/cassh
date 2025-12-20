package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultUserConfig(t *testing.T) {
	config := DefaultUserConfig()

	if config.RefreshIntervalSeconds != 30 {
		t.Errorf("RefreshIntervalSeconds = %d, want 30", config.RefreshIntervalSeconds)
	}

	if !config.NotificationSound {
		t.Error("NotificationSound should be true by default")
	}

	if config.PreferredMeme != "random" {
		t.Errorf("PreferredMeme = %q, want %q", config.PreferredMeme, "random")
	}

	if config.SSHKeyPath == "" {
		t.Error("SSHKeyPath should not be empty")
	}

	if config.SSHCertPath == "" {
		t.Error("SSHCertPath should not be empty")
	}

	// Verify paths are in .ssh directory
	if filepath.Base(filepath.Dir(config.SSHKeyPath)) != ".ssh" {
		t.Errorf("SSHKeyPath should be in .ssh directory, got %q", config.SSHKeyPath)
	}

	// Verify cert path matches key path pattern
	expectedCertPath := config.SSHKeyPath + "-cert.pub"
	if config.SSHCertPath != expectedCertPath {
		t.Errorf("SSHCertPath = %q, want %q", config.SSHCertPath, expectedCertPath)
	}
}

func TestServerConfigIsDevMode(t *testing.T) {
	tests := []struct {
		name     string
		config   ServerConfig
		expected bool
	}{
		{
			name:     "DevMode explicitly true",
			config:   ServerConfig{DevMode: true, OIDCTenant: "tenant"},
			expected: true,
		},
		{
			name:     "No tenant means dev mode",
			config:   ServerConfig{DevMode: false, OIDCTenant: ""},
			expected: true,
		},
		{
			name:     "Production mode",
			config:   ServerConfig{DevMode: false, OIDCTenant: "tenant-id"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsDevMode(); got != tt.expected {
				t.Errorf("IsDevMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPolicyConfigIsDevMode(t *testing.T) {
	tests := []struct {
		name     string
		config   PolicyConfig
		expected bool
	}{
		{
			name:     "DevMode explicitly true",
			config:   PolicyConfig{DevMode: true, OIDCTenantID: "tenant"},
			expected: true,
		},
		{
			name:     "No tenant means dev mode",
			config:   PolicyConfig{DevMode: false, OIDCTenantID: ""},
			expected: true,
		},
		{
			name:     "Production mode",
			config:   PolicyConfig{DevMode: false, OIDCTenantID: "tenant-id"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsDevMode(); got != tt.expected {
				t.Errorf("IsDevMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "Valid dev mode config",
			config: ServerConfig{
				ServerBaseURL: "http://localhost:8080",
				DevMode:       true,
			},
			wantErr: false,
		},
		{
			name: "Missing server URL",
			config: ServerConfig{
				DevMode: true,
			},
			wantErr: true,
		},
		{
			name: "Valid production config",
			config: ServerConfig{
				ServerBaseURL:    "https://cassh.example.com",
				OIDCClientID:     "client-id",
				OIDCClientSecret: "client-secret",
				OIDCTenant:       "tenant-id",
				CAPrivateKey:     "-----BEGIN OPENSSH PRIVATE KEY-----",
			},
			wantErr: false,
		},
		{
			name: "Production missing client ID",
			config: ServerConfig{
				ServerBaseURL:    "https://cassh.example.com",
				OIDCClientSecret: "client-secret",
				OIDCTenant:       "tenant-id",
				CAPrivateKey:     "key",
			},
			wantErr: true,
		},
		{
			name: "Production missing client secret",
			config: ServerConfig{
				ServerBaseURL: "https://cassh.example.com",
				OIDCClientID:  "client-id",
				OIDCTenant:    "tenant-id",
				CAPrivateKey:  "key",
			},
			wantErr: true,
		},
		{
			name: "Production missing CA key",
			config: ServerConfig{
				ServerBaseURL:    "https://cassh.example.com",
				OIDCClientID:     "client-id",
				OIDCClientSecret: "client-secret",
				OIDCTenant:       "tenant-id",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// setEnv is a test helper that sets an env var and returns a cleanup function
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	old := os.Getenv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set env var %s: %v", key, err)
	}
	t.Cleanup(func() {
		if old != "" {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// unsetEnv is a test helper that unsets an env var and returns a cleanup function
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old := os.Getenv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Failed to unset env var %s: %v", key, err)
	}
	t.Cleanup(func() {
		if old != "" {
			_ = os.Setenv(key, old)
		}
	})
}

func TestLoadServerConfigFromEnv(t *testing.T) {
	// Clear and set env vars using helpers
	envVars := []string{
		"CASSH_SERVER_URL",
		"CASSH_CERT_VALIDITY_HOURS",
		"CASSH_OIDC_CLIENT_ID",
		"CASSH_OIDC_CLIENT_SECRET",
		"CASSH_OIDC_TENANT",
		"CASSH_DEV_MODE",
	}

	for _, v := range envVars {
		unsetEnv(t, v)
	}

	// Set test env vars
	setEnv(t, "CASSH_SERVER_URL", "https://test.example.com")
	setEnv(t, "CASSH_CERT_VALIDITY_HOURS", "24")
	setEnv(t, "CASSH_OIDC_CLIENT_ID", "test-client")
	setEnv(t, "CASSH_DEV_MODE", "true")

	config, err := LoadServerConfig("")
	if err != nil {
		t.Fatalf("LoadServerConfig() error = %v", err)
	}

	if config.ServerBaseURL != "https://test.example.com" {
		t.Errorf("ServerBaseURL = %q, want %q", config.ServerBaseURL, "https://test.example.com")
	}

	if config.CertValidityHours != 24 {
		t.Errorf("CertValidityHours = %d, want 24", config.CertValidityHours)
	}

	if config.OIDCClientID != "test-client" {
		t.Errorf("OIDCClientID = %q, want %q", config.OIDCClientID, "test-client")
	}

	if !config.DevMode {
		t.Error("DevMode should be true")
	}
}

func TestLoadServerConfigDefaults(t *testing.T) {
	// Clear relevant env vars using helper
	envVars := []string{
		"CASSH_SERVER_URL",
		"CASSH_CERT_VALIDITY_HOURS",
		"CASSH_DEV_MODE",
	}

	for _, v := range envVars {
		unsetEnv(t, v)
	}

	config, err := LoadServerConfig("")
	if err != nil {
		t.Fatalf("LoadServerConfig() error = %v", err)
	}

	// Default validity is 12 hours
	if config.CertValidityHours != 12 {
		t.Errorf("CertValidityHours = %d, want 12 (default)", config.CertValidityHours)
	}
}

func TestLoadServerConfigFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.policy.toml")

	configContent := `
server_base_url = "https://file.example.com"
cert_validity_hours = 6
dev_mode = true

[oidc]
client_id = "file-client"
tenant = "file-tenant"

[github]
enterprise_url = "https://github.corp.com"
allowed_orgs = ["org1", "org2"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Clear env vars to ensure file values are used
	envVars := []string{"CASSH_SERVER_URL", "CASSH_OIDC_CLIENT_ID"}
	for _, v := range envVars {
		unsetEnv(t, v)
	}

	config, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatalf("LoadServerConfig() error = %v", err)
	}

	if config.ServerBaseURL != "https://file.example.com" {
		t.Errorf("ServerBaseURL = %q, want %q", config.ServerBaseURL, "https://file.example.com")
	}

	if config.CertValidityHours != 6 {
		t.Errorf("CertValidityHours = %d, want 6", config.CertValidityHours)
	}

	if config.OIDCClientID != "file-client" {
		t.Errorf("OIDCClientID = %q, want %q", config.OIDCClientID, "file-client")
	}

	if config.GitHubEnterpriseURL != "https://github.corp.com" {
		t.Errorf("GitHubEnterpriseURL = %q, want %q", config.GitHubEnterpriseURL, "https://github.corp.com")
	}

	if len(config.GitHubAllowedOrgs) != 2 {
		t.Errorf("GitHubAllowedOrgs length = %d, want 2", len(config.GitHubAllowedOrgs))
	}
}

func TestMergeConfigs(t *testing.T) {
	policy := &PolicyConfig{
		CAPublicKey:       "ssh-ed25519 AAAA...",
		CertValidityHours: 12,
		ServerBaseURL:     "https://cassh.example.com",
	}

	user := &UserConfig{
		RefreshIntervalSeconds: 60,
		PreferredMeme:          "lsp",
	}

	merged := MergeConfigs(policy, user)

	if merged.Policy.CertValidityHours != 12 {
		t.Errorf("Policy.CertValidityHours = %d, want 12", merged.Policy.CertValidityHours)
	}

	if merged.User.PreferredMeme != "lsp" {
		t.Errorf("User.PreferredMeme = %q, want %q", merged.User.PreferredMeme, "lsp")
	}
}

func TestVerifyPolicyIntegrity(t *testing.T) {
	// Policy without signature should pass
	policy := &PolicyConfig{
		CAPublicKey:       "ssh-ed25519 AAAA...",
		ServerBaseURL:     "https://cassh.example.com",
		CertValidityHours: 12,
		OIDCTenantID:      "tenant-123",
	}

	err := VerifyPolicyIntegrity(policy, "")
	if err != nil {
		t.Errorf("VerifyPolicyIntegrity() with no signature should pass, got error: %v", err)
	}
}

func TestUserConfigPath(t *testing.T) {
	path, err := UserConfigPath()
	if err != nil {
		t.Fatalf("UserConfigPath() error = %v", err)
	}

	if path == "" {
		t.Error("UserConfigPath() returned empty path")
	}

	// Should end with config.toml
	if filepath.Base(path) != "config.toml" {
		t.Errorf("UserConfigPath() = %q, should end with config.toml", path)
	}

	// Should contain cassh in path
	if filepath.Base(filepath.Dir(path)) != "cassh" {
		t.Errorf("UserConfigPath() = %q, should be in cassh directory", path)
	}
}

func TestPolicyPath(t *testing.T) {
	path := PolicyPath()
	if path == "" {
		t.Error("PolicyPath() returned empty path")
	}

	// In test environment, should fall back to local file
	if filepath.Base(path) != "cassh.policy.toml" {
		t.Errorf("PolicyPath() = %q, should end with cassh.policy.toml", path)
	}
}

func TestLoadUserConfigNonExistent(t *testing.T) {
	// LoadUserConfig should return defaults when file doesn't exist
	config, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig() error = %v", err)
	}

	if config == nil {
		t.Fatal("LoadUserConfig() returned nil config")
	}

	// Should have default values
	if config.RefreshIntervalSeconds != 30 {
		t.Errorf("RefreshIntervalSeconds = %d, want 30 (default)", config.RefreshIntervalSeconds)
	}
}

func TestUserConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  UserConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Valid config with defaults",
			config:  DefaultUserConfig(),
			wantErr: false,
		},
		{
			name: "Valid config with zero values",
			config: UserConfig{
				UpdateCheckIntervalDays: 0,
				UpdateNotifyIntervalMin: 0,
				RefreshIntervalSeconds:  0,
			},
			wantErr: false,
		},
		{
			name: "Negative UpdateCheckIntervalDays",
			config: UserConfig{
				UpdateCheckIntervalDays: -1,
			},
			wantErr: true,
			errMsg:  "update_check_interval_days must be non-negative",
		},
		{
			name: "Negative UpdateNotifyIntervalMin",
			config: UserConfig{
				UpdateNotifyIntervalMin: -5,
			},
			wantErr: true,
			errMsg:  "update_notify_interval_min must be non-negative",
		},
		{
			name: "Negative RefreshIntervalSeconds",
			config: UserConfig{
				RefreshIntervalSeconds: -10,
			},
			wantErr: true,
			errMsg:  "refresh_interval_seconds must be non-negative",
		},
		{
			name: "Valid config with large positive values",
			config: UserConfig{
				UpdateCheckIntervalDays: 365,
				UpdateNotifyIntervalMin: 10080, // 1 week in minutes
				RefreshIntervalSeconds:  3600,  // 1 hour
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSaveUserConfigValidation(t *testing.T) {
	// Test that SaveUserConfig rejects invalid configs
	tmpDir := t.TempDir()
	
	// Temporarily change the home directory for this test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)
	
	tests := []struct {
		name    string
		config  UserConfig
		wantErr bool
	}{
		{
			name:    "Valid config",
			config:  DefaultUserConfig(),
			wantErr: false,
		},
		{
			name: "Invalid config - negative UpdateCheckIntervalDays",
			config: UserConfig{
				UpdateCheckIntervalDays: -1,
			},
			wantErr: true,
		},
		{
			name: "Invalid config - negative UpdateNotifyIntervalMin",
			config: UserConfig{
				UpdateNotifyIntervalMin: -5,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveUserConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveUserConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

