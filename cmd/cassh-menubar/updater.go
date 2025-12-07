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
	menuCheckUpdates *systray.MenuItem
	latestVersion    string
	updateStatus     UpdateStatus
)

// setupUpdateMenu adds the update menu item
func setupUpdateMenu() *systray.MenuItem {
	menuCheckUpdates = systray.AddMenuItem("Check for Updates...", "Check for new versions")
	return menuCheckUpdates
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
		menuCheckUpdates.SetTitle(fmt.Sprintf("Update Available: v%s ↗", latestVersion))
		menuCheckUpdates.Enable()
		log.Printf("Update available: %s -> %s", currentVersion, latestVersion)
		// Show update available dialog
		if showUpdateAvailableDialog(latestVersion) {
			openBrowser(releasesPageURL)
		}
	} else {
		updateStatus = UpdateStatusUpToDate
		menuCheckUpdates.SetTitle("Check for Updates...")
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

	if isNewerVersion(latestVersion, currentVersion) {
		updateStatus = UpdateStatusAvailable
		menuCheckUpdates.SetTitle(fmt.Sprintf("Update Available: v%s ↗", latestVersion))
		log.Printf("Update available (background check): %s -> %s", currentVersion, latestVersion)
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
