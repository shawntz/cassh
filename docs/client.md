# Client Distribution

This guide covers distributing the `cassh` menu bar app to users.

## macOS Menu Bar App

The menu bar app provides a visual indicator of certificate status and one-click renewal.

### Building the App

**One-liner build script** (after configuring `cassh.policy.toml`):

```bash
./build-release.sh
```

This script validates your config, builds the binary, creates the app bundle, DMG, and PKG.

**Or build step-by-step:**

```bash
# Build the binary
make menubar

# Create the app bundle
make app-bundle

# Create DMG for manual distribution (requires sudo for disk image mounting)
sudo make dmg

# Create PKG for MDM deployment
make pkg
```

### App Bundle Structure

```
cassh.app/
├── Contents/
│   ├── Info.plist
│   ├── MacOS/
│   │   └── cassh          # Binary
│   └── Resources/
│       ├── cassh.icns     # App icon
│       └── cassh.policy.toml  # Bundled policy (enterprise)
```

---

## MDM Deployment (Jamf, Kandji, etc.)

For enterprise deployment, use the PKG installer which:

- Installs the app to `/Applications`
- Bundles the policy configuration
- Sets up LaunchAgent for auto-start

### Build Enterprise PKG

```bash
# Ensure policy is configured
cp cassh.policy.example.toml cassh.policy.toml
# Edit cassh.policy.toml with your settings

# Build signed PKG
make pkg
```

### PKG Contents

The PKG installs:

| Path | Contents |
|------|----------|
| `/Applications/cassh.app` | The menu bar app |
| `~/Library/Application Support/cassh/cassh.policy.toml` | Policy config |
| `~/Library/LaunchAgents/com.shawntz.cassh.plist` | Auto-start agent |

### Deploy via MDM

1. Upload the PKG to your MDM (Jamf, Kandji, Mosyle, etc.)
2. Create a policy to deploy to target machines
3. The app will auto-start on user login

---

## Manual Distribution

For smaller deployments or testing:

### DMG Installation

1. Download the DMG from [Releases](https://github.com/shawntz/cassh/releases)
2. Open the DMG
3. Drag cassh to Applications
4. Configure the client policy (see below)

### Client Configuration

Create the policy file:

```bash
mkdir -p ~/Library/Application\ Support/cassh
cat > ~/Library/Application\ Support/cassh/cassh.policy.toml << 'EOF'
server_base_url = "https://cassh.yourcompany.com"
github_enterprise_url = "https://github.yourcompany.com"
EOF
```

The `github_enterprise_url` is required for automatic SSH config setup. When you generate a certificate, `cassh` will automatically add the correct SSH config entry for your GitHub Enterprise instance.

### Auto-Start on Login

To start `cassh` automatically:

```bash
# Install the LaunchAgent
make install-launchagent
```

Or manually:

```bash
mkdir -p ~/Library/LaunchAgents
cat > ~/Library/LaunchAgents/com.shawntz.cassh.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.shawntz.cassh</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Applications/cassh.app/Contents/MacOS/cassh</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
EOF

launchctl load ~/Library/LaunchAgents/com.shawntz.cassh.plist
```

---

## CLI for Servers/CI

For headless environments (Linux servers, CI pipelines), use the CLI:

```bash
# Build CLI
make cli

# Generate certificate
./cassh --server https://cassh.yourcompany.com

# With custom key path
./cassh --server https://cassh.yourcompany.com --key ~/.ssh/my_key
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Get SSH Certificate
  run: |
    curl -sSL https://github.com/shawntz/cassh/releases/download/v0.1.0/cassh-linux-amd64 -o cassh
    chmod +x cassh
    ./cassh --server ${{ secrets.CASSH_SERVER }} --token ${{ secrets.CASSH_TOKEN }}
```

---

## User Guide

Share this with your users:

### First Time Setup

1. Look for the **terminal icon** in your menu bar (top-right)
2. Click it to see the dropdown menu
3. Select **"Generate / Renew Cert"**
4. Complete SSO login in your browser
5. Status will show green when active

### Daily Usage

- **Green status** = Certificate valid, you can push/pull
- **Yellow status** = Certificate expiring soon
- **Red status** = Certificate expired, click to renew

Certificates are valid for 12 hours. The app will notify you before expiration.

### Troubleshooting

| Issue | Solution |
|-------|----------|
| Red status won't go green | Click "Generate / Renew Cert" and complete SSO |
| Browser doesn't open | Check if default browser is set |
| "Server unreachable" | Check network/VPN connection |
| SSH still fails | Run `ssh-add -l` to verify cert is loaded |

---

## Git SSH Configuration

`cassh` automatically configures your SSH config when you generate a certificate (if `github_enterprise_url` is set in your policy). However, you also need to ensure your Git repositories are using SSH URLs.

### Check Your Remote URL

```bash
git remote -v
```

If you see `https://github.yourcompany.com/...`, you need to switch to SSH.

### Switch to SSH URL

```bash
# For GitHub Enterprise
git remote set-url origin git@github.yourcompany.com:org/repo.git
```

### What cassh Configures Automatically

When you generate a certificate, `cassh` adds an entry to `~/.ssh/config`:

```
Host github.yourcompany.com
    HostName github.yourcompany.com
    User git
    IdentityFile ~/.ssh/cassh_id_ed25519
    IdentitiesOnly yes
```

This ensures Git uses your cassh certificate for authentication.

### Verify SSH Connection

```bash
# Test SSH connection to GitHub Enterprise
ssh -T git@github.yourcompany.com

# Verify cert is loaded in ssh-agent
ssh-add -l
```

You should see your key with `[CERT]` next to it.

### System Notifications

cassh sends macOS notifications for:

- **Certificate Activated** - When a new certificate is installed
- **Certificate Expiring** - When your certificate has less than 1 hour remaining
- **Certificate Expired** - When your certificate has expired

Click the menu bar icon to renew your certificate when notified.
