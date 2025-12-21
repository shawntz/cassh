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

	// Platform configuration (GitHub or GitLab)
	Platform string `toml:"platform"` // "github" or "gitlab"

	// GitHub Enterprise
	GitHubEnterpriseURL string `toml:"github_enterprise_url"`

	// GitLab Enterprise
	GitLabEnterpriseURL string `toml:"gitlab_enterprise_url"`

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

// Platform identifies the git platform (GitHub or GitLab)
type Platform string

const (
	PlatformGitHub Platform = "github"
	PlatformGitLab Platform = "gitlab"
)

// ConnectionType identifies the type of connection
type ConnectionType string

const (
	ConnectionTypeEnterprise ConnectionType = "enterprise"
	ConnectionTypePersonal   ConnectionType = "personal"
)

// Connection represents a single git platform connection (enterprise or personal, GitHub or GitLab)
type Connection struct {
	// Unique identifier for this connection
	ID string `toml:"id"`

	// Platform: "github" or "gitlab"
	Platform Platform `toml:"platform"`

	// Type of connection: "enterprise" or "personal"
	Type ConnectionType `toml:"type"`

	// Display name (e.g., "GitHub Enterprise", "GitLab.com", etc.)
	Name string `toml:"name"`

	// For enterprise: the cassh server URL
	ServerURL string `toml:"server_url,omitempty"`

	// Platform hostname
	// For GitHub: "github.com" or "github.yourcompany.com"
	// For GitLab: "gitlab.com" or "gitlab.yourcompany.com"
	Host string `toml:"host"`

	// SSH username for connections
	// For GitHub Enterprise: the SSH username from clone URL (e.g., "yourcorp_123456")
	// For GitHub Personal: GitHub username (from gh auth)
	// For GitLab: GitLab username
	Username string `toml:"username,omitempty"`

	// SSH key paths for this connection
	SSHKeyPath  string `toml:"ssh_key_path"`
	SSHCertPath string `toml:"ssh_cert_path,omitempty"` // Only for enterprise

	// For personal: key rotation settings
	KeyRotationHours int   `toml:"key_rotation_hours,omitempty"` // 0 = no rotation
	KeyCreatedAt     int64 `toml:"key_created_at,omitempty"`     // Unix timestamp

	// SSH Key IDs for deletion on personal connections
	// Note: GitHub uses string IDs while GitLab uses integer IDs due to
	// differences in their respective API designs
	GitHubKeyID string `toml:"github_key_id,omitempty"` // For GitHub personal
	GitLabKeyID int    `toml:"gitlab_key_id,omitempty"` // For GitLab personal

	// For GitLab personal: Personal Access Token for API authentication
	// This is stored in user config since it's user-specific
	// For security, consider using system keychain in future
	GitLabToken string `toml:"gitlab_token,omitempty"`

	// Deprecated fields (for backwards compatibility with GitHub-only configs)
	GitHubHost     string `toml:"github_host,omitempty"`     // Use Host instead
	GitHubUsername string `toml:"github_username,omitempty"` // Use Username instead

	// Connection status (not persisted, runtime only)
	IsActive bool `toml:"-"`
}

// IsGitHub returns true if this is a GitHub connection.
//
// NOTE: For historical reasons, an empty Platform value is treated as GitHub.
// Older configs did not persist the Platform field at all, so connections created
// before Platform was introduced will have Platform == "" and must continue to be
// interpreted as GitHub. Do not change this behavior unless you also provide a
// migration path for existing user configurations.
func (c *Connection) IsGitHub() bool {
	return c.Platform == PlatformGitHub || c.Platform == ""
}

// IsGitLab returns true if this is a GitLab connection
func (c *Connection) IsGitLab() bool {
	return c.Platform == PlatformGitLab
}

// GetHost returns the platform hostname, handling deprecated fields
func (c *Connection) GetHost() string {
	if c.Host != "" {
		return c.Host
	}
	// Backwards compatibility
	if c.GitHubHost != "" {
		return c.GitHubHost
	}
	return ""
}

// GetUsername returns the username, handling deprecated fields
func (c *Connection) GetUsername() string {
	if c.Username != "" {
		return c.Username
	}
	// Backwards compatibility
	if c.GitHubUsername != "" {
		return c.GitHubUsername
	}
	return ""
}

// MigrateDeprecatedFields migrates old GitHub-only fields to new platform-agnostic fields
func (c *Connection) MigrateDeprecatedFields() {
	// If platform is empty, default to GitHub
	if c.Platform == "" {
		c.Platform = PlatformGitHub
	}

	// Migrate host field
	if c.Host == "" && c.GitHubHost != "" {
		c.Host = c.GitHubHost
	}

	// Migrate username field
	if c.Username == "" && c.GitHubUsername != "" {
		c.Username = c.GitHubUsername
	}
}

// UserConfig contains user-editable prefs
// Stored in ~/.config/cassh/config.toml (dotfiles) or ~/Library/Application Support/cassh/ (macOS)
type UserConfig struct {
	// UI prefs
	RefreshIntervalSeconds int    `toml:"refresh_interval_seconds"`
	NotificationSound      bool   `toml:"notification_sound"`
	PreferredMeme          string `toml:"preferred_meme"` // "lsp", "sloth", or "random"
	ShowInDock             bool   `toml:"show_in_dock"`   // Show app icon in Dock

	// Connections (enterprise and/or personal GitHub accounts)
	Connections []Connection `toml:"connections"`

	// Legacy fields for backwards compatibility (deprecated, use Connections)
	SSHKeyPath  string `toml:"ssh_key_path,omitempty"`
	SSHCertPath string `toml:"ssh_cert_path,omitempty"`

	// Runtime state (not persisted)
	usingDotfiles bool `toml:"-"` // True if loaded from ~/.config/cassh/config.toml
}

// HasConnections returns true if the user has any connections configured
func (u *UserConfig) HasConnections() bool {
	return len(u.Connections) > 0
}

// GetConnection returns a connection by ID
func (u *UserConfig) GetConnection(id string) *Connection {
	for i := range u.Connections {
		if u.Connections[i].ID == id {
			return &u.Connections[i]
		}
	}
	return nil
}

// AddConnection adds a new connection
func (u *UserConfig) AddConnection(conn Connection) {
	u.Connections = append(u.Connections, conn)
}

// RemoveConnection removes a connection by ID
func (u *UserConfig) RemoveConnection(id string) bool {
	for i, conn := range u.Connections {
		if conn.ID == id {
			u.Connections = append(u.Connections[:i], u.Connections[i+1:]...)
			return true
		}
	}
	return false
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

	// Platform configuration
	Platform string `toml:"platform"` // "github" or "gitlab"

	// GitHub settings
	GitHubEnterpriseURL string   `toml:"github_enterprise_url"`
	GitHubAllowedOrgs   []string `toml:"github_allowed_orgs"`
	// PrincipalSource determines how to derive the SSH certificate principal from OIDC claims
	// Options: "email_prefix" (default), "email", "username", or a custom claim name
	GitHubPrincipalSource string `toml:"github_principal_source"`

	// GitLab settings
	GitLabEnterpriseURL string   `toml:"gitlab_enterprise_url"`
	GitLabAllowedGroups []string `toml:"gitlab_allowed_groups"`
	// PrincipalSource for GitLab (same options as GitHub)
	GitLabPrincipalSource string `toml:"gitlab_principal_source"`

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
//   - CASSH_PLATFORM (github or gitlab)
//   - CASSH_GITHUB_ENTERPRISE_URL
//   - CASSH_GITHUB_PRINCIPAL_SOURCE (email_prefix, email, username)
//   - CASSH_GITLAB_ENTERPRISE_URL
//   - CASSH_GITLAB_PRINCIPAL_SOURCE (email_prefix, email, username)
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
				Platform string `toml:"platform"`
				GitHub   struct {
					EnterpriseURL   string   `toml:"enterprise_url"`
					AllowedOrgs     []string `toml:"allowed_orgs"`
					PrincipalSource string   `toml:"principal_source"`
				} `toml:"github"`
				GitLab struct {
					EnterpriseURL   string   `toml:"enterprise_url"`
					AllowedGroups   []string `toml:"allowed_groups"`
					PrincipalSource string   `toml:"principal_source"`
				} `toml:"gitlab"`
			}

			if err := toml.Unmarshal(data, &fileConfig); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}

			config.ServerBaseURL = fileConfig.ServerBaseURL
			if fileConfig.CertValidityHours > 0 {
				config.CertValidityHours = fileConfig.CertValidityHours
			}
			config.DevMode = fileConfig.DevMode
			config.Platform = fileConfig.Platform
			config.OIDCClientID = fileConfig.OIDC.ClientID
			config.OIDCClientSecret = fileConfig.OIDC.ClientSecret
			config.OIDCTenant = fileConfig.OIDC.Tenant
			config.OIDCRedirectURL = fileConfig.OIDC.RedirectURL
			config.CAPrivateKeyPath = fileConfig.CA.PrivateKeyPath
			config.GitHubEnterpriseURL = fileConfig.GitHub.EnterpriseURL
			config.GitHubAllowedOrgs = fileConfig.GitHub.AllowedOrgs
			config.GitHubPrincipalSource = fileConfig.GitHub.PrincipalSource
			config.GitLabEnterpriseURL = fileConfig.GitLab.EnterpriseURL
			config.GitLabAllowedGroups = fileConfig.GitLab.AllowedGroups
			config.GitLabPrincipalSource = fileConfig.GitLab.PrincipalSource
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
	if v := os.Getenv("CASSH_PLATFORM"); v != "" {
		platform := strings.ToLower(v)
		if platform == "github" || platform == "gitlab" {
			config.Platform = platform
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid CASSH_PLATFORM value %q; expected \"github\" or \"gitlab\". Keeping existing platform configuration.\n", v)
		}
	}
	if v := os.Getenv("CASSH_GITHUB_PRINCIPAL_SOURCE"); v != "" {
		config.GitHubPrincipalSource = v
	}
	if v := os.Getenv("CASSH_GITLAB_ENTERPRISE_URL"); v != "" {
		config.GitLabEnterpriseURL = v
	}
	if v := os.Getenv("CASSH_GITLAB_PRINCIPAL_SOURCE"); v != "" {
		config.GitLabPrincipalSource = v
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

	// Use intermediate struct to handle nested sections
	var fileConfig struct {
		PolicyConfig
		GitHub struct {
			EnterpriseURL string   `toml:"enterprise_url"`
			AllowedOrgs   []string `toml:"allowed_orgs"`
					PrincipalSource string   `toml:"principal_source"`
		} `toml:"github"`
	}

	if err := toml.Unmarshal(data, &fileConfig); err != nil {
		return nil, fmt.Errorf("failed to parse policy file: %w", err)
	}

	policy := fileConfig.PolicyConfig

	// Map nested [github] section to flat fields
	if fileConfig.GitHub.EnterpriseURL != "" {
		policy.GitHubEnterpriseURL = fileConfig.GitHub.EnterpriseURL
	}

	return &policy, nil
}

// LoadUserConfig loads user prefs from the config dir
// Checks dotfiles location (~/.config/cassh/config.toml) first, then platform-specific location
func LoadUserConfig() (*UserConfig, error) {
	// Check dotfiles location first (cross-platform, easy to backup)
	dotfilesPath := DotfilesConfigPath()
	if _, err := os.Stat(dotfilesPath); err == nil {
		data, err := os.ReadFile(dotfilesPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read dotfiles config: %w", err)
		}

		config := DefaultUserConfig()
		if err := toml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse dotfiles config: %w", err)
		}

		// Migrate deprecated fields for backwards compatibility
		for i := range config.Connections {
			config.Connections[i].MigrateDeprecatedFields()
		}

		// Mark that we're using dotfiles config
		config.usingDotfiles = true
		return &config, nil
	}

	// Fall back to platform-specific location
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

	// Migrate deprecated fields for backwards compatibility
	for i := range config.Connections {
		config.Connections[i].MigrateDeprecatedFields()
	}

	return &config, nil
}

// SaveUserConfig persists user prefs
// Always saves to dotfiles location (~/.config/cassh/config.toml) for easy backup
func SaveUserConfig(config *UserConfig) error {
	// Always use dotfiles location for new saves - it's user-friendly and backup-friendly
	configPath := DotfilesConfigPath()

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

// SaveUserConfigToDotfiles saves config to the dotfiles location (~/.config/cassh/config.toml)
// This can be used to migrate config to the dotfiles location
func SaveUserConfigToDotfiles(config *UserConfig) error {
	configPath := DotfilesConfigPath()

	// Ensure dir exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create dotfiles config directory: %w", err)
	}

	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write dotfiles config: %w", err)
	}

	// Mark as using dotfiles for future saves
	config.usingDotfiles = true
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

// DotfilesConfigPath returns path to the dotfiles config location
// This is always ~/.config/cassh/config.toml regardless of platform
// Users can create this file to use as their primary config (easy to backup with dotfiles)
func DotfilesConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "cassh", "config.toml")
}

// UsingDotfiles returns true if the config was loaded from the dotfiles location
func (u *UserConfig) UsingDotfiles() bool {
	return u.usingDotfiles
}

// SetUsingDotfiles marks the config as using the dotfiles location
func (u *UserConfig) SetUsingDotfiles(using bool) {
	u.usingDotfiles = using
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

// NeedsSetup returns true if the app needs to show the setup wizard
// This happens when:
// - No connections are configured in user config
// - AND the bundled policy doesn't have a valid server URL (OSS mode)
func NeedsSetup(policy *PolicyConfig, user *UserConfig) bool {
	// If user has connections configured, no setup needed
	if user.HasConnections() {
		return false
	}

	// If policy has a valid server URL (enterprise mode), no setup needed
	// The policy server URL acts as the default enterprise connection
	if policy != nil && policy.ServerBaseURL != "" {
		return false
	}

	// No connections and no enterprise policy = needs setup
	return true
}

// IsEnterpriseMode returns true if the app is running in enterprise mode
// (has a bundled policy with server URL)
func IsEnterpriseMode(policy *PolicyConfig) bool {
	return policy != nil && policy.ServerBaseURL != ""
}

// CreateEnterpriseConnectionFromPolicy creates a Connection from the bundled policy
// This is used for enterprise deployments where IT bundles the config
func CreateEnterpriseConnectionFromPolicy(policy *PolicyConfig) *Connection {
	if policy == nil || policy.ServerBaseURL == "" {
		return nil
	}

	homeDir, _ := os.UserHomeDir()

	return &Connection{
		ID:          "enterprise-default",
		Type:        ConnectionTypeEnterprise,
		Name:        "GitHub Enterprise",
		ServerURL:   policy.ServerBaseURL,
		GitHubHost:  ExtractHostFromURL(policy.GitHubEnterpriseURL),
		SSHKeyPath:  filepath.Join(homeDir, ".ssh", "cassh_id_ed25519"),
		SSHCertPath: filepath.Join(homeDir, ".ssh", "cassh_id_ed25519-cert.pub"),
	}
}

// ExtractHostFromURL extracts the hostname from a URL
func ExtractHostFromURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	// Simple extraction - remove protocol
	host := urlStr
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	// Remove path
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	return host
}
