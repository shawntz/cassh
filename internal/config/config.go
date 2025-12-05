// Handles split configuration: immutable policy + user prefs
// Policy config bundled in the signed app so it can't be overridden
// User config allows personal prefs only
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// PolicyConfig contains IT-controlled settings that users can't modify
// This is bundled inside the signed app bundle
type PolicyConfig struct {
	// CA configuration
	CAPublicKey     string `toml:"ca_public_key"`
	CAKeyFingerprint string `toml:"ca_key_fingerprint"`

	// Certificate settings
	CertValidityHours int      `toml:"cert_validity_hours"`
	AllowedPrincipals []string `toml:"allowed_principals"`

	// Server endpoints
	ServerBaseURL string `toml:"server_base_url"`

	// OIDC / Entra settings
	OIDCIssuer   string `toml:"oidc_issuer"`
	OIDCClientID string `toml:"oidc_client_id"`
	OIDCTenantID string `toml:"oidc_tenant_id"`

	// Policy integrity
	PolicyVersion   string `toml:"policy_version"`
	PolicySignature string `toml:"policy_signature"`

	// Devel mode - skips OIDC, uses mock auth
	DevMode bool `toml:"dev_mode"`
}

// IsDevMode returns true if running in devel mode
func (p *PolicyConfig) IsDevMode() bool {
	return p.DevMode || p.OIDCTenantID == ""
}

// UserConfig contains user-editable prefs
// Stored in ~/Library/Application Support/cassh/
type UserConfig struct {
	// UI prefs
	RefreshIntervalSeconds int    `toml:"refresh_interval_seconds"`
	NotificationSound      bool   `toml:"notification_sound"`
	PreferredMeme          string `toml:"preferred_meme"` // "lsp", "sloth", or "random"

	// Local paths (auto-managed)
	SSHKeyPath  string `toml:"ssh_key_path"`
	SSHCertPath string `toml:"ssh_cert_path"`
}

// MergedConfig is the final runtime config
type MergedConfig struct {
	Policy PolicyConfig
	User   UserConfig
}

// ServerConfig contains all server-side configs (including secrets)
// Extends PolicyConfig with server-only fields
type ServerConfig struct {
	// Base settings
	ServerBaseURL     string `toml:"server_base_url"`
	CertValidityHours int    `toml:"cert_validity_hours"`

	// OIDC settings
	OIDCClientID     string `toml:"oidc_client_id"`
	OIDCClientSecret string `toml:"oidc_client_secret"`
	OIDCTenant       string `toml:"oidc_tenant"`
	OIDCRedirectURL  string `toml:"oidc_redirect_url"`

	// CA settings
	CAPrivateKeyPath string `toml:"ca_private_key_path"`
	CAPrivateKey     string `toml:"-"` // Loaded from file or env, never from TOML directly

	// GitHub settings
	GitHubEnterpriseURL string   `toml:"github_enterprise_url"`
	GitHubAllowedOrgs   []string `toml:"github_allowed_orgs"`

	// Devel mode
	DevMode bool `toml:"dev_mode"`
}

// LoadServerConfig loads server config from file, with env var overrides
// Env vars take precedence over file values
//
// Supported env vars:
//   - CASSH_SERVER_URL
//   - CASSH_CERT_VALIDITY_HOURS
//   - CASSH_OIDC_CLIENT_ID
//   - CASSH_OIDC_CLIENT_SECRET
//   - CASSH_OIDC_TENANT
//   - CASSH_OIDC_REDIRECT_URL
//   - CASSH_CA_PRIVATE_KEY (raw key content)
//   - CASSH_CA_PRIVATE_KEY_PATH (path to key file)
//   - CASSH_GITHUB_ENTERPRISE_URL
//   - CASSH_DEV_MODE
func LoadServerConfig(policyPath string) (*ServerConfig, error) {
	config := &ServerConfig{
		CertValidityHours: 12, // Default
	}

	// Try to load from file first
	if policyPath != "" {
		if data, err := os.ReadFile(policyPath); err == nil {
			// Parse TOML - supports nested [oidc], [ca], [github] sections
			var fileConfig struct {
				ServerBaseURL     string `toml:"server_base_url"`
				CertValidityHours int    `toml:"cert_validity_hours"`
				DevMode           bool   `toml:"dev_mode"`
				OIDC              struct {
					ClientID     string `toml:"client_id"`
					ClientSecret string `toml:"client_secret"`
					Tenant       string `toml:"tenant"`
					RedirectURL  string `toml:"redirect_url"`
				} `toml:"oidc"`
				CA struct {
					PrivateKeyPath string `toml:"private_key_path"`
				} `toml:"ca"`
				GitHub struct {
					EnterpriseURL string   `toml:"enterprise_url"`
					AllowedOrgs   []string `toml:"allowed_orgs"`
				} `toml:"github"`
			}

			if err := toml.Unmarshal(data, &fileConfig); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}

			config.ServerBaseURL = fileConfig.ServerBaseURL
			if fileConfig.CertValidityHours > 0 {
				config.CertValidityHours = fileConfig.CertValidityHours
			}
			config.DevMode = fileConfig.DevMode
			config.OIDCClientID = fileConfig.OIDC.ClientID
			config.OIDCClientSecret = fileConfig.OIDC.ClientSecret
			config.OIDCTenant = fileConfig.OIDC.Tenant
			config.OIDCRedirectURL = fileConfig.OIDC.RedirectURL
			config.CAPrivateKeyPath = fileConfig.CA.PrivateKeyPath
			config.GitHubEnterpriseURL = fileConfig.GitHub.EnterpriseURL
			config.GitHubAllowedOrgs = fileConfig.GitHub.AllowedOrgs
		}
	}

	// Override with env vars
	if v := os.Getenv("CASSH_SERVER_URL"); v != "" {
		config.ServerBaseURL = v
	}
	if v := os.Getenv("CASSH_CERT_VALIDITY_HOURS"); v != "" {
		if hours, err := strconv.Atoi(v); err == nil {
			config.CertValidityHours = hours
		}
	}
	if v := os.Getenv("CASSH_OIDC_CLIENT_ID"); v != "" {
		config.OIDCClientID = v
	}
	if v := os.Getenv("CASSH_OIDC_CLIENT_SECRET"); v != "" {
		config.OIDCClientSecret = v
	}
	if v := os.Getenv("CASSH_OIDC_TENANT"); v != "" {
		config.OIDCTenant = v
	}
	if v := os.Getenv("CASSH_OIDC_REDIRECT_URL"); v != "" {
		config.OIDCRedirectURL = v
	}
	if v := os.Getenv("CASSH_CA_PRIVATE_KEY_PATH"); v != "" {
		config.CAPrivateKeyPath = v
	}
	if v := os.Getenv("CASSH_GITHUB_ENTERPRISE_URL"); v != "" {
		config.GitHubEnterpriseURL = v
	}
	if v := os.Getenv("CASSH_DEV_MODE"); v == "true" || v == "1" {
		config.DevMode = true
	}

	// Load CA private key - from env var or file
	if v := os.Getenv("CASSH_CA_PRIVATE_KEY"); v != "" {
		// Handle escaped newlines from cloud platform env vars
		config.CAPrivateKey = strings.ReplaceAll(v, "\\n", "\n")
	} else if config.CAPrivateKeyPath != "" {
		keyData, err := os.ReadFile(config.CAPrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA private key from %s: %w", config.CAPrivateKeyPath, err)
		}
		config.CAPrivateKey = string(keyData)
	}

	return config, nil
}

// IsDevMode returns true if running in devel mode
func (c *ServerConfig) IsDevMode() bool {
	return c.DevMode || c.OIDCTenant == ""
}

// Validate checks that required config is present
func (c *ServerConfig) Validate() error {
	if c.ServerBaseURL == "" {
		return fmt.Errorf("server_base_url is required (set CASSH_SERVER_URL)")
	}
	if !c.IsDevMode() {
		if c.OIDCClientID == "" {
			return fmt.Errorf("OIDC client_id is required (set CASSH_OIDC_CLIENT_ID)")
		}
		if c.OIDCClientSecret == "" {
			return fmt.Errorf("OIDC client_secret is required (set CASSH_OIDC_CLIENT_SECRET)")
		}
		if c.OIDCTenant == "" {
			return fmt.Errorf("OIDC tenant is required (set CASSH_OIDC_TENANT)")
		}
		if c.CAPrivateKey == "" {
			return fmt.Errorf("CA private key is required (set CASSH_CA_PRIVATE_KEY or CASSH_CA_PRIVATE_KEY_PATH)")
		}
	}
	return nil
}

// DefaultUserConfig returns sensible defaults for user prefs
func DefaultUserConfig() UserConfig {
	homeDir, _ := os.UserHomeDir()
	return UserConfig{
		RefreshIntervalSeconds: 30,
		NotificationSound:      true,
		PreferredMeme:          "random",
		SSHKeyPath:             filepath.Join(homeDir, ".ssh", "cassh_id_ed25519"),
		SSHCertPath:            filepath.Join(homeDir, ".ssh", "cassh_id_ed25519-cert.pub"),
	}
}

// LoadPolicy loads the immutable policy config from app bundle
func LoadPolicy(policyPath string) (*PolicyConfig, error) {
	data, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	var policy PolicyConfig
	if err := toml.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy file: %w", err)
	}

	return &policy, nil
}

// LoadUserConfig loads user prefs from the config dir
func LoadUserConfig() (*UserConfig, error) {
	configPath, err := UserConfigPath()
	if err != nil {
		return nil, err
	}

	// If config doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaults := DefaultUserConfig()
		return &defaults, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	config := DefaultUserConfig() // Start with defaults
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	return &config, nil
}

// SaveUserConfig persists user prefs
func SaveUserConfig(config *UserConfig) error {
	configPath, err := UserConfigPath()
	if err != nil {
		return err
	}

	// Ensure dir exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// UserConfigPath returns path to the user config file
func UserConfigPath() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, "Library", "Application Support", "cassh")
	case "linux":
		configDir = filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "cassh")
		if configDir == "/cassh" {
			homeDir, _ := os.UserHomeDir()
			configDir = filepath.Join(homeDir, ".config", "cassh")
		}
	default:
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".cassh")
	}

	return filepath.Join(configDir, "config.toml"), nil
}

// PolicyPath returns expected policy file location based on build mode
func PolicyPath() string {
	// Check if running from app bundle (macOS)
	executable, _ := os.Executable()
	bundlePath := filepath.Join(filepath.Dir(executable), "..", "Resources", "cassh.policy.toml")
	if _, err := os.Stat(bundlePath); err == nil {
		return bundlePath
	}

	// Fallback for devel / OSS mode
	return "cassh.policy.toml"
}

// VerifyPolicyIntegrity checks that policy hasn't been tampered with
// TODO: In prod should verify a cryptographic signature
func VerifyPolicyIntegrity(policy *PolicyConfig, expectedFingerprint string) error {
	// Simple hash check for integrity
	policyData := fmt.Sprintf("%s:%s:%d:%s",
		policy.CAPublicKey,
		policy.ServerBaseURL,
		policy.CertValidityHours,
		policy.OIDCTenantID,
	)

	hash := sha256.Sum256([]byte(policyData))
	fingerprint := hex.EncodeToString(hash[:8])

	if policy.PolicySignature != "" && fingerprint != policy.PolicySignature {
		return fmt.Errorf("policy integrity check failed - contact IT")
	}

	return nil
}

// MergeConfigs creates the final runtime config
// Policy vals always win over user values for sec-critical settings
func MergeConfigs(policy *PolicyConfig, user *UserConfig) *MergedConfig {
	return &MergedConfig{
		Policy: *policy,
		User:   *user,
	}
}
