# Configuration Reference

`cassh` uses a split configuration model:

- **Policy Config** - IT-controlled settings (bundled in app or from env vars)
- **User Config** - Personal preferences (editable by user)

## Environment Variables

Environment variables take precedence over file configuration.

### Server Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `CASSH_SERVER_URL` | Public server URL | Yes | - |
| `CASSH_OIDC_CLIENT_ID` | Entra app client ID | Yes* | - |
| `CASSH_OIDC_CLIENT_SECRET` | Entra app client secret | Yes* | - |
| `CASSH_OIDC_TENANT` | Entra tenant ID | Yes* | - |
| `CASSH_OIDC_REDIRECT_URL` | OAuth callback URL | No | `{server_url}/auth/callback` |
| `CASSH_CA_PRIVATE_KEY` | CA private key content | Yes** | - |
| `CASSH_CA_PRIVATE_KEY_PATH` | Path to CA private key file | Yes** | - |
| `CASSH_CERT_VALIDITY_HOURS` | Certificate lifetime in hours | No | `12` |
| `CASSH_LISTEN_ADDR` | Server listen address | No | `:8080` |
| `CASSH_DEV_MODE` | Enable development mode | No | `false` |
| `CASSH_POLICY_PATH` | Path to policy TOML file | No | `cassh.policy.toml` |

*Required in production mode
**One of these is required in production mode

### Development Mode

Set `CASSH_DEV_MODE=true` to:

- Skip OIDC authentication (mock auth)
- Use local CA key for testing
- Enable verbose logging

---

## Policy Config (TOML)

The policy TOML file contains IT-controlled settings.

### Server Configuration

```toml
# Server settings
server_base_url = "https://cassh.yourcompany.com"
cert_validity_hours = 12
dev_mode = false

# OIDC / Microsoft Entra
[oidc]
client_id = "your-client-id"
client_secret = "your-client-secret"
tenant = "your-tenant-id"
redirect_url = "https://cassh.yourcompany.com/auth/callback"

# Certificate Authority
[ca]
private_key_path = "./ca_key"

# GitHub Enterprise
[github]
enterprise_url = "https://github.yourcompany.com"
allowed_orgs = ["your-org"]  # Optional: restrict to specific orgs
```

### Client Configuration

Clients only need the server URL:

```toml
server_base_url = "https://cassh.yourcompany.com"
```

### Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `server_base_url` | string | Public URL of cassh server |
| `cert_validity_hours` | int | Certificate lifetime (default: 12) |
| `dev_mode` | bool | Enable development mode |
| `oidc.client_id` | string | Entra application client ID |
| `oidc.client_secret` | string | Entra application secret |
| `oidc.tenant` | string | Entra tenant ID |
| `oidc.redirect_url` | string | OAuth callback URL |
| `ca.private_key_path` | string | Path to CA private key file |
| `github.enterprise_url` | string | GitHub Enterprise base URL |
| `github.allowed_orgs` | []string | Restrict access to these orgs |

---

## User Config

The user config stores your connections (GitHub accounts) and UI preferences. This is the primary file you'll want to back up.

### Config Locations

cassh checks for user config in this order:

1. **Dotfiles location** (recommended): `~/.config/cassh/config.toml`
2. **Platform-specific** (fallback):
   - macOS: `~/Library/Application Support/cassh/config.toml`
   - Linux: `~/.config/cassh/config.toml`

If the dotfiles location exists, it takes precedence. This makes it easy to back up your cassh configuration as part of your dotfiles.

### Migrating to Dotfiles

To migrate your existing config to the dotfiles location:

```bash
# Create the dotfiles directory
mkdir -p ~/.config/cassh

# Copy existing config (macOS)
cp ~/Library/Application\ Support/cassh/config.toml ~/.config/cassh/config.toml

# Or start fresh with the example
curl -o ~/.config/cassh/config.toml \
  https://raw.githubusercontent.com/shawntz/cassh/main/config.example.toml
```

Once the file exists at `~/.config/cassh/config.toml`, cassh will use it automatically.

### Example Configuration

Here's a complete example configuration with both enterprise and personal connections:

```toml
# cassh user configuration
# Location: ~/.config/cassh/config.toml (recommended for dotfiles backup)
# Or: ~/Library/Application Support/cassh/config.toml (macOS default)

# UI preferences
refresh_interval_seconds = 30
notification_sound = true
preferred_meme = "random"  # "lsp", "sloth", or "random"

# Connections - add your GitHub accounts here
# Each connection can be either "enterprise" (certificate-based) or "personal" (key-based)

[[connections]]
id = "enterprise-work"
type = "enterprise"
name = "Work GitHub"
server_url = "https://cassh.yourcompany.com"
github_host = "github.yourcompany.com"
ssh_key_path = "~/.ssh/cassh_work_id_ed25519"
ssh_cert_path = "~/.ssh/cassh_work_id_ed25519-cert.pub"

[[connections]]
id = "personal-github"
type = "personal"
name = "Personal GitHub"
github_host = "github.com"
github_username = "yourusername"
ssh_key_path = "~/.ssh/cassh_personal_id_ed25519"
key_rotation_hours = 168  # Rotate key every 7 days (0 = no rotation)
# key_created_at and github_key_id are managed automatically
```

### Connection Types

#### Enterprise Connections

Enterprise connections use SSH certificates signed by your organization's CA:

```toml
[[connections]]
id = "enterprise-work"           # Unique identifier
type = "enterprise"              # Must be "enterprise"
name = "Work GitHub"             # Display name in menu bar
server_url = "https://cassh.yourcompany.com"  # Your cassh server
github_host = "github.yourcompany.com"        # GitHub Enterprise hostname
ssh_key_path = "~/.ssh/cassh_work_id_ed25519"
ssh_cert_path = "~/.ssh/cassh_work_id_ed25519-cert.pub"
```

#### Personal Connections

Personal connections use SSH keys uploaded to GitHub.com via the `gh` CLI:

```toml
[[connections]]
id = "personal-github"           # Unique identifier
type = "personal"                # Must be "personal"
name = "Personal GitHub"         # Display name in menu bar
github_host = "github.com"       # Always "github.com" for personal
github_username = "yourusername" # Your GitHub username
ssh_key_path = "~/.ssh/cassh_personal_id_ed25519"
key_rotation_hours = 168         # Rotate every 7 days (0 = disable)
```

### Field Reference

#### Global Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `refresh_interval_seconds` | int | `30` | How often to check connection status |
| `notification_sound` | bool | `true` | Play sound on warnings |
| `preferred_meme` | string | `"random"` | Landing page character ("lsp", "sloth", "random") |

#### Connection Fields (All Types)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique identifier for the connection |
| `type` | string | Yes | `"enterprise"` or `"personal"` |
| `name` | string | Yes | Display name in menu bar |
| `github_host` | string | Yes | GitHub hostname (e.g., `github.com` or `github.yourcompany.com`) |
| `ssh_key_path` | string | Yes | Path to SSH private key |

#### Enterprise-Only Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `server_url` | string | Yes | URL of your cassh server |
| `ssh_cert_path` | string | Yes | Path to SSH certificate |

#### Personal-Only Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `github_username` | string | Yes | Your GitHub username |
| `key_rotation_hours` | int | No | Hours between key rotations (0 = disabled) |
| `key_created_at` | int | Auto | Unix timestamp of key creation |
| `github_key_id` | string | Auto | GitHub's ID for the uploaded key |

### Rotation Policies

For personal connections, you can configure automatic key rotation:

| Policy | Hours | Use Case |
|--------|-------|----------|
| 4 hours | `4` | Shared/public computers |
| 24 hours | `24` | Work laptop |
| 7 days | `168` | Personal machine (recommended) |
| 30 days | `720` | Low-risk environment |
| 90 days | `2160` | Maximum allowed |
| Disabled | `0` | No automatic rotation |

---

## Config File Locations Summary

### macOS

| Config | Location |
|--------|----------|
| User config (dotfiles) | `~/.config/cassh/config.toml` |
| User config (fallback) | `~/Library/Application Support/cassh/config.toml` |
| Policy (bundled) | `cassh.app/Contents/Resources/cassh.policy.toml` |
| Policy (fallback) | `./cassh.policy.toml` |
| SSH keys | `~/.ssh/cassh_*_id_ed25519` |
| SSH certs | `~/.ssh/cassh_*_id_ed25519-cert.pub` |

### Linux

| Config | Location |
|--------|----------|
| User config | `~/.config/cassh/config.toml` |
| Policy | `./cassh.policy.toml` or `CASSH_POLICY_PATH` |
| SSH keys | `~/.ssh/cassh_*_id_ed25519` |
| SSH certs | `~/.ssh/cassh_*_id_ed25519-cert.pub` |

---

## Config Precedence

1. **Environment variables** (highest priority)
2. **Dotfiles config** (`~/.config/cassh/config.toml`)
3. **Platform-specific config** (`~/Library/Application Support/cassh/config.toml`)
4. **Policy file** (TOML)
5. **Default values** (lowest priority)

For security-critical settings (CA key, OIDC secrets), policy always wins over user config.
