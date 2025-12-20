//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

const (
	githubRepoOwner = "shawntz"
	githubRepoName  = "cassh"
	updateCheckURL  = "https://api.github.com/repos/" + githubRepoOwner + "/" + githubRepoName + "/releases/latest"
	releasesPageURL = "https://github.com/" + githubRepoOwner + "/" + githubRepoName + "/releases/latest"
)

// GitHubRelease represents a GitHub release from the API
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
}

// UpdateStatus represents the current update state
type UpdateStatus int

const (
	UpdateStatusUnknown UpdateStatus = iota
	UpdateStatusUpToDate
	UpdateStatusAvailable
	UpdateStatusError
	UpdateStatusNoReleases
)

var (
	menuCheckUpdates       *systray.MenuItem
	menuDismissUpdate      *systray.MenuItem
	latestVersion          string
	updateStatus           UpdateStatus
	lastNotificationTime   time.Time
	updateNotificationSent bool
)

// setupUpdateMenu adds the update menu items
func setupUpdateMenu() (*systray.MenuItem, *systray.MenuItem) {
	menuCheckUpdates = systray.AddMenuItem("Check for Updates...", "Check for new versions")
	menuDismissUpdate = systray.AddMenuItem("  Dismiss Update", "Stop notifications for this version")
	menuDismissUpdate.Hide() // Hidden by default, shown when update is available
	return menuCheckUpdates, menuDismissUpdate
}

// handleUpdateMenuClick handles clicks on the update menu item
func handleUpdateMenuClick() {
	switch updateStatus {
	case UpdateStatusAvailable:
		// Open releases page directly if update is available
		openBrowser(releasesPageURL)
	default:
		// Check for updates
		go checkForUpdatesWithUI()
	}
}

// handleDismissUpdateClick handles clicks on the dismiss update menu item
func handleDismissUpdateClick() {
	dismissUpdate()
}

// checkForUpdatesWithUI checks for updates and shows dialogs
func checkForUpdatesWithUI() {
	menuCheckUpdates.SetTitle("Checking...")
	menuCheckUpdates.Disable()

	release, err := fetchLatestRelease()
	currentVersion := normalizeVersion(version)

	if err != nil {
		// Check if it's a 404 (no releases yet)
		if strings.Contains(err.Error(), "404") {
			log.Printf("No releases found yet - you're on the latest version")
			updateStatus = UpdateStatusNoReleases
			menuCheckUpdates.SetTitle("Check for Updates...")
			menuCheckUpdates.Enable()
			// Show dialog
			showUpdateDialog("cassh", fmt.Sprintf("You're on the latest version of cassh!\n\nVersion: v%s", currentVersion))
			return
		}

		log.Printf("Failed to check for updates: %v", err)
		menuCheckUpdates.SetTitle("Check for Updates...")
		menuCheckUpdates.Enable()
		updateStatus = UpdateStatusError
		// Show error dialog
		showUpdateDialog("Update Check Failed", "Could not check for updates.\nPlease check your internet connection.")
		return
	}

	latestVersion = normalizeVersion(release.TagName)

	if isNewerVersion(latestVersion, currentVersion) {
		updateStatus = UpdateStatusAvailable
		menuCheckUpdates.SetTitle(fmt.Sprintf("🔔 Update Available: v%s", latestVersion))
		if menuDismissUpdate != nil {
			menuDismissUpdate.Show()
		}
		menuCheckUpdates.Enable()
		log.Printf("Update available: %s -> %s", currentVersion, latestVersion)
		// Show update available dialog
		if showUpdateAvailableDialog(latestVersion) {
			openBrowser(releasesPageURL)
		}
	} else {
		updateStatus = UpdateStatusUpToDate
		menuCheckUpdates.SetTitle("Check for Updates...")
		if menuDismissUpdate != nil {
			menuDismissUpdate.Hide()
		}
		menuCheckUpdates.Enable()
		log.Printf("Already up to date: %s", currentVersion)
		// Show up to date dialog
		showUpdateDialog("cassh", fmt.Sprintf("You're on the latest version of cassh!\n\nVersion: v%s", currentVersion))
	}
}

// checkForUpdatesBackground silently checks for updates on startup
func checkForUpdatesBackground() {
	// Wait a bit before checking to not slow down startup
	time.Sleep(5 * time.Second)

	// Check if update checks are disabled
	if !cfg.User.UpdateCheckEnabled {
		log.Printf("Update checks disabled by user")
		return
	}

	// Check if we should check for updates based on interval
	lastCheckTime := time.Unix(cfg.User.LastUpdateCheckTime, 0)
	checkInterval := time.Duration(cfg.User.UpdateCheckIntervalDays) * 24 * time.Hour
	if cfg.User.UpdateCheckIntervalDays == 0 {
		checkInterval = 24 * time.Hour // Default to daily
	}

	if time.Since(lastCheckTime) < checkInterval {
		log.Printf("Skipping update check, last checked %v ago (interval: %v)", time.Since(lastCheckTime), checkInterval)
		return
	}

	release, err := fetchLatestRelease()
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			log.Printf("No releases found yet (background check)")
			updateStatus = UpdateStatusNoReleases
		} else {
			log.Printf("Background update check failed: %v", err)
		}
		return
	}

	latestVersion = normalizeVersion(release.TagName)
	currentVersion := normalizeVersion(version)

	// Update last check time
	cfg.User.LastUpdateCheckTime = time.Now().Unix()
	if err := config.SaveUserConfig(&cfg.User); err != nil {
		log.Printf("Failed to save config after update check: %v", err)
	}

	if isNewerVersion(latestVersion, currentVersion) {
		updateStatus = UpdateStatusAvailable
		menuCheckUpdates.SetTitle(fmt.Sprintf("🔔 Update Available: v%s", latestVersion))
		if menuDismissUpdate != nil {
			menuDismissUpdate.Show()
		}
		log.Printf("Update available (background check): %s -> %s", currentVersion, latestVersion)

		// Check if user dismissed this version
		if cfg.User.DismissedUpdateVersion == latestVersion {
			log.Printf("User dismissed update v%s, skipping notification", latestVersion)
			return
		}

		// Show persistent notification
		showUpdateNotification(latestVersion, release)
	} else {
		updateStatus = UpdateStatusUpToDate
		if menuDismissUpdate != nil {
			menuDismissUpdate.Hide()
		}
		log.Printf("Already up to date: %s", currentVersion)
	}
}

// startPeriodicUpdateChecker starts a background goroutine that checks for updates periodically
func startPeriodicUpdateChecker() {
	if !cfg.User.UpdateCheckEnabled {
		return
	}

	go func() {
		// Initial check after startup
		checkForUpdatesBackground()

		// Set up periodic checks with dynamic config reloading
		checkInterval := time.Duration(cfg.User.UpdateCheckIntervalDays) * 24 * time.Hour
		if cfg.User.UpdateCheckIntervalDays == 0 {
			checkInterval = 24 * time.Hour
		}

		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		// Track current interval to detect changes
		currentIntervalDays := cfg.User.UpdateCheckIntervalDays

		for range ticker.C {
			if !cfg.User.UpdateCheckEnabled {
				log.Printf("Update checks disabled, stopping periodic checker")
				return
			}

			// Reload config to check for interval changes
			userCfg, err := config.LoadUserConfig()
			if err == nil {
				// Check if the interval has changed
				if userCfg.UpdateCheckIntervalDays != currentIntervalDays {
					newInterval := time.Duration(userCfg.UpdateCheckIntervalDays) * 24 * time.Hour
					if userCfg.UpdateCheckIntervalDays == 0 {
						newInterval = 24 * time.Hour
					}

					log.Printf("Update check interval changed from %d to %d days, resetting ticker", currentIntervalDays, userCfg.UpdateCheckIntervalDays)
					currentIntervalDays = userCfg.UpdateCheckIntervalDays
					cfg.User.UpdateCheckIntervalDays = userCfg.UpdateCheckIntervalDays

					// Reset ticker with new interval (no memory leak, same ticker instance)
					ticker.Reset(newInterval)
				}

				// Update global config with reloaded values
				cfg.User = *userCfg
			}

			release, err := fetchLatestRelease()
			if err != nil {
				log.Printf("Periodic update check failed: %v", err)
				continue
			}

			latestVersion = normalizeVersion(release.TagName)
			currentVersion := normalizeVersion(version)

			// Update last check time
			cfg.User.LastUpdateCheckTime = time.Now().Unix()
			if err := config.SaveUserConfig(&cfg.User); err != nil {
				log.Printf("Failed to save config after periodic update check: %v", err)
			}

			if isNewerVersion(latestVersion, currentVersion) {
				if cfg.User.DismissedUpdateVersion != latestVersion {
					updateStatus = UpdateStatusAvailable
					menuCheckUpdates.SetTitle(fmt.Sprintf("🔔 Update Available: v%s", latestVersion))
					if menuDismissUpdate != nil {
						menuDismissUpdate.Show()
					}
					log.Printf("Update available (periodic check): %s -> %s", currentVersion, latestVersion)

					// Show notification if persistent notifications are enabled
					if cfg.User.UpdateNotifyPersistent {
						showUpdateNotification(latestVersion, release)
					}
				}
			} else {
				if menuDismissUpdate != nil {
					menuDismissUpdate.Hide()
				}
			}
		}
	}()
}

// startPersistentUpdateNotifier sends periodic reminders about available updates
func startPersistentUpdateNotifier() {
	if !cfg.User.UpdateNotifyPersistent {
		return
	}

	notifyInterval := time.Duration(cfg.User.UpdateNotifyIntervalMin) * time.Minute
	if cfg.User.UpdateNotifyIntervalMin == 0 {
		notifyInterval = 6 * time.Hour // Default to 6 hours
	}

	go func() {
		ticker := time.NewTicker(notifyInterval)
		defer ticker.Stop()

		for range ticker.C {
			// Only notify if update is available and not dismissed
			if updateStatus == UpdateStatusAvailable &&
				cfg.User.DismissedUpdateVersion != latestVersion &&
				cfg.User.UpdateNotifyPersistent {

				// Check if we've sent a notification recently
				if time.Since(lastNotificationTime) >= notifyInterval {
					currentVersion := normalizeVersion(version)
					log.Printf("Sending persistent update reminder: %s -> %s", currentVersion, latestVersion)
					sendNativeNotification(
						"cassh Update Available",
						fmt.Sprintf("Version %s is available. You're on v%s.\n\nClick to download.", latestVersion, currentVersion),
						"update-available",
					)
					lastNotificationTime = time.Now()
				}
			}
		}
	}()
}

// showUpdateNotification shows a native macOS notification about the update
func showUpdateNotification(newVersion string, release *GitHubRelease) {
	currentVersion := normalizeVersion(version)
	message := fmt.Sprintf("Version %s is available. You're on v%s.\n\nClick to download or dismiss in menu.", newVersion, currentVersion)

	sendNativeNotification(
		"cassh Update Available",
		message,
		"update-available",
	)

	lastNotificationTime = time.Now()
	updateNotificationSent = true
}

// sendNativeNotification sends a macOS User Notification
func sendNativeNotification(title, message, identifier string) {
	script := fmt.Sprintf(`
		display notification "%s" with title "%s" sound name "default"
	`, escapeForAppleScript(message), escapeForAppleScript(title))

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to send notification: %v", err)
	}
}

// escapeForAppleScript escapes quotes and backslashes for AppleScript
func escapeForAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// dismissUpdate marks the current update version as dismissed
func dismissUpdate() {
	if latestVersion == "" {
		return
	}

	cfg.User.DismissedUpdateVersion = latestVersion
	if err := config.SaveUserConfig(&cfg.User); err != nil {
		log.Printf("Failed to save dismissed update version: %v", err)
	} else {
		log.Printf("Dismissed update v%s", latestVersion)
		updateStatus = UpdateStatusUpToDate
		menuCheckUpdates.SetTitle("Check for Updates...")
		if menuDismissUpdate != nil {
			menuDismissUpdate.Hide()
		}
		showNotification("Update Dismissed", fmt.Sprintf("You can check for updates again from the menu.\n\nDismissed version: v%s", latestVersion))
	}
}

// clearDismissedUpdate clears the dismissed update version (called when manually checking for updates)
func clearDismissedUpdate() {
	if cfg.User.DismissedUpdateVersion != "" {
		cfg.User.DismissedUpdateVersion = ""
		if err := config.SaveUserConfig(&cfg.User); err != nil {
			log.Printf("Failed to clear dismissed update version: %v", err)
		}
	}
}

// fetchLatestRelease fetches the latest release from GitHub API
func fetchLatestRelease() (*GitHubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", updateCheckURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "cassh/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("404: no releases found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &release, nil
}

// normalizeVersion removes 'v' prefix and cleans up version string
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	// Remove any suffix after hyphen (e.g., "1.0.0-dirty" -> "1.0.0")
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	return v
}

// isNewerVersion returns true if latest is newer than current
func isNewerVersion(latest, current string) bool {
	// Handle dev builds - always consider them outdated
	if current == "dev" || current == "" {
		return true
	}

	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	// Pad with zeros if needed
	for len(latestParts) < 3 {
		latestParts = append(latestParts, "0")
	}
	for len(currentParts) < 3 {
		currentParts = append(currentParts, "0")
	}

	// Compare major.minor.patch
	for i := 0; i < 3; i++ {
		var latestNum, currentNum int
		fmt.Sscanf(latestParts[i], "%d", &latestNum)
		fmt.Sscanf(currentParts[i], "%d", &currentNum)

		if latestNum > currentNum {
			return true
		} else if latestNum < currentNum {
			return false
		}
	}

	return false // Same version
}

// showUpdateDialog shows a native macOS dialog using AppleScript
func showUpdateDialog(title, message string) {
	script := fmt.Sprintf(`display dialog "%s" with title "%s" buttons {"OK"} default button "OK"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to show dialog: %v", err)
	}
}

// showUpdateAvailableDialog shows dialog with option to download
func showUpdateAvailableDialog(newVersion string) bool {
	script := fmt.Sprintf(`display dialog "A new version of cassh is available!\n\nCurrent: v%s\nLatest: v%s\n\nWould you like to download it?" with title "Update Available" buttons {"Later", "Download"} default button "Download"`, normalizeVersion(version), newVersion)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Dialog error: %v", err)
		return false
	}
	return strings.Contains(string(output), "Download")
}
