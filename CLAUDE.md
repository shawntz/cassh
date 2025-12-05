# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`cassh` is an ephemeral SSH certificate system for GitHub Enterprise access. It issues time-bound SSH certificates (12-hour validity) signed by an internal CA, eliminating the need for permanent SSH keys and manual revocation.

## Build Commands

```bash
# Install dependencies
make deps

# Build all binaries
make build

# Build individual components
make server      # cassh-server (web + CA)
make menubar     # cassh-menubar (macOS tray app)
make cli         # cassh CLI (headless)

# Run tests
make test

# Lint
make lint

# Development server (requires dev CA key)
make dev-ca      # Generate dev CA key first
make dev-server  # Run server locally on :8080

# Build distributions
make build-oss           # OSS template (config editable)
make build-enterprise    # Enterprise (locked policy)

# macOS packaging (one-liner after configuring cassh.policy.toml)
./build-release.sh

# Or step-by-step:
make app-bundle   # Create cassh.app
sudo make dmg     # Create DMG installer (requires sudo)
make pkg          # Create PKG for MDM deployment
make sign         # Code sign (requires APPLE_DEVELOPER_ID)
make notarize     # Notarize for Gatekeeper

# Full release
make release      # Clean, test, lint, build all artifacts
```

## Architecture

```
cmd/
  cassh-server/       # Web server: landing page + OIDC + cert signing
    templates/        # HTML templates (embedded)
    static/           # Static assets (embedded)
  cassh-menubar/      # macOS menu bar app with loopback listener
  cassh-cli/          # Headless CLI for CI/servers

internal/
  config/             # Split config: policy (immutable) + user (editable)
  ca/                 # SSH certificate generation and parsing
  oidc/               # Microsoft Entra ID authentication
  memes/              # LSP and Sloth quote rotation

packaging/macos/      # macOS distribution files
  Info.plist.template
  entitlements.plist
  com.shawntz.cassh.plist  # LaunchAgent
  distribution.xml         # PKG installer
  scripts/                 # Install scripts
  resources/               # PKG resources (HTML)
```

## Configuration System

**Split config model** - policy cannot be overridden by users:

| Config | Location | Editable | Contains |
|--------|----------|----------|----------|
| Policy | `cassh.policy.toml` (bundled in app) | No | CA key, cert validity, OIDC settings, GHE URL |
| User | `~/Library/Application Support/cassh/config.toml` | Yes | UI prefs, refresh interval |

## Key Files

- `cassh.policy.toml` - Template policy config (fill in before enterprise build)
- `internal/config/config.go` - `LoadServerConfig()` loads config with env var overrides
- `internal/ca/ca.go` - `SignPublicKey()` - certificate signing logic
- `internal/oidc/oidc.go` - `StartAuth()` / `HandleCallback()` - Entra OIDC flow
- `cmd/cassh-menubar/main.go` - Menu bar app with loopback listener

## Environment Variables

**Server (for cloud deployment):**
- `CASSH_SERVER_URL` - Public server URL (e.g., `https://cassh.onrender.com`)
- `CASSH_OIDC_CLIENT_ID` - Entra app client ID
- `CASSH_OIDC_CLIENT_SECRET` - Entra app client secret
- `CASSH_OIDC_TENANT` - Entra tenant ID
- `CASSH_OIDC_REDIRECT_URL` - OAuth callback URL (default: `{server_url}/auth/callback`)
- `CASSH_CA_PRIVATE_KEY` - CA private key content (for cloud, paste full key)
- `CASSH_CA_PRIVATE_KEY_PATH` - Path to CA private key file (for local/VPS)
- `CASSH_CERT_VALIDITY_HOURS` - Certificate validity (default: `12`)
- `CASSH_DEV_MODE` - Set to `true` to skip OIDC, use mock auth
- `CASSH_LISTEN_ADDR` - Listen address (default: `:8080`)
- `CASSH_POLICY_PATH` - Path to policy TOML (default: `cassh.policy.toml`)

**CLI:**
- `CASSH_SERVER` - Server URL for cert generation

## GitHub Actions Secrets (for releases)

- `APPLE_CERTIFICATE_BASE64` - Code signing certificate (p12, base64)
- `APPLE_CERTIFICATE_PASSWORD` - Certificate password
- `APPLE_DEVELOPER_ID` - Developer ID name
- `APPLE_ID` - Apple ID for notarization
- `APPLE_APP_PASSWORD` - App-specific password
- `APPLE_TEAM_ID` - Team ID

## Auth Flow

1. User clicks terminal icon in menu bar, selects "Generate / Renew Cert"
2. Menubar ensures `~/.ssh/cassh_id_ed25519` exists, reads pubkey
3. Opens browser to `https://cassh.example.com/?pubkey=<pubkey>`
4. Landing page shows LSP or Sloth meme with SSO button
5. User clicks SSO → Entra OIDC flow
6. On callback, server signs cert with CA
7. Browser can POST cert to loopback listener for auto-install
8. Menubar writes cert to `~/.ssh/cassh_id_ed25519-cert.pub`, runs `ssh-add`
9. Dropdown menu shows green status indicator

## Static Assets

Place meme images in `cmd/cassh-server/static/images/`:
- `lsp.png` - Lumpy Space Princess
- `sloth.png` - Flash from Zootopia

Templates fall back to emoji if images missing.
