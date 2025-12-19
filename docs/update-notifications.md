# Update Notifications

cassh includes an intelligent update notification system that ensures you never miss critical updates while not being overly intrusive.

## Features

- **Automatic Update Checks**: Checks for new releases from GitHub automatically
- **Persistent Notifications**: Sends periodic reminders about available updates
- **Dismissible Updates**: Dismiss notifications for specific versions you don't want to install
- **Visual Indicators**: Menu bar shows 🔔 icon when updates are available
- **Configurable Intervals**: Control how often to check and how often to be reminded
- **Native macOS Notifications**: Uses system notifications that appear in Notification Center

## How It Works

### Automatic Checks

cassh automatically checks for updates:

1. **On Startup**: 5 seconds after app launch (doesn't slow down startup)
2. **Periodically**: Based on your configured interval (default: daily)
3. **Last Check Tracking**: Skips checks if recently checked

### Notification Behavior

When a new version is available:

1. **Menu Bar Indicator**: Shows "🔔 Update Available: vX.Y.Z"
2. **Native Notification**: macOS notification appears with version info
3. **Persistent Reminders**: If you don't update, periodic reminders are sent (default: every 6 hours)
4. **Dismissal**: You can dismiss notifications for a specific version

### Menu Actions

**Check for Updates...**
- When clicked: Opens releases page if update is available
- Otherwise: Manually checks for updates and shows a dialog

**Dismiss Update** (appears when update is available)
- Stops notifications for the current version
- Remains hidden until next version is released
- Menu item hides after dismissing

## Configuration

### Config File Location

Updates are configured in `~/.config/cassh/config.toml`:

```toml
# Update notification settings
update_check_enabled = true        # Enable automatic update checks
update_check_interval_days = 1     # Days between update checks
update_notify_persistent = true    # Show persistent notifications
update_notify_interval_min = 360   # Minutes between re-notifications (6 hours)
dismissed_update_version = "1.2.3" # Version user dismissed
last_update_check_time = 1703001234 # Unix timestamp of last check
```

### Configuration Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `update_check_enabled` | bool | `true` | Enable/disable automatic update checks |
| `update_check_interval_days` | int | `1` | How often to check for updates (in days) |
| `update_notify_persistent` | bool | `true` | Send periodic reminder notifications |
| `update_notify_interval_min` | int | `360` | Minutes between reminder notifications |
| `dismissed_update_version` | string | `""` | Version that user dismissed (auto-managed) |
| `last_update_check_time` | int64 | `0` | Unix timestamp of last check (auto-managed) |

### Disabling Update Checks

To completely disable update checks, edit your config:

```toml
update_check_enabled = false
```

After disabling, the menu item will still be available for manual checks.

### Adjusting Notification Frequency

To reduce notification frequency:

```toml
# Check weekly instead of daily
update_check_interval_days = 7

# Notify once per day instead of every 6 hours
update_notify_interval_min = 1440  # 24 hours
```

To disable persistent reminders but keep update checks:

```toml
# Check for updates, but only notify once
update_notify_persistent = false
```

## User Workflows

### Installing an Update

1. See notification: "cassh Update Available"
2. Click notification or menu bar item
3. Opens releases page in browser
4. Download and install new version
5. Restart cassh

### Dismissing an Update

If you don't want to update to a specific version:

1. Click "Dismiss Update" from menu bar
2. Notification appears: "Update Dismissed"
3. No more notifications for this version
4. When a newer version is released, notifications resume

### Manual Update Check

To manually check for updates:

1. Click "Check for Updates..." in menu bar
2. Dialog shows current and latest version
3. If update available, option to download
4. If up to date, confirmation dialog appears

## Notification Examples

### Initial Update Available

```
╔══════════════════════════════════════╗
║  cassh Update Available              ║
║                                      ║
║  Version 1.2.0 is available.         ║
║  You're on v1.1.0.                   ║
║                                      ║
║  Click to download or dismiss        ║
║  in menu.                            ║
╚══════════════════════════════════════╝
```

### Periodic Reminder

```
╔══════════════════════════════════════╗
║  cassh Update Available              ║
║                                      ║
║  Version 1.2.0 is available.         ║
║  You're on v1.1.0.                   ║
║                                      ║
║  Click to download.                  ║
╚══════════════════════════════════════╝
```

### After Dismissal

```
╔══════════════════════════════════════╗
║  Update Dismissed                    ║
║                                      ║
║  You can check for updates again     ║
║  from the menu.                      ║
║                                      ║
║  Dismissed version: v1.2.0           ║
╚══════════════════════════════════════╝
```

## Menu Bar States

### No Update Available

```
Check for Updates...
```

### Update Available (Not Dismissed)

```
🔔 Update Available: v1.2.0
  Dismiss Update
```

### Update Available (Dismissed)

```
Check for Updates...
```
(Menu title returns to normal, but dismissed version is tracked)

## Technical Details

### Update Check Process

1. **Fetch Latest Release**: Calls GitHub API (`/repos/shawntz/cassh/releases/latest`)
2. **Version Comparison**: Compares semantic versions (major.minor.patch)
3. **Update Config**: Saves last check time and dismissed version
4. **Show Notification**: If new version and not dismissed

### Notification System

Uses macOS native notifications via AppleScript:

```applescript
display notification "Version X is available..."
  with title "cassh Update Available"
  sound name "default"
```

Notifications appear in:
- Notification Center (can be clicked to action)
- Banner (top-right corner, auto-dismiss)
- Lock screen (if enabled in System Preferences)

### Background Tasks

Two background goroutines run:

1. **Periodic Update Checker**: Checks for new releases at configured interval
2. **Persistent Notifier**: Sends reminder notifications for undismissed updates

Both respect user configuration and stop if updates are disabled.

## Privacy & Security

- **No Tracking**: Update checks only query public GitHub API
- **No Analytics**: No data is sent about your usage or update behavior
- **Rate Limiting**: Respects GitHub API rate limits (60 requests/hour unauthenticated)
- **User Agent**: Identifies as `cassh/VERSION` to GitHub
- **Local Storage**: Dismissed versions and last check times stored locally only

## Troubleshooting

### Not Receiving Notifications

**Check notification permissions:**
1. System Preferences → Notifications
2. Find "cassh" in the list
3. Ensure notifications are enabled
4. Set alert style to "Banners" or "Alerts"

**Check cassh config:**
```bash
cat ~/.config/cassh/config.toml | grep update
```

Ensure `update_check_enabled = true`.

**Check logs:**
```bash
# macOS Console.app
# Filter: process:cassh-menubar
# Look for: "Update available" or "Failed to check for updates"
```

### Notifications Too Frequent

Adjust the interval in config:

```toml
# Check less often
update_check_interval_days = 7

# Notify less often
update_notify_interval_min = 1440  # Daily instead of every 6 hours
```

### Notifications Not Stopping After Dismiss

Check if dismissed version is saved:

```bash
grep dismissed_update_version ~/.config/cassh/config.toml
```

Should show:
```toml
dismissed_update_version = "1.2.0"
```

If not saving, check file permissions:
```bash
ls -la ~/.config/cassh/config.toml
# Should be: -rw------- (0600)
```

### Dev Builds Always Show Update

If running a dev build (`version = "dev"`), it always considers itself outdated. This is intentional - dev builds should not be used in production.

## Comparison with Other Apps

| Feature | cassh | Sparkle Framework | Manual Checks Only |
|---------|-------|-------------------|-------------------|
| Auto-update check | ✅ | ✅ | ❌ |
| Persistent notifications | ✅ | ❌ | ❌ |
| Dismissible updates | ✅ | ❌ | ❌ |
| Configurable intervals | ✅ | ⚠️ | N/A |
| No external dependencies | ✅ | ❌ | ✅ |
| Native notifications | ✅ | ⚠️ | N/A |

## Future Enhancements

Planned improvements:

- [ ] In-app update installation (auto-download and install)
- [ ] Release notes display in notification
- [ ] Update channels (stable, beta, dev)
- [ ] Silent auto-updates option
- [ ] Rollback to previous version
- [ ] Delta updates (download only changes)

## Related Documentation

- [Release Process](releases.md) - How releases are created
- [Configuration Guide](configuration.md) - Full config reference
- [Contributing](../CONTRIBUTING.md) - How to contribute

## Feedback

Have suggestions for improving update notifications?

- [Open an issue](https://github.com/shawntz/cassh/issues/new)
- [Discussion forum](https://github.com/shawntz/cassh/discussions)
