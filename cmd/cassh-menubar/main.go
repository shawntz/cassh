//go:build darwin

// cassh-menubar is the macOS menu bar application
// It shows cert status and handles automatic cert installation
// Supports both GitHub Enterprise (certificate-based) and GitHub.com (key-based) auth
package main

import (
	"bufio"
	"crypto/ed25519"
	"embed"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/shawntz/cassh/internal/ca"
	"github.com/shawntz/cassh/internal/config"
	"golang.org/x/crypto/ssh"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

const (
	loopbackPort      = 52849     // cassh loopback listener port
	expiryWarningTime = time.Hour // Warn when cert expires within this duration
)

// Build variables set by ldflags
var (
	version     = "dev"
	buildCommit = "dev"
	buildTime   = ""
)

var (
	cfg        *config.MergedConfig
	needsSetup bool
	templates  *template.Template

	// Menu items
	menuStatus      *systray.MenuItem
	menuConnections []*systray.MenuItem // Dynamic list of connection menu items
	menuRevokeItems []*systray.MenuItem // Dynamic list of revoke menu items
	menuAddConn     *systray.MenuItem
	menuQuit        *systray.MenuItem

	// Connection status tracking (keyed by connection ID)
	connectionStatus map[string]*ConnectionStatus
)

// ConnectionStatus tracks the status of a single connection
type ConnectionStatus struct {
	ConnectionID       string
	Valid              bool
	TimeLeft           time.Duration
	ValidBefore        time.Time
	LastCheck          time.Time
	NotifiedActivation bool
	NotifiedExpiring   bool
	NotifiedExpired    bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Check for headless mode flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--rotate-keys":
			runHeadlessKeyRotation()
			return
		case "--version":
			fmt.Printf("cassh %s (commit %s)\n", version, buildCommit)
			return
		}
	}

	log.Println("Starting cassh-menubar...")

	// Initialize native notifications (request permission)
	initNotifications()

	// Register URL scheme handler for cassh:// URLs
	registerURLSchemeHandler()

	// Register as login item (triggers system prompt on first run)
	// Uses SMAppService on macOS 13+, falls back to LaunchAgent on older versions
	registerAsLoginItem()

	// Load templates for setup wizard
	var err error
	templates, err = template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Printf("Warning: Could not load templates: %v", err)
	}

	// Load config
	// Try devel policy first, then default policy path
	policyPath := config.PolicyPath()
	if _, err := os.Stat("cassh.policy.dev.toml"); err == nil {
		policyPath = "cassh.policy.dev.toml"
	}

	policy, err := config.LoadPolicy(policyPath)
	if err != nil {
		log.Printf("Warning: Could not load policy, using defaults: %v", err)
		policy = &config.PolicyConfig{
			CertValidityHours: 12,
		}
	}

	userCfg, err := config.LoadUserConfig()
	if err != nil {
		log.Printf("Warning: Could not load user config: %v", err)
		defaults := config.DefaultUserConfig()
		userCfg = &defaults
	}

	cfg = config.MergeConfigs(policy, userCfg)
	connectionStatus = make(map[string]*ConnectionStatus)

	// Apply visibility settings (dock/menu bar)
	applyVisibilitySettings()

	// Check if setup is needed (OSS mode with no connections configured)
	needsSetup = config.NeedsSetup(&cfg.Policy, &cfg.User)

	// Only auto-create enterprise connection for true enterprise deployments:
	// - Policy must have server URL (enterprise mode)
	// - Policy must be loaded from app bundle (not local dev file)
	// - User must have no connections yet
	// - Not in dev mode
	// This prevents OSS/personal users from seeing an enterprise connection by default
	policyFromBundle := strings.Contains(policyPath, ".app/Contents/Resources")
	if config.IsEnterpriseMode(&cfg.Policy) && !cfg.User.HasConnections() && policyFromBundle && !cfg.Policy.IsDevMode() {
		if conn := config.CreateEnterpriseConnectionFromPolicy(&cfg.Policy); conn != nil {
			cfg.User.AddConnection(*conn)
			// Save the connection
			if err := config.SaveUserConfig(&cfg.User); err != nil {
				log.Printf("Warning: Could not save user config: %v", err)
			}
		}
	}

	// For OSS/personal mode: if no connections and policy is not from app bundle, show setup
	// This ensures personal users always see the setup wizard, even with local policy files
	if !cfg.User.HasConnections() && !policyFromBundle {
		needsSetup = true
	}

	// Initialize connection status for all connections
	for _, conn := range cfg.User.Connections {
		connectionStatus[conn.ID] = &ConnectionStatus{ConnectionID: conn.ID}
	}

	// Check for keys that need rotation on startup
	go checkAndRotateExpiredKeys()

	// Start loopback listener for auto-install and setup wizard
	go startLoopbackListener()

	// Start connection monitors (if we have connections)
	if !needsSetup {
		go monitorConnections()
	}

	// Run systray
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTemplateIcon(terminalIcon, terminalIcon)
	systray.SetTooltip(fmt.Sprintf("cassh v%s", version))

	if needsSetup {
		// Setup mode - show setup wizard prompt
		menuStatus = systray.AddMenuItem("Setup Required", "Click to configure cassh")
		menuStatus.Disable()

		systray.AddSeparator()

		menuAddConn = systray.AddMenuItem("Open Setup Wizard...", "Configure cassh for GitHub")

		systray.AddSeparator()

		// Help submenu
		menuHelp := systray.AddMenuItem("Help", "Help and support options")
		menuHelpDocs := menuHelp.AddSubMenuItem("Documentation", "View cassh documentation")
		menuHelpBug := menuHelp.AddSubMenuItem("Report a Bug", "Report an issue on GitHub")
		menuHelpFeature := menuHelp.AddSubMenuItem("Request a Feature", "Suggest a new feature")

		// Community submenu
		menuCommunity := systray.AddMenuItem("Community", "Community and support")
		menuCommunityContribute := menuCommunity.AddSubMenuItem("Contribute", "Contribute to cassh on GitHub")
		menuCommunitySponsor := menuCommunity.AddSubMenuItem("Sponsor", "Support cassh development")
		menuCommunityShare := menuCommunity.AddSubMenuItem("Share cassh...", "Share cassh with friends")

		// Appearance submenu
		setupVisibilityMenu()

		systray.AddSeparator()

		menuUpdates := setupUpdateMenu()
		menuAbout := systray.AddMenuItem("About cassh", "About this application")
		menuVersion := systray.AddMenuItem(fmt.Sprintf("Version %s", version), "")
		menuVersion.Disable()

		systray.AddSeparator()

		menuUninstall := systray.AddMenuItem("Uninstall cassh...", "Remove cassh from your system")
		menuQuit = systray.AddMenuItem("Quit", "Quit cassh")

		// Handle menu clicks
		go func() {
			for {
				select {
				case <-menuAddConn.ClickedCh:
					openSetupWizard()
				case <-menuHelpDocs.ClickedCh:
					openBrowser("https://shawnschwartz.com/cassh")
				case <-menuHelpBug.ClickedCh:
					openBrowser("https://github.com/shawntz/cassh/issues/new?template=bug_report.md")
				case <-menuHelpFeature.ClickedCh:
					openBrowser("https://github.com/shawntz/cassh/issues/new?template=feature_request.md")
				case <-menuCommunityContribute.ClickedCh:
					openBrowser("https://github.com/shawntz/cassh?tab=contributing-ov-file")
				case <-menuCommunitySponsor.ClickedCh:
					openBrowser("https://github.com/sponsors/shawntz")
				case <-menuCommunityShare.ClickedCh:
					showShareDialog()
				case <-menuShowInDock.ClickedCh:
					handleShowInDockToggle()
				case <-menuUpdates.ClickedCh:
					handleUpdateMenuClick()
				case <-menuAbout.ClickedCh:
					showAbout()
				case <-menuUninstall.ClickedCh:
					uninstallCassh()
				case <-menuQuit.ClickedCh:
					systray.Quit()
				}
			}
		}()

		// Check for updates in background
		go checkForUpdatesBackground()

		// Auto-open setup wizard on first launch
		go func() {
			time.Sleep(500 * time.Millisecond)
			openSetupWizard()
		}()
	} else {
		// Normal mode - show connections and their status
		buildConnectionMenu()
	}
}

// buildConnectionMenu creates menu items for all configured connections
func buildConnectionMenu() {
	// Add connection status items
	for i, conn := range cfg.User.Connections {
		statusText := fmt.Sprintf("%s: Checking...", conn.Name)
		menuItem := systray.AddMenuItem(statusText, fmt.Sprintf("Status for %s", conn.Name))
		menuItem.Disable()
		menuConnections = append(menuConnections, menuItem)

		// Add action item for this connection
		actionText := "Generate / Renew"
		if conn.Type == config.ConnectionTypePersonal {
			actionText = "Refresh Key"
		}
		actionItem := systray.AddMenuItem(fmt.Sprintf("  %s", actionText), fmt.Sprintf("Generate/renew for %s", conn.Name))

		// Add revoke item for this connection (starts disabled until cert is verified)
		revokeItem := systray.AddMenuItem("  Revoke Certificate", fmt.Sprintf("Revoke certificate for %s", conn.Name))
		revokeItem.Disable() // Disabled by default, enabled when cert is active
		menuRevokeItems = append(menuRevokeItems, revokeItem)

		// Capture connection for closure
		connID := conn.ID
		connIdx := i
		go func() {
			for range actionItem.ClickedCh {
				handleConnectionAction(connID)
			}
		}()
		go func() {
			for range revokeItem.ClickedCh {
				revokeConnectionCert(connID, connIdx)
			}
		}()

		// Update status for this connection
		go updateConnectionStatus(connIdx)

		if i < len(cfg.User.Connections)-1 {
			systray.AddSeparator()
		}
	}

	systray.AddSeparator()

	menuAddConn = systray.AddMenuItem("+ Add Connection...", "Add another GitHub connection")
	menuSettings := systray.AddMenuItem("Settings...", "Manage connections and settings")

	systray.AddSeparator()

	// Help submenu
	menuHelp := systray.AddMenuItem("Help", "Help and support options")
	menuHelpDocs := menuHelp.AddSubMenuItem("Documentation", "View cassh documentation")
	menuHelpBug := menuHelp.AddSubMenuItem("Report a Bug", "Report an issue on GitHub")
	menuHelpFeature := menuHelp.AddSubMenuItem("Request a Feature", "Suggest a new feature")

	// Community submenu
	menuCommunity := systray.AddMenuItem("Community", "Community and support")
	menuCommunityContribute := menuCommunity.AddSubMenuItem("Contribute", "Contribute to cassh on GitHub")
	menuCommunitySponsor := menuCommunity.AddSubMenuItem("Sponsor", "Support cassh development")
	menuCommunityShare := menuCommunity.AddSubMenuItem("Share cassh...", "Share cassh with friends")

	// Appearance submenu
	setupVisibilityMenu()

	systray.AddSeparator()

	menuUpdates := setupUpdateMenu()
	menuAbout := systray.AddMenuItem("About cassh", "About this application")
	menuVersion := systray.AddMenuItem(fmt.Sprintf("Version %s", version), "")
	menuVersion.Disable()

	systray.AddSeparator()

	menuUninstall := systray.AddMenuItem("Uninstall cassh...", "Remove cassh from your system")
	menuQuit = systray.AddMenuItem("Quit", "Quit cassh")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-menuAddConn.ClickedCh:
				openSetupWizard()
			case <-menuSettings.ClickedCh:
				openSetupWizard()
			case <-menuHelpDocs.ClickedCh:
				openBrowser("https://shawnschwartz.com/cassh")
			case <-menuHelpBug.ClickedCh:
				openBrowser("https://github.com/shawntz/cassh/issues/new?template=bug_report.md")
			case <-menuHelpFeature.ClickedCh:
				openBrowser("https://github.com/shawntz/cassh/issues/new?template=feature_request.md")
			case <-menuCommunityContribute.ClickedCh:
				openBrowser("https://github.com/shawntz/cassh")
			case <-menuCommunitySponsor.ClickedCh:
				openBrowser("https://github.com/sponsors/shawntz")
			case <-menuCommunityShare.ClickedCh:
				showShareDialog()
			case <-menuShowInDock.ClickedCh:
				handleShowInDockToggle()
			case <-menuUpdates.ClickedCh:
				handleUpdateMenuClick()
			case <-menuAbout.ClickedCh:
				showAbout()
			case <-menuUninstall.ClickedCh:
				uninstallCassh()
			case <-menuQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	// Check for updates in background
	go checkForUpdatesBackground()
}

// openSetupWizard opens the setup wizard in a native window
func openSetupWizard() {
	if runtime.GOOS == "darwin" {
		// Use native WebView window on macOS
		openSetupWindow()
	} else {
		// Fallback to browser on other platforms
		setupURL := fmt.Sprintf("http://localhost:%d/setup", loopbackPort)
		if err := openBrowser(setupURL); err != nil {
			log.Printf("Error opening setup wizard: %v", err)
		}
	}
}

// handleConnectionAction handles the action for a specific connection
func handleConnectionAction(connID string) {
	conn := cfg.User.GetConnection(connID)
	if conn == nil {
		log.Printf("Connection not found: %s", connID)
		return
	}

	if conn.Type == config.ConnectionTypeEnterprise {
		generateCertForConnection(conn)
	} else {
		refreshKeyForConnection(conn)
	}
}

// revokeConnectionCert revokes the certificate for a connection
func revokeConnectionCert(connID string, connIdx int) {
	conn := cfg.User.GetConnection(connID)
	if conn == nil {
		log.Printf("Connection not found: %s", connID)
		return
	}

	log.Printf("Revoking certificate for connection: %s", conn.Name)

	// Remove key from ssh-agent first
	if conn.SSHKeyPath != "" {
		if err := exec.Command("ssh-add", "-d", conn.SSHKeyPath).Run(); err != nil {
			log.Printf("Note: Could not remove key from ssh-agent: %v", err)
		} else {
			log.Printf("Removed key from ssh-agent: %s", conn.SSHKeyPath)
		}
	}

	// Remove certificate file
	if conn.Type == config.ConnectionTypeEnterprise && conn.SSHCertPath != "" {
		if err := os.Remove(conn.SSHCertPath); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Error removing certificate: %v", err)
			}
		} else {
			log.Printf("Removed certificate: %s", conn.SSHCertPath)
		}
	}

	// Update the menu status
	updateConnectionStatus(connIdx)

	log.Printf("Certificate revoked for connection: %s", conn.Name)

	// Send notification
	sendNotification("Certificate Revoked",
		fmt.Sprintf("%s certificate has been revoked.", conn.Name),
		false)
}

// generateCertForConnection opens WebView to generate cert for enterprise connection
func generateCertForConnection(conn *config.Connection) {
	// Ensure SSH key exists
	if err := ensureSSHKey(conn.SSHKeyPath); err != nil {
		log.Printf("Error ensuring SSH key: %v", err)
		return
	}

	// Read public key
	pubKeyPath := conn.SSHKeyPath + ".pub"
	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		log.Printf("Error reading public key: %v", err)
		return
	}

	// Build URL
	authURL := fmt.Sprintf("%s/?pubkey=%s",
		conn.ServerURL,
		url.QueryEscape(string(pubKeyData)),
	)

	// Open in native WebView on macOS, fallback to browser on other platforms
	if runtime.GOOS == "darwin" {
		openNativeWebView(authURL, fmt.Sprintf("Sign in - %s", conn.Name), 800, 700)
	} else {
		if err := openBrowser(authURL); err != nil {
			log.Printf("Error opening browser: %v", err)
		}
	}
}

// refreshKeyForConnection handles key refresh for personal GitHub connection
func refreshKeyForConnection(conn *config.Connection) {
	// Check if gh CLI is authenticated
	ghStatus := checkGHAuth()
	if !ghStatus.Installed {
		sendNotification("cassh", "GitHub CLI (gh) is not installed", false)
		return
	}
	if !ghStatus.Authenticated {
		sendNotification("cassh", "Please run 'gh auth login' first", false)
		return
	}

	// Rotate the key (delete old, generate new, upload new)
	if err := rotatePersonalGitHubSSH(conn); err != nil {
		log.Printf("Failed to rotate key: %v", err)
		sendNotification("cassh", fmt.Sprintf("Failed to rotate key: %v", err), false)
		return
	}

	// Save updated connection config with new key ID and timestamp
	if err := config.SaveUserConfig(&cfg.User); err != nil {
		log.Printf("Failed to save config after key rotation: %v", err)
	}

	sendNotification("cassh", fmt.Sprintf("SSH key rotated for %s", conn.Name), false)
	log.Printf("Rotated SSH key for: %s", conn.Name)
}

// updateConnectionStatus checks and updates the status for a specific connection
func updateConnectionStatus(connIdx int) {
	if connIdx >= len(cfg.User.Connections) {
		return
	}

	conn := cfg.User.Connections[connIdx]
	status := connectionStatus[conn.ID]
	if status == nil {
		status = &ConnectionStatus{ConnectionID: conn.ID}
		connectionStatus[conn.ID] = status
	}

	status.LastCheck = time.Now()

	if conn.Type == config.ConnectionTypeEnterprise {
		// Check certificate status
		certData, err := os.ReadFile(conn.SSHCertPath)
		if err != nil {
			setConnectionStatusInvalid(connIdx, "No certificate", false)
			return
		}

		cert, err := ca.ParseCertificate(certData)
		if err != nil {
			setConnectionStatusInvalid(connIdx, "Invalid certificate", false)
			return
		}

		info := ca.GetCertInfo(cert)
		if info.IsExpired {
			setConnectionStatusInvalid(connIdx, "Certificate expired", true)
			return
		}

		// Certificate is valid
		status.Valid = true
		status.TimeLeft = info.TimeLeft
		status.ValidBefore = info.ValidBefore

		hours := int(info.TimeLeft.Hours())
		mins := int(info.TimeLeft.Minutes()) % 60

		var statusText string
		if hours > 0 {
			statusText = fmt.Sprintf("🟢 %s (%dh %dm)", conn.Name, hours, mins)
		} else {
			statusText = fmt.Sprintf("🟡 %s (%dm)", conn.Name, mins)
		}

		if connIdx < len(menuConnections) {
			menuConnections[connIdx].SetTitle(statusText)
		}
		// Enable revoke button since cert is valid
		if connIdx < len(menuRevokeItems) {
			menuRevokeItems[connIdx].Enable()
		}
	} else {
		// Personal connection - check if key exists
		if _, err := os.Stat(conn.SSHKeyPath); err != nil {
			setConnectionStatusInvalid(connIdx, "No key configured", false)
			return
		}

		// Key exists - show as active with time until rotation
		status.Valid = true

		var statusText string
		if conn.KeyRotationHours > 0 && conn.KeyCreatedAt > 0 {
			// Calculate time until key rotation
			rotationDuration := time.Duration(conn.KeyRotationHours) * time.Hour
			keyAge := time.Since(time.Unix(conn.KeyCreatedAt, 0))
			timeUntilRotation := rotationDuration - keyAge

			status.TimeLeft = timeUntilRotation
			status.ValidBefore = time.Unix(conn.KeyCreatedAt, 0).Add(rotationDuration)

			if timeUntilRotation <= 0 {
				// Key rotation is due
				statusText = fmt.Sprintf("🟡 %s (@%s) - rotation due", conn.Name, conn.GitHubUsername)
			} else {
				hours := int(timeUntilRotation.Hours())
				mins := int(timeUntilRotation.Minutes()) % 60

				if hours >= 24 {
					days := hours / 24
					hours = hours % 24
					statusText = fmt.Sprintf("🟢 %s (@%s) %dd %dh", conn.Name, conn.GitHubUsername, days, hours)
				} else if hours > 0 {
					statusText = fmt.Sprintf("🟢 %s (@%s) %dh %dm", conn.Name, conn.GitHubUsername, hours, mins)
				} else {
					statusText = fmt.Sprintf("🟡 %s (@%s) %dm", conn.Name, conn.GitHubUsername, mins)
				}
			}
		} else {
			// No rotation policy or unknown creation time
			statusText = fmt.Sprintf("🟢 %s (@%s)", conn.Name, conn.GitHubUsername)
		}

		if connIdx < len(menuConnections) {
			menuConnections[connIdx].SetTitle(statusText)
		}
		// Enable revoke button since key is valid
		if connIdx < len(menuRevokeItems) {
			menuRevokeItems[connIdx].Enable()
		}
	}
}

// setConnectionStatusInvalid marks a connection as invalid in the menu
func setConnectionStatusInvalid(connIdx int, reason string, isExpired bool) {
	if connIdx >= len(cfg.User.Connections) || connIdx >= len(menuConnections) {
		return
	}

	conn := cfg.User.Connections[connIdx]
	status := connectionStatus[conn.ID]
	if status != nil {
		status.Valid = false
		status.TimeLeft = 0
	}

	menuConnections[connIdx].SetTitle(fmt.Sprintf("🔴 %s - %s", conn.Name, reason))

	// Disable revoke button since cert/key is not valid
	if connIdx < len(menuRevokeItems) {
		menuRevokeItems[connIdx].Disable()
	}
}

// monitorConnections periodically checks all connection statuses
func monitorConnections() {
	interval := time.Duration(cfg.User.RefreshIntervalSeconds) * time.Second
	if interval == 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		for i := range cfg.User.Connections {
			updateConnectionStatus(i)
		}
	}
}

func onExit() {
	log.Println("cassh-menubar exiting...")
}

// Legacy function removed - now using updateConnectionStatus and connection-based model

// ensureSSHKey creates the SSH key if it doesn't exist
func ensureSSHKey(keyPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return nil // Key exists
	}

	log.Println("Generating new SSH key...")

	// Ensure dir exists
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return fmt.Errorf("failed to create ssh directory: %w", err)
	}

	// Generate Ed25519 key
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	// Convert to SSH format
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("failed to convert public key: %w", err)
	}

	// Write private key (must encode the PEM block to get proper format)
	privPEM, err := ssh.MarshalPrivateKey(priv, "cassh generated key")
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(privPEM), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	pubData := ssh.MarshalAuthorizedKey(sshPub)
	if err := os.WriteFile(keyPath+".pub", pubData, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	log.Printf("Generated new SSH key at %s", keyPath)
	return nil
}

// ensureSSHConfig ensures the SSH config has the correct Host entry for GitHub Enterprise
func ensureSSHConfig(gheURL string, keyPath string) error {
	if gheURL == "" {
		return nil // No GHE URL configured
	}

	// Parse the GHE URL to get the hostname
	parsed, err := url.Parse(gheURL)
	if err != nil {
		return fmt.Errorf("failed to parse GHE URL: %w", err)
	}
	gheHost := parsed.Hostname()
	if gheHost == "" {
		return fmt.Errorf("invalid GHE URL: no hostname")
	}

	// SSH config path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	sshConfigPath := filepath.Join(homeDir, ".ssh", "config")

	// Ensure .ssh directory exists
	sshDir := filepath.Dir(sshConfigPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Check if config file exists and if it already has this host
	if _, err := os.Stat(sshConfigPath); err == nil {
		// File exists, check for existing host entry
		hasHost, err := sshConfigHasHost(sshConfigPath, gheHost)
		if err != nil {
			return fmt.Errorf("failed to check SSH config: %w", err)
		}
		if hasHost {
			log.Printf("SSH config already has entry for %s", gheHost)
			return nil
		}
	}

	// Append the host entry
	// IdentityAgent none bypasses 1Password and other SSH agent managers
	hostEntry := fmt.Sprintf(`
# Added by cassh for GitHub Enterprise SSH certificate auth
Host %s
    HostName %s
    User git
    IdentityFile %s
    IdentitiesOnly yes
    IdentityAgent none
`, gheHost, gheHost, keyPath)

	f, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(hostEntry); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}

	log.Printf("Added SSH config entry for %s", gheHost)
	return nil
}

// sshConfigHasHost checks if the SSH config already has a Host entry for the given hostname
func sshConfigHasHost(configPath string, hostname string) (bool, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Match "Host hostname" or "Host *hostname*" patterns
	hostPattern := regexp.MustCompile(`(?i)^\s*Host\s+.*\b` + regexp.QuoteMeta(hostname) + `\b`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if hostPattern.MatchString(line) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// sshConfigHasCorrectKey checks if the SSH config for a hostname has the correct IdentityFile
func sshConfigHasCorrectKey(configPath string, hostname string, expectedKeyPath string) (bool, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	hostPattern := regexp.MustCompile(`(?i)^\s*Host\s+.*\b` + regexp.QuoteMeta(hostname) + `\b`)
	identityPattern := regexp.MustCompile(`(?i)^\s*IdentityFile\s+(.+)`)

	inHostBlock := false
	for _, line := range lines {
		if hostPattern.MatchString(line) {
			inHostBlock = true
			continue
		}
		// Check if we've entered a new Host block
		if inHostBlock && regexp.MustCompile(`(?i)^\s*Host\s+`).MatchString(line) {
			break // Moved to another host block
		}
		if inHostBlock {
			if matches := identityPattern.FindStringSubmatch(line); len(matches) > 1 {
				currentKeyPath := strings.TrimSpace(matches[1])
				return currentKeyPath == expectedKeyPath, nil
			}
		}
	}

	return false, nil // No IdentityFile found
}

// removeSSHConfigHost removes a Host entry from the SSH config
func removeSSHConfigHost(configPath string, hostname string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	hostPattern := regexp.MustCompile(`(?i)^\s*Host\s+.*\b` + regexp.QuoteMeta(hostname) + `\b`)
	casshCommentPattern := regexp.MustCompile(`(?i)^\s*#\s*Added by cassh`)

	var newLines []string
	skipBlock := false
	skipComment := false

	for i, line := range lines {
		// Check if this is a cassh comment right before a Host block
		if casshCommentPattern.MatchString(line) {
			// Look ahead to see if next non-empty line is the Host we're removing
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "" {
					continue
				}
				if hostPattern.MatchString(nextLine) {
					skipComment = true
				}
				break
			}
		}

		if skipComment {
			skipComment = false
			continue // Skip the cassh comment
		}

		if hostPattern.MatchString(line) {
			skipBlock = true
			continue
		}

		// Check if we've entered a new Host block or hit an empty line after options
		if skipBlock {
			trimmed := strings.TrimSpace(line)
			// If line starts with Host (new block) or is empty after we've seen some content
			if regexp.MustCompile(`(?i)^\s*Host\s+`).MatchString(line) {
				skipBlock = false
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t") {
				// Non-indented, non-comment, non-empty line means new section
				skipBlock = false
			} else {
				continue // Still in the block we're removing
			}
		}

		newLines = append(newLines, line)
	}

	// Remove trailing empty lines and write back
	result := strings.TrimRight(strings.Join(newLines, "\n"), "\n") + "\n"
	return os.WriteFile(configPath, []byte(result), 0600)
}

// ensureSSHConfigForConnection adds or updates SSH config for a connection
// Supports both enterprise (certificate) and personal (key-only) connections
func ensureSSHConfigForConnection(conn *config.Connection) error {
	if conn.GitHubHost == "" {
		return nil // No host configured
	}

	// SSH config path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	sshConfigPath := filepath.Join(homeDir, ".ssh", "config")

	// Ensure .ssh directory exists
	sshDir := filepath.Dir(sshConfigPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Build the expected host entry based on connection type
	var hostEntry string

	// Determine SSH user - for enterprise, use the SCIM-provisioned username from clone URL
	// For personal/github.com, always use "git"
	sshUser := "git"
	if conn.Type == config.ConnectionTypeEnterprise && conn.GitHubUsername != "" {
		sshUser = conn.GitHubUsername
	}

	if conn.Type == config.ConnectionTypeEnterprise {
		// Enterprise: use certificate auth
		hostEntry = fmt.Sprintf(`
# Added by cassh for %s (enterprise certificate auth)
Host %s
    HostName %s
    User %s
    IdentityFile %s
    CertificateFile %s
    IdentitiesOnly yes
    IdentityAgent none
`, conn.Name, conn.GitHubHost, conn.GitHubHost, sshUser, conn.SSHKeyPath, conn.SSHCertPath)
	} else {
		// Personal: use key-only auth (always User git for github.com)
		hostEntry = fmt.Sprintf(`
# Added by cassh for %s (personal key auth)
Host %s
    HostName %s
    User git
    IdentityFile %s
    IdentitiesOnly yes
    IdentityAgent none
`, conn.Name, conn.GitHubHost, conn.GitHubHost, conn.SSHKeyPath)
	}

	// Check if config file exists and if it already has this host
	if _, err := os.Stat(sshConfigPath); err == nil {
		hasHost, err := sshConfigHasHost(sshConfigPath, conn.GitHubHost)
		if err != nil {
			return fmt.Errorf("failed to check SSH config: %w", err)
		}
		if hasHost {
			// Check if the key path is correct
			hasCorrectKey, err := sshConfigHasCorrectKey(sshConfigPath, conn.GitHubHost, conn.SSHKeyPath)
			if err != nil {
				return fmt.Errorf("failed to check SSH config key path: %w", err)
			}
			if hasCorrectKey {
				log.Printf("SSH config already has correct entry for %s", conn.GitHubHost)
				return nil
			}
			// Key path is wrong, remove old entry and add new one
			log.Printf("SSH config has outdated entry for %s, updating...", conn.GitHubHost)
			if err := removeSSHConfigHost(sshConfigPath, conn.GitHubHost); err != nil {
				return fmt.Errorf("failed to remove old SSH config entry: %w", err)
			}
		}
	}

	// Append the new host entry
	f, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open SSH config: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(hostEntry); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}

	log.Printf("Added SSH config entry for %s (%s)", conn.GitHubHost, conn.Type)
	return nil
}

// ensureGitConfigForConnection sets up git configuration for a connection
// Uses includeIf to apply different user.name/email based on remote URL
func ensureGitConfigForConnection(conn *config.Connection, userName, userEmail string) error {
	if conn.GitHubHost == "" || (userName == "" && userEmail == "") {
		return nil // No host or no git identity to configure
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	// Create cassh config directory
	casshConfigDir := filepath.Join(homeDir, ".config", "cassh")
	if err := os.MkdirAll(casshConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create cassh config dir: %w", err)
	}

	// Create per-connection gitconfig file
	connGitConfigPath := filepath.Join(casshConfigDir, fmt.Sprintf("gitconfig-%s", conn.ID))
	var gitConfigContent strings.Builder
	gitConfigContent.WriteString(fmt.Sprintf("# Git config for %s (%s)\n", conn.Name, conn.GitHubHost))
	gitConfigContent.WriteString("# Managed by cassh - do not edit manually\n")
	gitConfigContent.WriteString("[user]\n")
	if userName != "" {
		gitConfigContent.WriteString(fmt.Sprintf("    name = %s\n", userName))
	}
	if userEmail != "" {
		gitConfigContent.WriteString(fmt.Sprintf("    email = %s\n", userEmail))
	}

	if err := os.WriteFile(connGitConfigPath, []byte(gitConfigContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write connection gitconfig: %w", err)
	}

	// Add includeIf to ~/.gitconfig for this host
	gitConfigPath := filepath.Join(homeDir, ".gitconfig")
	includeDirective := fmt.Sprintf(`
# cassh: Include config for %s
[includeIf "hasconfig:remote.*.url:%s@%s:**"]
    path = %s
[includeIf "hasconfig:remote.*.url:ssh://%s@%s/**"]
    path = %s
`, conn.Name, conn.GitHubUsername, conn.GitHubHost, connGitConfigPath,
		conn.GitHubUsername, conn.GitHubHost, connGitConfigPath)

	// For personal github.com, use git@github.com pattern
	if conn.Type == config.ConnectionTypePersonal {
		includeDirective = fmt.Sprintf(`
# cassh: Include config for %s
[includeIf "hasconfig:remote.*.url:git@%s:**"]
    path = %s
[includeIf "hasconfig:remote.*.url:ssh://git@%s/**"]
    path = %s
`, conn.Name, conn.GitHubHost, connGitConfigPath, conn.GitHubHost, connGitConfigPath)
	}

	// Check if gitconfig exists and if it already has this include
	if _, err := os.Stat(gitConfigPath); err == nil {
		content, err := os.ReadFile(gitConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read gitconfig: %w", err)
		}
		// Check if we already have an includeIf for this connection
		if strings.Contains(string(content), fmt.Sprintf("cassh: Include config for %s", conn.Name)) {
			log.Printf("Git config already has includeIf for %s", conn.Name)
			// Update the per-connection file anyway in case identity changed
			return nil
		}
	}

	// Append includeIf to gitconfig
	f, err := os.OpenFile(gitConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open gitconfig: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(includeDirective); err != nil {
		return fmt.Errorf("failed to write gitconfig: %w", err)
	}

	log.Printf("Added git config for %s (%s)", conn.GitHubHost, conn.Name)
	return nil
}

// removeGitConfigForConnection removes the git configuration for a connection
func removeGitConfigForConnection(conn *config.Connection) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	// Remove the per-connection gitconfig file
	casshConfigDir := filepath.Join(homeDir, ".config", "cassh")
	connGitConfigPath := filepath.Join(casshConfigDir, fmt.Sprintf("gitconfig-%s", conn.ID))
	if err := os.Remove(connGitConfigPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove connection gitconfig: %v", err)
	}

	// Remove includeIf from ~/.gitconfig
	gitConfigPath := filepath.Join(homeDir, ".gitconfig")
	if _, err := os.Stat(gitConfigPath); err != nil {
		return nil // No gitconfig, nothing to remove
	}

	content, err := os.ReadFile(gitConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read gitconfig: %w", err)
	}

	// Remove the cassh section for this connection
	lines := strings.Split(string(content), "\n")
	var newLines []string
	skipUntilBlank := false
	marker := fmt.Sprintf("# cassh: Include config for %s", conn.Name)

	for _, line := range lines {
		if strings.Contains(line, marker) {
			skipUntilBlank = true
			continue
		}
		if skipUntilBlank {
			// Skip includeIf lines until we hit a blank line or new section
			if strings.TrimSpace(line) == "" || (strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[includeIf")) {
				skipUntilBlank = false
				newLines = append(newLines, line)
			}
			continue
		}
		newLines = append(newLines, line)
	}

	if err := os.WriteFile(gitConfigPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write gitconfig: %w", err)
	}

	log.Printf("Removed git config for %s", conn.Name)
	return nil
}

func openBrowser(urlStr string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "linux":
		cmd = exec.Command("xdg-open", urlStr)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// sendNotification sends a macOS notification with the app's icon
// If actionOnClick is true, the notification will have a "Renew Now" action button
func sendNotification(title, message string, actionOnClick bool) {
	if runtime.GOOS != "darwin" {
		return
	}

	// Use native UserNotifications framework with proper app icon
	// Category determines the action buttons shown
	if actionOnClick {
		// Use CERT_EXPIRING category which has "Renew Now" and "Dismiss" actions
		sendNotificationWithCategory(title, message, "CERT_EXPIRING")
	} else {
		// General notification without actions
		sendNativeNotification(title, message)
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%d days", days)
	}

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%d hours", hours)
	}

	return fmt.Sprintf("%d minutes", minutes)
}

// uninstallCassh removes cassh and all its data from the system
func uninstallCassh() {
	// Show confirmation dialog
	if !showUninstallConfirmation() {
		return
	}

	homeDir, _ := os.UserHomeDir()

	// 1. Delete SSH keys created by cassh for each connection
	for _, conn := range cfg.User.Connections {
		// Delete from GitHub if personal account
		if conn.Type == config.ConnectionTypePersonal && conn.GitHubKeyID != "" {
			if err := deleteSSHKeyFromGitHub(conn.GitHubKeyID); err != nil {
				log.Printf("Warning: Could not delete GitHub key: %v", err)
			}
		}

		// Delete local key files
		if conn.SSHKeyPath != "" {
			os.Remove(conn.SSHKeyPath)
			os.Remove(conn.SSHKeyPath + ".pub")
		}
		if conn.SSHCertPath != "" {
			os.Remove(conn.SSHCertPath)
		}

		// Remove git config for this connection
		removeGitConfigForConnection(&conn)
	}

	// 2. Unregister from login items (SMAppService) and remove LaunchAgents
	unregisterAsLoginItem()
	// Remove user-level LaunchAgents
	userLaunchAgentPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.shawnschwartz.cassh.plist")
	exec.Command("launchctl", "unload", userLaunchAgentPath).Run()
	os.Remove(userLaunchAgentPath)
	userRotateAgentPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.shawnschwartz.cassh.rotate.plist")
	exec.Command("launchctl", "unload", userRotateAgentPath).Run()
	os.Remove(userRotateAgentPath)
	// Remove system-level LaunchAgents (installed by PKG)
	systemLaunchAgentPath := "/Library/LaunchAgents/com.shawnschwartz.cassh.plist"
	exec.Command("launchctl", "unload", systemLaunchAgentPath).Run()
	systemRotateAgentPath := "/Library/LaunchAgents/com.shawnschwartz.cassh.rotate.plist"
	exec.Command("launchctl", "unload", systemRotateAgentPath).Run()
	// System LaunchAgents require admin to remove - will be handled by the uninstall script

	// 3. Remove Application Support directory (contains user config)
	appSupportDir := filepath.Join(homeDir, "Library", "Application Support", "cassh")
	if err := os.RemoveAll(appSupportDir); err != nil {
		log.Printf("Warning: Could not remove Application Support directory: %v", err)
	}

	// 4. Remove cassh config directory (contains git configs)
	casshConfigDir := filepath.Join(homeDir, ".config", "cassh")
	if err := os.RemoveAll(casshConfigDir); err != nil {
		log.Printf("Warning: Could not remove cassh config directory: %v", err)
	}

	// 5. Remove preferences
	prefsPath := filepath.Join(homeDir, "Library", "Preferences", "com.shawnschwartz.cassh.plist")
	os.Remove(prefsPath)

	// 6. Get current app path
	execPath, _ := os.Executable()
	appPath := ""

	// If running from .app bundle, get the .app path
	if idx := strings.Index(execPath, ".app/"); idx != -1 {
		appPath = execPath[:idx+4]
	}

	// 7. Create script to delete the app after we quit
	// Use osascript with admin privileges if the app is in /Applications
	// Also remove the system-level LaunchAgent installed by PKG
	var uninstallScript string
	if strings.HasPrefix(appPath, "/Applications") {
		// Need admin privileges to delete from /Applications and system LaunchAgent
		// Use a separate AppleScript file to avoid escaping issues
		uninstallScript = fmt.Sprintf(`#!/bin/bash
sleep 2
APP_PATH='%s'
LAUNCH_AGENT='/Library/LaunchAgents/com.shawnschwartz.cassh.plist'

# Create AppleScript to run with admin privileges
cat > /tmp/cassh_uninstall.scpt << 'APPLESCRIPT'
do shell script "rm -rf '/Applications/cassh.app' '/Library/LaunchAgents/com.shawnschwartz.cassh.plist' '/Library/LaunchAgents/com.shawnschwartz.cassh.rotate.plist' 2>/dev/null || true" with prompt "cassh needs to remove the application." with administrator privileges
APPLESCRIPT

if osascript /tmp/cassh_uninstall.scpt 2>/dev/null; then
    osascript -e 'display notification "cassh has been uninstalled successfully." with title "Uninstall Complete"'
else
    # Try without admin as fallback (in case app was moved)
    rm -rf "$APP_PATH" 2>/dev/null
    osascript -e 'display notification "cassh uninstall may be incomplete. Check /Applications manually." with title "Uninstall"'
fi

rm -f /tmp/cassh_uninstall.scpt
rm -f "$0"
`, appPath)
	} else if appPath != "" {
		// Can delete app without admin privileges, but still try to remove system LaunchAgents
		uninstallScript = fmt.Sprintf(`#!/bin/bash
sleep 2
rm -rf '%s'
# Try to remove system LaunchAgents (may fail without admin)
rm -f /Library/LaunchAgents/com.shawnschwartz.cassh.plist 2>/dev/null || true
rm -f /Library/LaunchAgents/com.shawnschwartz.cassh.rotate.plist 2>/dev/null || true
osascript -e 'display notification "cassh has been uninstalled" with title "Uninstall Complete"'
rm -f "$0"
`, appPath)
	} else {
		// Just show notification, no app to delete
		uninstallScript = `#!/bin/bash
sleep 2
osascript -e 'display notification "cassh data has been removed" with title "Uninstall Complete"'
rm -f "$0"
`
	}

	scriptPath := filepath.Join(os.TempDir(), "cassh_uninstall.sh")
	if err := os.WriteFile(scriptPath, []byte(uninstallScript), 0755); err != nil {
		log.Printf("Warning: Could not create uninstall script: %v", err)
	}

	// Run the uninstall script in background and detach it
	cmd := exec.Command("bash", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Warning: Could not start uninstall script: %v", err)
	}

	// Quit the app
	log.Println("Uninstalling cassh...")
	systray.Quit()
}

// installLaunchAgent installs a LaunchAgent to start cassh on login
func installLaunchAgent() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: Could not get home directory: %v", err)
		return
	}

	launchAgentDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	launchAgentPath := filepath.Join(launchAgentDir, "com.shawnschwartz.cassh.plist")

	// Check if already installed
	if _, err := os.Stat(launchAgentPath); err == nil {
		return // Already installed
	}

	// Create LaunchAgents directory if needed
	if err := os.MkdirAll(launchAgentDir, 0755); err != nil {
		log.Printf("Warning: Could not create LaunchAgents directory: %v", err)
		return
	}

	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: Could not get executable path: %v", err)
		return
	}

	// Create LaunchAgent plist
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.shawnschwartz.cassh</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, execPath)

	if err := os.WriteFile(launchAgentPath, []byte(plist), 0644); err != nil {
		log.Printf("Warning: Could not write LaunchAgent: %v", err)
		return
	}

	// Load the LaunchAgent
	cmd := exec.Command("launchctl", "load", launchAgentPath)
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: Could not load LaunchAgent: %v", err)
	}

	log.Println("Installed LaunchAgent for auto-start on login")
}

// Legacy monitorCertificate removed - now using monitorConnections

// startLoopbackListener starts the local HTTP server for auto-install and setup wizard
func startLoopbackListener() {
	mux := http.NewServeMux()

	// Static files (images, etc.)
	staticHandler := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", staticHandler)

	// Setup wizard endpoints
	mux.HandleFunc("/setup", handleSetup)
	mux.HandleFunc("/setup/add-enterprise", handleAddEnterprise)
	mux.HandleFunc("/setup/add-personal", handleAddPersonal)
	mux.HandleFunc("/setup/delete-connection", handleDeleteConnection)
	mux.HandleFunc("/setup/gh-status", handleGHStatus)

	// Certificate/key management endpoints
	mux.HandleFunc("/install-cert", handleInstallCert)
	mux.HandleFunc("/status", handleStatus)

	addr := fmt.Sprintf("127.0.0.1:%d", loopbackPort)
	log.Printf("Loopback listener on %s", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Printf("Loopback listener error: %v", err)
	}
}

// handleInstallCert receives the cert from browser (for enterprise connections)
func handleInstallCert(w http.ResponseWriter, r *http.Request) {
	log.Printf("handleInstallCert: received %s request from %s", r.Method, r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("handleInstallCert: rejecting non-POST method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Cert         string `json:"cert"`
		ConnectionID string `json:"connection_id,omitempty"` // Optional: specific connection
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("handleInstallCert: failed to decode request body: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	log.Printf("handleInstallCert: received cert (%d bytes), connection_id=%q", len(req.Cert), req.ConnectionID)

	// Validate cert
	if _, err := ca.ParseCertificate([]byte(req.Cert)); err != nil {
		log.Printf("handleInstallCert: invalid certificate: %v", err)
		http.Error(w, "Invalid certificate", http.StatusBadRequest)
		return
	}

	// Find the connection to install cert for
	var conn *config.Connection
	if req.ConnectionID != "" {
		conn = cfg.User.GetConnection(req.ConnectionID)
	} else if len(cfg.User.Connections) > 0 {
		// Default to first enterprise connection
		for i := range cfg.User.Connections {
			if cfg.User.Connections[i].Type == config.ConnectionTypeEnterprise {
				conn = &cfg.User.Connections[i]
				break
			}
		}
	}

	// Fallback to legacy paths if no connection found
	var certPath, keyPath string
	var gheURL string
	if conn != nil {
		certPath = conn.SSHCertPath
		keyPath = conn.SSHKeyPath
		gheURL = "https://" + conn.GitHubHost
	} else {
		// Legacy fallback
		certPath = cfg.User.SSHCertPath
		keyPath = cfg.User.SSHKeyPath
		gheURL = cfg.Policy.GitHubEnterpriseURL
	}

	// Write cert
	if err := os.WriteFile(certPath, []byte(req.Cert), 0644); err != nil {
		log.Printf("Failed to write cert: %v", err)
		http.Error(w, "Failed to save certificate", http.StatusInternalServerError)
		return
	}

	// Add to ssh-agent
	if err := exec.Command("ssh-add", keyPath).Run(); err != nil {
		log.Printf("Warning: ssh-add failed: %v", err)
	}

	// Ensure SSH config has the correct Host entry for GHE
	log.Printf("GitHubEnterpriseURL: %q", gheURL)
	if err := ensureSSHConfig(gheURL, keyPath); err != nil {
		log.Printf("Warning: failed to configure SSH config: %v", err)
	}

	log.Println("Certificate installed successfully")

	// Parse cert to get expiration info
	parsedCert, _ := ca.ParseCertificate([]byte(req.Cert))
	certInfo := ca.GetCertInfo(parsedCert)

	// Send activation notification with time remaining
	connName := "GitHub Enterprise"
	if conn != nil {
		connName = conn.Name
	}
	timeRemaining := formatDuration(certInfo.TimeLeft)
	sendNotification("Certificate Activated",
		fmt.Sprintf("%s is now active. Valid for %s.", connName, timeRemaining),
		false)

	// Update connection status
	if conn != nil {
		for i, c := range cfg.User.Connections {
			if c.ID == conn.ID {
				go updateConnectionStatus(i)
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleStatus returns current status for all connections
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	statuses := make([]map[string]interface{}, 0)
	for _, conn := range cfg.User.Connections {
		status := connectionStatus[conn.ID]

		// Determine github_host based on connection type
		githubHost := conn.GitHubHost
		if githubHost == "" {
			if conn.Type == "personal" {
				githubHost = "github.com"
			} else if conn.ServerURL != "" {
				githubHost = config.ExtractHostFromURL(conn.ServerURL)
			}
		}

		s := map[string]interface{}{
			"id":          conn.ID,
			"name":        conn.Name,
			"type":        conn.Type,
			"github_host": githubHost,
			"is_valid":    false,
			"time_left":   "",
		}
		if status != nil {
			s["is_valid"] = status.Valid
			s["time_left"] = status.TimeLeft.String()
			s["expires_at"] = status.ValidBefore
			s["last_check"] = status.LastCheck
		}
		statuses = append(statuses, s)
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"connections": statuses,
		"needs_setup": needsSetup,
	})
}

// handleSetup serves the setup wizard page
func handleSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		HasConnections bool
		Connections    []config.Connection
	}{
		HasConnections: cfg.User.HasConnections(),
		Connections:    cfg.User.Connections,
	}

	if templates != nil {
		if err := templates.ExecuteTemplate(w, "setup.html", data); err != nil {
			log.Printf("Template error: %v", err)
			// Fallback to basic HTML
			fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>cassh Setup</title></head><body>
				<h1>cassh Setup</h1>
				<p>Template error: %v</p>
				<p><a href="/setup/add-enterprise">Add GitHub Enterprise</a></p>
				<p><a href="/setup/add-personal">Add GitHub.com Personal</a></p>
			</body></html>`, err)
		}
	} else {
		// Basic HTML fallback
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>cassh Setup</title></head><body>
			<h1>cassh Setup</h1>
			<p><a href="/setup/add-enterprise">Add GitHub Enterprise</a></p>
			<p><a href="/setup/add-personal">Add GitHub.com Personal</a></p>
		</body></html>`)
	}
}

// handleAddEnterprise handles adding a GitHub Enterprise connection
func handleAddEnterprise(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Show form
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if templates != nil {
			if err := templates.ExecuteTemplate(w, "add-enterprise.html", nil); err != nil {
				log.Printf("Template error: %v", err)
				serveEnterpriseFormFallback(w)
			}
		} else {
			serveEnterpriseFormFallback(w)
		}
		return
	}

	if r.Method == http.MethodPost {
		// Parse JSON request
		var req struct {
			Name           string `json:"name"`
			ServerURL      string `json:"server_url"`
			GitHubHost     string `json:"github_host"`
			GitHubUsername string `json:"github_username"`
			GitName        string `json:"git_name"`
			GitEmail       string `json:"git_email"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON: " + err.Error()})
			return
		}

		if req.Name == "" {
			req.Name = "GitHub Enterprise"
		}

		if req.ServerURL == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Server URL is required"})
			return
		}

		if req.GitHubHost == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "GitHub Enterprise URL is required"})
			return
		}

		if req.GitHubUsername == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "GitHub SSH username is required (from SSH clone URL)"})
			return
		}

		// Create connection
		homeDir, _ := os.UserHomeDir()
		connID := fmt.Sprintf("enterprise-%d", time.Now().Unix())

		conn := config.Connection{
			ID:             connID,
			Type:           config.ConnectionTypeEnterprise,
			Name:           req.Name,
			ServerURL:      req.ServerURL,
			GitHubHost:     config.ExtractHostFromURL(req.GitHubHost),
			GitHubUsername: req.GitHubUsername,
			SSHKeyPath:     filepath.Join(homeDir, ".ssh", fmt.Sprintf("cassh_%s_id_ed25519", connID)),
			SSHCertPath:    filepath.Join(homeDir, ".ssh", fmt.Sprintf("cassh_%s_id_ed25519-cert.pub", connID)),
		}

		// Add connection to config
		cfg.User.AddConnection(conn)
		if err := config.SaveUserConfig(&cfg.User); err != nil {
			log.Printf("Failed to save config: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
			return
		}

		// Initialize status tracking
		connectionStatus[conn.ID] = &ConnectionStatus{ConnectionID: conn.ID}

		// Update needs setup flag
		needsSetup = false

		// Ensure SSH config is set up for this connection
		if err := ensureSSHConfigForConnection(&conn); err != nil {
			log.Printf("Warning: failed to update SSH config: %v", err)
		}

		// Set up git config for this connection (if git identity provided)
		if req.GitName != "" || req.GitEmail != "" {
			if err := ensureGitConfigForConnection(&conn, req.GitName, req.GitEmail); err != nil {
				log.Printf("Warning: failed to set up git config: %v", err)
			}
		}

		log.Printf("Added enterprise connection: %s (%s -> %s)", conn.Name, conn.ServerURL, conn.GitHubHost)

		// Send notification for new connection
		sendNotification("Connection Added",
			fmt.Sprintf("%s has been configured. Click the menu bar icon to generate your first certificate.", conn.Name),
			false)

		// Return success with restart flag
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       true,
			"connection":    conn,
			"needs_restart": true,
		})

		// Schedule app restart to rebuild menu
		go scheduleRestart()
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func serveEnterpriseFormFallback(w http.ResponseWriter) {
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Add GitHub Enterprise</title></head><body>
		<h1>Add GitHub Enterprise Connection</h1>
		<form method="POST">
			<p><label>Connection Name: <input type="text" name="name" placeholder="GitHub Enterprise"></label></p>
			<p><label>Server URL: <input type="url" name="server_url" placeholder="https://cassh.yourcompany.com" required></label></p>
			<p><button type="submit">Add Connection</button></p>
		</form>
	</body></html>`)
}

// handleAddPersonal handles adding a GitHub.com personal connection
func handleAddPersonal(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Check if gh CLI is installed
		ghInstalled := false
		if _, err := exec.LookPath("gh"); err == nil {
			ghInstalled = true
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if templates != nil {
			data := struct{ GHInstalled bool }{GHInstalled: ghInstalled}
			if err := templates.ExecuteTemplate(w, "add-personal.html", data); err != nil {
				log.Printf("Template error: %v", err)
				servePersonalFormFallback(w, ghInstalled)
			}
		} else {
			servePersonalFormFallback(w, ghInstalled)
		}
		return
	}

	if r.Method == http.MethodPost {
		// Parse JSON request
		var req struct {
			Name             string `json:"name"`
			GitHubUsername   string `json:"github_username"`
			KeyRotationHours int    `json:"key_rotation_hours"`
			GitName          string `json:"git_name"`
			GitEmail         string `json:"git_email"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON: " + err.Error()})
			return
		}

		if req.Name == "" {
			req.Name = "GitHub.com"
		}

		// Default to 12 hours if not specified
		if req.KeyRotationHours == 0 {
			req.KeyRotationHours = 12
		}

		if req.GitHubUsername == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "GitHub username is required"})
			return
		}

		// Check gh CLI status
		ghStatus := checkGHAuth()
		if !ghStatus.Installed {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"error": "GitHub CLI (gh) is not installed. Please install it: brew install gh",
			})
			return
		}

		if !ghStatus.Authenticated {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Please authenticate with GitHub CLI first: gh auth login",
			})
			return
		}

		// Create connection
		homeDir, _ := os.UserHomeDir()
		connID := fmt.Sprintf("personal-%d", time.Now().Unix())

		conn := config.Connection{
			ID:               connID,
			Type:             config.ConnectionTypePersonal,
			Name:             req.Name,
			GitHubHost:       "github.com",
			GitHubUsername:   req.GitHubUsername,
			SSHKeyPath:       filepath.Join(homeDir, ".ssh", fmt.Sprintf("cassh_%s_id_ed25519", connID)),
			KeyRotationHours: req.KeyRotationHours,
			// No cert path for personal accounts (key-based auth)
		}

		// Generate SSH key and upload to GitHub
		if err := setupPersonalGitHubSSH(&conn); err != nil {
			log.Printf("Failed to setup SSH key: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to setup SSH key: " + err.Error()})
			return
		}

		// Add connection to config
		cfg.User.AddConnection(conn)
		if err := config.SaveUserConfig(&cfg.User); err != nil {
			log.Printf("Failed to save config: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
			return
		}

		// Initialize status tracking
		connectionStatus[conn.ID] = &ConnectionStatus{ConnectionID: conn.ID, Valid: true}

		// Update needs setup flag
		needsSetup = false

		// Ensure SSH config is set up for this connection
		if err := ensureSSHConfigForConnection(&conn); err != nil {
			log.Printf("Warning: failed to update SSH config: %v", err)
		}

		// Set up git config for this connection (if git identity provided)
		if req.GitName != "" || req.GitEmail != "" {
			if err := ensureGitConfigForConnection(&conn, req.GitName, req.GitEmail); err != nil {
				log.Printf("Warning: failed to set up git config: %v", err)
			}
		}

		log.Printf("Added personal connection: %s (@%s)", conn.Name, conn.GitHubUsername)

		// Send notification for new connection
		sendNotification("Connection Added",
			fmt.Sprintf("%s is ready to use. SSH key uploaded to GitHub.", conn.Name),
			false)

		// Return success with restart flag
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       true,
			"connection":    conn,
			"message":       "SSH key generated and uploaded to GitHub!",
			"needs_restart": true,
		})

		// Schedule app restart to rebuild menu
		go scheduleRestart()
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleDeleteConnection removes a connection from the configuration
func handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	log.Printf("handleDeleteConnection called: method=%s", r.Method)

	if r.Method == http.MethodPost || r.Method == http.MethodDelete {
		var req struct {
			ID string `json:"id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("handleDeleteConnection: decode error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
			return
		}

		log.Printf("handleDeleteConnection: request ID=%s", req.ID)

		if req.ID == "" {
			log.Printf("handleDeleteConnection: empty ID")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Connection ID is required"})
			return
		}

		// Find and remove the connection
		var removedConn *config.Connection
		newConnections := make([]config.Connection, 0)
		for _, conn := range cfg.User.Connections {
			if conn.ID == req.ID {
				removedConn = &conn
			} else {
				newConnections = append(newConnections, conn)
			}
		}

		if removedConn == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Connection not found"})
			return
		}

		// For personal connections, try to delete the key from GitHub
		if removedConn.Type == config.ConnectionTypePersonal && removedConn.GitHubKeyID != "" {
			if err := deleteSSHKeyFromGitHub(removedConn.GitHubKeyID); err != nil {
				log.Printf("Warning: failed to delete SSH key from GitHub: %v", err)
			}
		}

		// Delete local SSH key files
		if removedConn.SSHKeyPath != "" {
			os.Remove(removedConn.SSHKeyPath)
			os.Remove(removedConn.SSHKeyPath + ".pub")
		}

		// Delete certificate if exists
		if removedConn.SSHCertPath != "" {
			os.Remove(removedConn.SSHCertPath)
		}

		// Remove git config for this connection
		if err := removeGitConfigForConnection(removedConn); err != nil {
			log.Printf("Warning: failed to remove git config: %v", err)
		}

		// Update config
		cfg.User.Connections = newConnections
		if err := config.SaveUserConfig(&cfg.User); err != nil {
			log.Printf("Failed to save config: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save configuration"})
			return
		}

		// Remove from status tracking
		delete(connectionStatus, req.ID)

		// Update needs setup flag
		needsSetup = len(cfg.User.Connections) == 0

		log.Printf("Deleted connection: %s (%s)", removedConn.Name, removedConn.ID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Connection removed successfully",
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleGHStatus returns the GitHub CLI authentication status
func handleGHStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := checkGHAuth()
	json.NewEncoder(w).Encode(status)
}

func servePersonalFormFallback(w http.ResponseWriter, ghInstalled bool) {
	ghStatus := "not installed"
	if ghInstalled {
		ghStatus = "installed"
	}
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Add GitHub.com Personal</title></head><body>
		<h1>Add GitHub.com Personal Account</h1>
		<p>GitHub CLI status: %s</p>
		<p>This feature is coming soon!</p>
		<p><a href="/setup">Back to Setup</a></p>
	</body></html>`, ghStatus)
}

// =============================================================================
// GitHub CLI (gh) Helper Functions
// =============================================================================

// GHAuthStatus represents the authentication status from gh CLI
type GHAuthStatus struct {
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
	Scopes        string `json:"scopes"`
	Error         string `json:"error,omitempty"`
}

// ghPath stores the cached path to the gh binary
var ghPath string

// findGHBinary finds the gh binary, checking common Homebrew paths
// This is necessary because GUI apps don't inherit the user's shell PATH
func findGHBinary() string {
	if ghPath != "" {
		return ghPath
	}

	// First try the system PATH
	if path, err := exec.LookPath("gh"); err == nil {
		ghPath = path
		return ghPath
	}

	// Check common Homebrew locations
	commonPaths := []string{
		"/opt/homebrew/bin/gh",              // Apple Silicon
		"/usr/local/bin/gh",                 // Intel Mac
		"/home/linuxbrew/.linuxbrew/bin/gh", // Linux Homebrew
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			ghPath = path
			return ghPath
		}
	}

	return ""
}

// checkGHInstalled checks if the GitHub CLI is installed
func checkGHInstalled() bool {
	return findGHBinary() != ""
}

// checkGHAuth checks the authentication status of gh CLI
func checkGHAuth() GHAuthStatus {
	status := GHAuthStatus{}

	if !checkGHInstalled() {
		status.Error = "GitHub CLI (gh) is not installed"
		return status
	}
	status.Installed = true

	// Run gh auth status
	cmd := exec.Command(findGHBinary(), "auth", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		status.Error = "Not authenticated with GitHub CLI"
		return status
	}

	status.Authenticated = true

	// Parse output to extract username
	outputStr := string(output)
	// Look for "Logged in to github.com account <username>"
	usernameRegex := regexp.MustCompile(`Logged in to github\.com.*account\s+(\S+)`)
	if matches := usernameRegex.FindStringSubmatch(outputStr); len(matches) > 1 {
		status.Username = matches[1]
	}

	return status
}

// generateSSHKeyForPersonal generates an SSH key pair for a personal GitHub account
func generateSSHKeyForPersonal(conn *config.Connection) error {
	keyPath := conn.SSHKeyPath

	// Ensure .ssh directory exists
	sshDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Check if key already exists
	if _, err := os.Stat(keyPath); err == nil {
		log.Printf("SSH key already exists at %s", keyPath)
		return nil
	}

	// Generate ED25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	// Write private key
	privKeyPEM, err := ssh.MarshalPrivateKey(privKey, "cassh personal key")
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := os.WriteFile(keyPath, privKeyPEM.Bytes, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to create SSH public key: %w", err)
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	pubKeyPath := keyPath + ".pub"
	if err := os.WriteFile(pubKeyPath, pubKeyBytes, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	log.Printf("Generated new SSH key pair at %s", keyPath)
	return nil
}

// getKeyTitle generates a unique SSH key title for GitHub
// Format: cassh-{connID}@{hostname} (e.g., cassh-personal-123@MacBook-Pro)
func getKeyTitle(connID string) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	// Clean hostname - replace dots with dashes for cleaner display
	hostname = strings.ReplaceAll(hostname, ".", "-")
	// Truncate long hostnames
	if len(hostname) > 30 {
		hostname = hostname[:30]
	}
	return fmt.Sprintf("cassh-%s@%s", connID, hostname)
}

// getLegacyKeyTitle returns the old key title format without hostname
// Used for backwards compatibility when looking up existing keys
func getLegacyKeyTitle(connID string) string {
	return fmt.Sprintf("cassh-%s", connID)
}

// uploadSSHKeyToGitHub uploads an SSH public key to GitHub using gh CLI
// Returns the GitHub key ID for later deletion
func uploadSSHKeyToGitHub(keyPath string, title string) (string, error) {
	pubKeyPath := keyPath + ".pub"

	// Verify public key exists
	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("public key not found at %s", pubKeyPath)
	}

	// Use gh CLI to add the key
	cmd := exec.Command(findGHBinary(), "ssh-key", "add", pubKeyPath, "--title", title)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if key already exists
		if regexp.MustCompile(`already in use`).Match(output) {
			log.Printf("SSH key already exists on GitHub")
			// Try to find the key ID
			keyID := findGitHubKeyIDByTitle(title)
			return keyID, nil
		}
		return "", fmt.Errorf("failed to upload key: %s: %w", string(output), err)
	}

	log.Printf("Uploaded SSH key to GitHub: %s", title)

	// Get the key ID by listing keys and finding our title
	keyID := findGitHubKeyIDByTitle(title)
	return keyID, nil
}

// findGitHubKeyIDByTitle finds the GitHub SSH key ID by its title
// Also checks legacy title format for backward compatibility
func findGitHubKeyIDByTitle(title string) string {
	cmd := exec.Command(findGHBinary(), "ssh-key", "list")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to list SSH keys: %v", err)
		return ""
	}

	// Prepare legacy title if applicable
	// Check for new format (cassh-{connID}@{hostname}) vs legacy (cassh-{connID})
	var legacyTitle string
	if strings.Contains(title, "@") && strings.HasPrefix(title, "cassh-") {
		// Extract connection ID from new title format (cassh-{connID}@{hostname})
		withoutPrefix := strings.TrimPrefix(title, "cassh-")
		parts := strings.Split(withoutPrefix, "@")
		// Validate both connection ID and hostname are present
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			connID := parts[0]
			legacyTitle = getLegacyKeyTitle(connID)
		}
	}

	// Parse output to find our key
	// Format: "TITLE    TYPE    KEY    ADDED    KEY_ID    KEY_TYPE"
	// Example: "cassh-personal-123    ssh-ed25519    AAAA...    2025-12-09T02:43:40Z    137889594    authentication"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			keyTitle := fields[0]
			keyID := fields[len(fields)-2] // Key ID is the second-to-last field
			// Check for exact match with new title format
			if keyTitle == title {
				return keyID
			}
			// Check for exact match with legacy title format (if applicable)
			if legacyTitle != "" && keyTitle == legacyTitle {
				log.Printf("Found key with legacy title format: %s", legacyTitle)
				return keyID
			}
		}
	}

	return ""
}

// deleteSSHKeyFromGitHub deletes an SSH key from GitHub using gh CLI
func deleteSSHKeyFromGitHub(keyID string) error {
	if keyID == "" {
		return nil // Nothing to delete
	}

	cmd := exec.Command(findGHBinary(), "ssh-key", "delete", keyID, "--yes")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if key doesn't exist (already deleted)
		if strings.Contains(string(output), "not found") {
			log.Printf("SSH key %s already deleted from GitHub", keyID)
			return nil
		}
		return fmt.Errorf("failed to delete key %s: %s: %w", keyID, string(output), err)
	}

	log.Printf("Deleted SSH key %s from GitHub", keyID)
	return nil
}

// setupPersonalGitHubSSH generates a key and uploads it to GitHub
// Updates conn with KeyCreatedAt and GitHubKeyID
func setupPersonalGitHubSSH(conn *config.Connection) error {
	// 1. Generate key if needed
	if err := generateSSHKeyForPersonal(conn); err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}

	// 2. Upload to GitHub
	keyTitle := getKeyTitle(conn.ID)
	keyID, err := uploadSSHKeyToGitHub(conn.SSHKeyPath, keyTitle)
	if err != nil {
		return fmt.Errorf("key upload failed: %w", err)
	}

	// 3. Store key metadata
	conn.GitHubKeyID = keyID
	conn.KeyCreatedAt = time.Now().Unix()

	return nil
}

// rotatePersonalGitHubSSH rotates the SSH key for a personal GitHub connection
// Deletes old key from GitHub, generates new key, uploads new key
func rotatePersonalGitHubSSH(conn *config.Connection) error {
	log.Printf("Rotating SSH key for %s", conn.Name)

	// 1. Delete old key from GitHub
	if conn.GitHubKeyID != "" {
		if err := deleteSSHKeyFromGitHub(conn.GitHubKeyID); err != nil {
			log.Printf("Warning: failed to delete old key: %v", err)
			// Continue anyway - we still want to generate a new key
		}
	} else if conn.ID != "" {
		// No stored key ID - try to find and delete using both new and legacy title formats
		// This handles migration from older versions where GitHubKeyID wasn't tracked
		// Defensive check: ensure conn.ID is valid before attempting lookup
		keyTitle := getKeyTitle(conn.ID)
		if keyID := findGitHubKeyIDByTitle(keyTitle); keyID != "" {
			log.Printf("Found existing key on GitHub during rotation (ID: %s)", keyID)
			if err := deleteSSHKeyFromGitHub(keyID); err != nil {
				log.Printf("Warning: failed to delete existing key: %v", err)
			}
		}
	}

	// 2. Delete local key files
	os.Remove(conn.SSHKeyPath)
	os.Remove(conn.SSHKeyPath + ".pub")

	// 3. Generate new key
	if err := generateSSHKeyForPersonal(conn); err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}

	// 4. Upload new key to GitHub
	keyTitle := getKeyTitle(conn.ID)
	keyID, err := uploadSSHKeyToGitHub(conn.SSHKeyPath, keyTitle)
	if err != nil {
		return fmt.Errorf("key upload failed: %w", err)
	}

	// 5. Update connection metadata
	conn.GitHubKeyID = keyID
	conn.KeyCreatedAt = time.Now().Unix()

	log.Printf("SSH key rotated for %s (new key ID: %s)", conn.Name, keyID)
	return nil
}

// needsKeyRotation checks if a personal connection needs key rotation
func needsKeyRotation(conn *config.Connection) bool {
	if conn.Type != config.ConnectionTypePersonal {
		return false
	}
	if conn.KeyRotationHours <= 0 {
		return false // No rotation configured
	}
	if conn.KeyCreatedAt == 0 {
		return false // No creation time recorded
	}

	rotationDuration := time.Duration(conn.KeyRotationHours) * time.Hour
	keyAge := time.Since(time.Unix(conn.KeyCreatedAt, 0))

	return keyAge >= rotationDuration
}

// checkAndRotateExpiredKeys checks all personal connections and rotates expired keys
func checkAndRotateExpiredKeys() {
	// Wait a bit for app to fully initialize
	time.Sleep(2 * time.Second)

	// Check if gh CLI is available
	ghStatus := checkGHAuth()
	if !ghStatus.Installed || !ghStatus.Authenticated {
		return // Can't rotate keys without gh CLI
	}

	rotatedCount := 0
	for i := range cfg.User.Connections {
		conn := &cfg.User.Connections[i]
		if needsKeyRotation(conn) {
			log.Printf("Key rotation needed for %s (age: %v, policy: %dh)",
				conn.Name,
				time.Since(time.Unix(conn.KeyCreatedAt, 0)).Round(time.Hour),
				conn.KeyRotationHours)

			if err := rotatePersonalGitHubSSH(conn); err != nil {
				log.Printf("Failed to rotate key for %s: %v", conn.Name, err)
				sendNotification("cassh", fmt.Sprintf("Key rotation failed for %s", conn.Name), false)
				continue
			}

			rotatedCount++
		}
	}

	// Save config if any keys were rotated
	if rotatedCount > 0 {
		if err := config.SaveUserConfig(&cfg.User); err != nil {
			log.Printf("Failed to save config after key rotation: %v", err)
		} else {
			log.Printf("Rotated %d key(s) on startup", rotatedCount)
			if rotatedCount == 1 {
				sendNotification("cassh", "SSH key rotated automatically", false)
			} else {
				sendNotification("cassh", fmt.Sprintf("%d SSH keys rotated automatically", rotatedCount), false)
			}
		}
	}
}

// runHeadlessKeyRotation runs key rotation without starting the GUI
// This is called when the app is launched with --rotate-keys flag
// Used by the LaunchAgent for background key rotation
func runHeadlessKeyRotation() {
	log.Println("Running headless key rotation...")

	// Load config
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		log.Printf("Could not load user config: %v", err)
		os.Exit(1)
	}

	if len(userCfg.Connections) == 0 {
		log.Println("No connections configured, nothing to rotate")
		os.Exit(0)
	}

	// Check if gh CLI is available
	ghStatus := checkGHAuth()
	if !ghStatus.Installed {
		log.Println("GitHub CLI (gh) not installed, cannot rotate keys")
		os.Exit(1)
	}
	if !ghStatus.Authenticated {
		log.Println("GitHub CLI not authenticated, cannot rotate keys")
		os.Exit(1)
	}

	rotatedCount := 0
	for i := range userCfg.Connections {
		conn := &userCfg.Connections[i]
		if needsKeyRotation(conn) {
			log.Printf("Key rotation needed for %s (age: %v, policy: %dh)",
				conn.Name,
				time.Since(time.Unix(conn.KeyCreatedAt, 0)).Round(time.Hour),
				conn.KeyRotationHours)

			if err := rotatePersonalGitHubSSH(conn); err != nil {
				log.Printf("Failed to rotate key for %s: %v", conn.Name, err)
				continue
			}

			rotatedCount++
			log.Printf("Successfully rotated key for %s", conn.Name)
		}
	}

	// Save config if any keys were rotated
	if rotatedCount > 0 {
		if err := config.SaveUserConfig(userCfg); err != nil {
			log.Printf("Failed to save config: %v", err)
			os.Exit(1)
		}
		log.Printf("Rotated %d key(s)", rotatedCount)
	} else {
		log.Println("No keys needed rotation")
	}

	os.Exit(0)
}

// scheduleRestart restarts the app to rebuild the menu after adding connections
func scheduleRestart() {
	// Wait for the HTTP response to be sent
	time.Sleep(1 * time.Second)

	// Close the WebView window
	closeNativeWebView()

	// Get current executable path and derive app bundle path
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Failed to get executable path: %v", err)
		sendNotification("cassh", "Please quit and reopen cassh to see your new connection", false)
		return
	}

	log.Println("Restarting app to apply changes...")

	// For app bundles, use 'open' to relaunch properly
	// execPath is like /Applications/cassh.app/Contents/MacOS/cassh
	// We need to find the .app bundle path
	appPath := execPath
	isAppBundle := false
	if idx := strings.Index(execPath, ".app/"); idx != -1 {
		appPath = execPath[:idx+4] // Include ".app"
		isAppBundle = true
	}

	// Write a temporary script that will relaunch the app
	// Using a script file ensures it survives parent process exit
	scriptContent := fmt.Sprintf("#!/bin/bash\nsleep 2\nopen -a '%s'\nrm -f \"$0\"\n", appPath)
	if !isAppBundle {
		scriptContent = fmt.Sprintf("#!/bin/bash\nsleep 2\n'%s' &\nrm -f \"$0\"\n", execPath)
	}

	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("cassh-restart-%d.sh", time.Now().UnixNano()))
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		log.Printf("Failed to write restart script: %v", err)
		sendNotification("cassh", "Please quit and reopen cassh to see your new connection", false)
		return
	}

	// Launch the script with nohup so it survives our exit
	cmd := exec.Command("nohup", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group so it's not killed with parent
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to launch restart script: %v", err)
		os.Remove(scriptPath)
		sendNotification("cassh", "Please quit and reopen cassh to see your new connection", false)
		return
	}

	// Now quit - the script will relaunch after we're gone
	systray.Quit()
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Terminal icon - PNG template icon (black on transparent, macOS inverts with dark mode)
var terminalIcon = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x48, 0x00, 0x00, 0x00, 0x48,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x55, 0xed, 0xb3, 0x47, 0x00, 0x00, 0x00,
	0x06, 0x62, 0x4b, 0x47, 0x44, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0xa0,
	0xbd, 0xa7, 0x93, 0x00, 0x00, 0x04, 0x10, 0x49, 0x44, 0x41, 0x54, 0x78,
	0x9c, 0xed, 0x9c, 0x49, 0x8c, 0x0d, 0x41, 0x18, 0xc7, 0x7f, 0xf3, 0x30,
	0x07, 0xc6, 0x76, 0xc0, 0x65, 0x08, 0x09, 0x11, 0xbb, 0xd8, 0x97, 0x23,
	0x89, 0x03, 0x61, 0xc8, 0x58, 0x82, 0x8b, 0xb8, 0x8a, 0x90, 0x20, 0x2e,
	0xc2, 0x49, 0x18, 0x42, 0x86, 0x48, 0xb8, 0x8c, 0x8b, 0x25, 0x44, 0x2c,
	0x89, 0x35, 0x22, 0x2e, 0xe2, 0x60, 0x39, 0x38, 0x89, 0x41, 0x44, 0xc6,
	0x32, 0x12, 0xeb, 0x58, 0x32, 0x33, 0x98, 0x71, 0xa8, 0xee, 0x4c, 0xbf,
	0xea, 0xa5, 0xda, 0xab, 0x5e, 0xdf, 0xab, 0x5f, 0xf2, 0xe5, 0xf5, 0x52,
	0x5f, 0xd5, 0xf7, 0xfe, 0xa9, 0xfe, 0xba, 0xbb, 0xba, 0xab, 0xab, 0xf0,
	0xa6, 0x1a, 0xa8, 0xb3, 0x6c, 0x1a, 0x50, 0x0b, 0xf4, 0xf3, 0x29, 0x9b,
	0x57, 0x7e, 0x02, 0x6f, 0x80, 0xc7, 0xc0, 0x15, 0xe0, 0x32, 0xd0, 0x19,
	0xc6, 0xb1, 0x1e, 0x78, 0x09, 0x74, 0x57, 0x98, 0xbd, 0x00, 0x56, 0x04,
	0x09, 0xd3, 0x0b, 0x38, 0x98, 0x81, 0x40, 0xd3, 0xb6, 0x06, 0xa0, 0xe0,
	0x25, 0xd0, 0xa1, 0x0c, 0x04, 0x97, 0x15, 0xdb, 0x2f, 0x8b, 0x53, 0xef,
	0x51, 0xa8, 0x1d, 0x68, 0x04, 0x66, 0x53, 0x7e, 0xf9, 0x07, 0xc4, 0x7f,
	0x9a, 0x03, 0x1c, 0x01, 0x3a, 0x70, 0xff, 0xff, 0xe5, 0x76, 0xc1, 0x6a,
	0xdc, 0x39, 0xa7, 0x05, 0x98, 0x9c, 0x6c, 0xbc, 0xa9, 0x32, 0x15, 0x91,
	0xb0, 0x9d, 0x1a, 0x3c, 0x47, 0x68, 0xc3, 0x6a, 0xdc, 0x3d, 0x67, 0x8a,
	0x4f, 0x45, 0xc3, 0x81, 0x0b, 0x40, 0x9b, 0x65, 0x97, 0x80, 0xb1, 0x21,
	0x02, 0x48, 0xda, 0xaf, 0x14, 0xa6, 0xe2, 0xee, 0x49, 0xf5, 0x00, 0x67,
	0xa4, 0x8d, 0x8d, 0x01, 0xc1, 0x7e, 0xc2, 0xdd, 0x15, 0x3f, 0x5b, 0xfb,
	0xfc, 0x48, 0xda, 0x4f, 0x87, 0xa3, 0x52, 0x5b, 0xa7, 0x00, 0x9e, 0x49,
	0x1b, 0x67, 0xf9, 0x38, 0x5f, 0xf0, 0x08, 0xd6, 0xb6, 0xf3, 0x01, 0x8d,
	0x26, 0xed, 0xa7, 0xc3, 0x1c, 0xa9, 0x9d, 0xa7, 0x00, 0xdf, 0xa5, 0x8d,
	0x35, 0x3e, 0xce, 0x6d, 0x01, 0x01, 0x7f, 0x0b, 0x68, 0x34, 0x69, 0x3f,
	0x1d, 0xfa, 0x4b, 0xed, 0xb4, 0x15, 0x70, 0x0b, 0xf2, 0xa3, 0x84, 0x8a,
	0xbb, 0x4b, 0x0c, 0x28, 0x69, 0x3f, 0x15, 0xdf, 0xa5, 0xf5, 0xfe, 0x9e,
	0x17, 0x44, 0x3e, 0xdc, 0x09, 0xd8, 0x77, 0x2b, 0x43, 0x7e, 0x91, 0x23,
	0x77, 0x5f, 0x3f, 0xc6, 0x22, 0x12, 0xa4, 0x5c, 0xfe, 0x23, 0xe2, 0x5e,
	0x2d, 0x2b, 0x7e, 0xba, 0xb8, 0xf4, 0x08, 0x2b, 0x10, 0x88, 0xb3, 0xc7,
	0x79, 0x44, 0x0e, 0xf8, 0x06, 0x9c, 0x23, 0x5c, 0xb0, 0x49, 0xfb, 0xe9,
	0x50, 0xa4, 0x47, 0x15, 0x6e, 0x51, 0xaa, 0x62, 0x0e, 0x20, 0xeb, 0x14,
	0xe9, 0xf1, 0x3f, 0x39, 0xa8, 0x22, 0xf1, 0xea, 0x41, 0x06, 0x07, 0xa6,
	0x07, 0x29, 0x30, 0x02, 0x29, 0x30, 0x02, 0x29, 0xe8, 0xed, 0xb1, 0xcd,
	0x9c, 0xc5, 0x1c, 0x98, 0x1e, 0xa4, 0xc0, 0x08, 0xa4, 0xc0, 0x08, 0xa4,
	0xc0, 0x08, 0xa4, 0xc0, 0x08, 0xa4, 0xc0, 0x08, 0xa4, 0xc0, 0x08, 0xa4,
	0xc0, 0x08, 0xa4, 0xc0, 0x08, 0xa4, 0x20, 0x6a, 0x81, 0x46, 0x00, 0x0b,
	0x81, 0x51, 0x11, 0xd7, 0x9b, 0x1a, 0x51, 0x0b, 0xb4, 0x02, 0xb8, 0x0d,
	0xac, 0x8f, 0xb8, 0xde, 0xd4, 0x88, 0xeb, 0x10, 0x2b, 0x9b, 0xfb, 0x39,
	0x23, 0x50, 0x08, 0xfe, 0x67, 0xd0, 0xde, 0x8b, 0x89, 0x40, 0x13, 0xf0,
	0x1a, 0xe8, 0x72, 0xd4, 0xd3, 0x0a, 0xdc, 0x07, 0x0e, 0x20, 0x9e, 0x58,
	0xe6, 0x05, 0xad, 0xa7, 0x1a, 0x32, 0x6b, 0xf1, 0x7e, 0x75, 0x44, 0xb6,
	0xb3, 0x11, 0x04, 0x9e, 0x14, 0x45, 0xb1, 0xeb, 0x1c, 0x62, 0xa3, 0x81,
	0x93, 0x58, 0xaf, 0x88, 0x78, 0x34, 0x52, 0x16, 0xe8, 0x08, 0xb4, 0x91,
	0x62, 0x71, 0x36, 0x01, 0x3b, 0xac, 0xe5, 0x3d, 0xc0, 0x18, 0x60, 0x25,
	0x70, 0x9d, 0x9c, 0x0b, 0x56, 0xea, 0x21, 0x76, 0xcd, 0xe1, 0xf3, 0xce,
	0xda, 0xb6, 0xc5, 0x5a, 0xdf, 0x25, 0x95, 0x1d, 0xa4, 0x19, 0x63, 0x92,
	0x44, 0x76, 0x88, 0x39, 0x87, 0x6b, 0x07, 0x22, 0xde, 0x8c, 0xf0, 0xe3,
	0xab, 0x46, 0x3b, 0xa9, 0xa2, 0x23, 0x50, 0xb3, 0x63, 0xb9, 0x2f, 0x70,
	0x11, 0x18, 0xa7, 0x17, 0x4e, 0x36, 0x29, 0xf5, 0x10, 0x9b, 0x41, 0xf1,
	0x69, 0xdd, 0x69, 0x2d, 0x88, 0x04, 0xbe, 0x84, 0xfc, 0xdd, 0xef, 0x45,
	0x7a, 0x9a, 0xdf, 0x89, 0xbf, 0x48, 0xb6, 0x3d, 0x24, 0x5f, 0xf7, 0x66,
	0x91, 0x0a, 0x04, 0xb0, 0x08, 0xb8, 0x07, 0xfc, 0xf5, 0xa8, 0xcb, 0xb6,
	0x57, 0xc0, 0x60, 0xcd, 0xc0, 0x93, 0x22, 0x72, 0x81, 0x6c, 0x86, 0x21,
	0x0e, 0xab, 0x6e, 0xc4, 0x7b, 0x3d, 0x72, 0xcf, 0xda, 0xad, 0x51, 0x77,
	0x92, 0x44, 0x76, 0x16, 0x93, 0xf9, 0x00, 0x3c, 0xb1, 0x96, 0x0f, 0x03,
	0x0b, 0x80, 0x3f, 0x8e, 0xfd, 0x8b, 0x23, 0x6c, 0x2b, 0x31, 0xe2, 0x4c,
	0xa0, 0x77, 0x11, 0xf7, 0x62, 0x36, 0x71, 0xbf, 0xf8, 0x14, 0x0b, 0x3a,
	0x02, 0x6d, 0x06, 0xe6, 0x2b, 0xca, 0x38, 0xf3, 0xce, 0x2f, 0x8d, 0xb6,
	0x52, 0x43, 0x47, 0xa0, 0xd9, 0x88, 0xe4, 0xfc, 0x00, 0xd8, 0x0a, 0xcc,
	0xa4, 0x47, 0x90, 0x21, 0x88, 0x09, 0x21, 0x93, 0x1c, 0xe5, 0xef, 0x93,
	0x53, 0x4a, 0x4d, 0xd2, 0xa7, 0x3d, 0x7c, 0xfd, 0xac, 0x13, 0xff, 0xe9,
	0x0d, 0x59, 0x23, 0xb2, 0x24, 0xfd, 0x3e, 0x64, 0xb9, 0x0e, 0x60, 0x03,
	0x3d, 0x09, 0x3c, 0x77, 0xe8, 0x9c, 0xe6, 0x47, 0x02, 0xdb, 0x81, 0xab,
	0x88, 0x29, 0x0d, 0x9d, 0x56, 0x1d, 0xbf, 0x11, 0x33, 0xf8, 0x8e, 0x13,
	0xdf, 0xe4, 0x93, 0xb8, 0x88, 0xed, 0x3a, 0x08, 0x60, 0x1b, 0xf9, 0xba,
	0xe6, 0xf1, 0x22, 0xb6, 0xeb, 0x20, 0xbb, 0x72, 0xe7, 0x6f, 0xee, 0xf1,
	0x7a, 0xc3, 0x4c, 0x87, 0xae, 0x10, 0x65, 0x06, 0x00, 0x6b, 0x34, 0xdb,
	0x69, 0x01, 0x6e, 0x68, 0xd6, 0x11, 0x8a, 0xa8, 0x05, 0xb2, 0x09, 0xea,
	0x41, 0x43, 0x81, 0x13, 0x9a, 0xf5, 0xdf, 0x24, 0xa7, 0x02, 0x35, 0x21,
	0xe6, 0xa0, 0x7f, 0x89, 0xb8, 0xde, 0xd4, 0x88, 0x5a, 0x20, 0x7b, 0x4e,
	0x45, 0x10, 0xad, 0xc0, 0x2a, 0xcd, 0x76, 0x5a, 0x35, 0xfd, 0x43, 0x63,
	0xe6, 0x6a, 0xb8, 0x29, 0xd2, 0x23, 0x6f, 0xa3, 0x7d, 0x89, 0x63, 0x04,
	0x52, 0x60, 0x04, 0x52, 0x50, 0xc0, 0x3d, 0x47, 0x35, 0xe8, 0xf1, 0x4d,
	0xb9, 0x33, 0x40, 0x5a, 0x6f, 0x2b, 0xd0, 0xf3, 0xd0, 0xcf, 0x66, 0x7c,
	0x42, 0xc1, 0x64, 0x91, 0x09, 0xd2, 0xfa, 0xfb, 0x02, 0xf0, 0x48, 0xda,
	0xb8, 0x2e, 0xa1, 0x60, 0xb2, 0x88, 0xfc, 0xdf, 0x1f, 0x82, 0xb8, 0x26,
	0x71, 0xde, 0xa0, 0xb5, 0x23, 0x3e, 0xd3, 0x50, 0x69, 0x4c, 0xc3, 0xe7,
	0xd3, 0x14, 0xd5, 0x88, 0xa1, 0x09, 0xe7, 0x8e, 0x37, 0x54, 0x96, 0x48,
	0xd3, 0x81, 0xb7, 0x14, 0x6b, 0xd0, 0x0c, 0xf4, 0xb1, 0x0b, 0x2c, 0xc7,
	0x3d, 0xec, 0xd1, 0x81, 0xf8, 0x96, 0xc5, 0x5c, 0xca, 0x33, 0x71, 0xd7,
	0x00, 0xf3, 0x80, 0x63, 0xb8, 0x7b, 0x4e, 0x17, 0xb0, 0x54, 0x76, 0x68,
	0xc0, 0x2d, 0x52, 0xa5, 0xda, 0x5e, 0x2f, 0x45, 0x0b, 0x88, 0x81, 0xf6,
	0xb4, 0x83, 0x4b, 0xdb, 0xf6, 0xa1, 0xb8, 0x3e, 0xac, 0x43, 0x7c, 0x5c,
	0x28, 0xed, 0x40, 0x93, 0xb6, 0x66, 0x60, 0x99, 0x2c, 0x86, 0xdf, 0x8d,
	0x69, 0x1f, 0xab, 0x70, 0x1d, 0x22, 0x81, 0xd5, 0xe2, 0xff, 0x55, 0x98,
	0xbc, 0xf2, 0x03, 0x31, 0xf0, 0x66, 0x7f, 0x26, 0xf0, 0x0a, 0x62, 0x2c,
	0xbd, 0x88, 0x7f, 0xb2, 0x12, 0x36, 0x89, 0x0c, 0x4c, 0x47, 0x23, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}
