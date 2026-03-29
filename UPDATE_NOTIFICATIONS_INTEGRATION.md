# Update Notifications Integration Guide

This document explains how to integrate the new update notification system into the cassh menu bar app.

## Changes Made

### 1. Configuration (`internal/config/config.go`)

Added update notification settings to `UserConfig`:

```go
// Update notification settings
UpdateCheckEnabled      bool   `toml:"update_check_enabled"`
UpdateCheckIntervalDays int    `toml:"update_check_interval_days"`
UpdateNotifyPersistent  bool   `toml:"update_notify_persistent"`
UpdateNotifyIntervalMin int    `toml:"update_notify_interval_min"`
DismissedUpdateVersion  string `toml:"dismissed_update_version"`
LastUpdateCheckTime     int64  `toml:"last_update_check_time"`
```

### 2. Updater (`cmd/cassh-menubar/updater.go`)

Enhanced with:

- **Persistent Notifications**: `showUpdateNotification()`, `sendNativeNotification()`
- **Periodic Checks**: `startPeriodicUpdateChecker()`
- **Persistent Reminders**: `startPersistentUpdateNotifier()`
- **Dismiss Functionality**: `dismissUpdate()`, `clearDismissedUpdate()`
- **Menu Items**: `menuDismissUpdate` for dismissing updates
- **Visual Indicators**: 🔔 emoji in menu when update available

## Integration Steps

### Step 1: Update Menu Setup in `main.go`

Find where `setupUpdateMenu()` is called and update it:

**Before:**
```go
menuCheckUpdates := setupUpdateMenu()
```

**After:**
```go
menuCheckUpdates, menuDismissUpdate := setupUpdateMenu()
```

### Step 2: Add Menu Click Handlers

Add handler for the dismiss menu item in your main event loop:

**In `onReady()` or main event loop:**
```go
go func() {
    for {
        select {
        case <-menuCheckUpdates.ClickedCh:
            handleUpdateMenuClick()
        case <-menuDismissUpdate.ClickedCh:
            handleDismissUpdateClick()
        // ... other menu handlers ...
        }
    }
}()
```

### Step 3: Start Background Tasks

Replace the old `checkForUpdatesBackground()` call with the new system:

**Before:**
```go
go checkForUpdatesBackground()
```

**After:**
```go
// Start periodic update checker (checks at configured interval)
startPeriodicUpdateChecker()

// Start persistent notifier (sends reminders)
startPersistentUpdateNotifier()
```

**Note**: `checkForUpdatesBackground()` is now called automatically by `startPeriodicUpdateChecker()`.

### Step 4: Clear Dismissed Version on Manual Check (Optional)

When user manually checks for updates, optionally clear dismissed version:

```go
// In checkForUpdatesWithUI(), before checking:
clearDismissedUpdate() // Allow user to see update they previously dismissed
```

## Complete Integration Example

Here's a complete example of how to wire everything up in `main.go`:

```go
package main

import (
    "log"
    "github.com/getlantern/systray"
    "your-project/internal/config"
)

var (
    cfg                *config.MergedConfig
    menuCheckUpdates   *systray.MenuItem
    menuDismissUpdate  *systray.MenuItem
)

func onReady() {
    // Set up systray
    systray.SetIcon(iconData)
    systray.SetTitle("cassh")
    systray.SetTooltip("SSH Certificate Manager")

    // Build menu
    // ... add connection menu items ...

    systray.AddSeparator()

    // Setup update menu (returns both menu items)
    menuCheckUpdates, menuDismissUpdate = setupUpdateMenu()

    systray.AddSeparator()

    // ... other menu items ...

    menuQuit := systray.AddMenuItem("Quit", "Quit cassh")

    // Start background tasks
    go monitorConnections()

    // NEW: Start update notification system
    startPeriodicUpdateChecker()    // Checks for updates periodically
    startPersistentUpdateNotifier() // Sends reminder notifications

    // Event loop
    go func() {
        for {
            select {
            case <-menuCheckUpdates.ClickedCh:
                handleUpdateMenuClick()
            case <-menuDismissUpdate.ClickedCh:
                handleDismissUpdateClick()
            // ... other menu item handlers ...
            case <-menuQuit.ClickedCh:
                systray.Quit()
                return
            }
        }
    }()
}

func main() {
    // Load config
    var err error
    cfg, err = loadConfig()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Run app
    systray.Run(onReady, onExit)
}

func onExit() {
    log.Println("Exiting cassh...")
}
```

## Configuration Defaults

The system works with sensible defaults:

| Setting | Default | Description |
|---------|---------|-------------|
| `update_check_enabled` | `true` | Auto-checks enabled |
| `update_check_interval_days` | `1` | Check daily |
| `update_notify_persistent` | `true` | Send reminders |
| `update_notify_interval_min` | `360` | Remind every 6 hours |

Users can customize these in `~/.config/cassh/config.toml`.

## Testing

### Test Update Available Flow

1. Build with an old version:
   ```bash
   make menubar VERSION=0.0.1
   ```

2. Run the app:
   ```bash
   ./build/cassh-menubar
   ```

3. After 5 seconds, should see:
   - Menu item: "🔔 Update Available: vX.Y.Z"
   - Native notification appears
   - "Dismiss Update" menu item appears

### Test Dismiss Flow

1. Click "Dismiss Update" from menu
2. Menu item changes back to "Check for Updates..."
3. "Dismiss Update" menu item hides
4. Check config:
   ```bash
   grep dismissed_update_version ~/.config/cassh/config.toml
   ```
   Should show the dismissed version

### Test Persistent Notifications

1. Don't dismiss update
2. Wait 6 hours (or change `update_notify_interval_min` to 1 for testing)
3. Another notification should appear

### Test Manual Check

1. Click "Check for Updates..."
2. Dialog appears with current and latest version
3. If update available, option to download

## Troubleshooting Integration

### Menu Items Not Appearing

**Check menu setup:**
```go
menuCheckUpdates, menuDismissUpdate := setupUpdateMenu()
```

Make sure you're capturing both return values.

### Notifications Not Sending

**Check config loaded:**
```go
log.Printf("Update checks enabled: %v", cfg.User.UpdateCheckEnabled)
log.Printf("Persistent notifications: %v", cfg.User.UpdateNotifyPersistent)
```

**Check background tasks started:**
```go
startPeriodicUpdateChecker()    // Must be called
startPersistentUpdateNotifier() // Must be called
```

### Dismiss Not Working

**Check event handler:**
```go
case <-menuDismissUpdate.ClickedCh:
    handleDismissUpdateClick()
```

**Check config save:**
```go
// In dismissUpdate()
if err := config.SaveUserConfig(&cfg.User); err != nil {
    log.Printf("Failed to save: %v", err) // Should not appear
}
```

## Migration from Old System

If you're migrating from the old update checker:

**Old Code (Remove):**
```go
go checkForUpdatesBackground() // OLD - remove this
```

**New Code (Add):**
```go
startPeriodicUpdateChecker()    // NEW - replaces old background check
startPersistentUpdateNotifier() // NEW - adds persistent reminders
```

The new system is backwards compatible and will work with existing installations.

## Performance Considerations

- **Startup**: 5-second delay before first check (doesn't slow startup)
- **CPU Usage**: Minimal - only runs when timer fires
- **Memory**: ~1KB for config, negligible for goroutines
- **Network**: One API call per day (default), < 1KB response
- **Battery**: Negligible impact on battery life

## Future Enhancements

When implementing these, consider:

- **In-app updates**: Download and install without browser
- **Release notes**: Show changelog in notification
- **Update channels**: Stable, beta, dev
- **Silent updates**: Auto-install without user interaction

## Questions?

If you encounter issues integrating:

1. Check the logs: `log.Printf` statements in `updater.go`
2. Verify config: `cat ~/.config/cassh/config.toml`
3. Test manually: Click "Check for Updates..."
4. Review this guide
5. Open an issue on GitHub
