# GitLab Support

cassh now supports both GitHub and GitLab, in both personal and enterprise modes.

> **⚠️ WORK IN PROGRESS**: GitLab support is currently under active development. Core functionality is implemented, but UI integration is not yet complete. See [Known Limitations](#known-limitations) below for details.

## Overview

GitLab support works the same way as GitHub support:

| Mode | GitLab Personal | GitLab Enterprise |
|------|-----------------|-------------------|
| **Authentication** | GitLab Personal Access Token | OIDC (via cassh server) |
| **Key/Cert Type** | SSH Key (via GitLab API) | SSH Certificate (CA-signed) |
| **Validity** | User-defined rotation policy | 12 hours (configurable) |
| **Dependencies** | GitLab API token | cassh-server, OIDC provider |

## Configuration

### Personal GitLab Connection

```toml
[[connections]]
id = "personal-gitlab"
platform = "gitlab"
type = "personal"
name = "Personal GitLab"
host = "gitlab.com"
username = "yourusername"
ssh_key_path = "~/.ssh/cassh_gitlab_id_ed25519"
key_rotation_hours = 168  # 7 days
gitlab_token = "glpat-xxxxxxxxxxxxxxxxxxxx"  # Personal Access Token
```

### GitLab Enterprise Connection

```toml
[[connections]]
id = "work-gitlab"
platform = "gitlab"
type = "enterprise"
name = "Work GitLab"
server_url = "https://cassh.yourcompany.com"
host = "gitlab.yourcompany.com"
username = "jdoe"
ssh_key_path = "~/.ssh/cassh_gitlab_work_id_ed25519"
ssh_cert_path = "~/.ssh/cassh_gitlab_work_id_ed25519-cert.pub"
```

## GitLab Personal Mode

### Prerequisites

1. GitLab account (gitlab.com or self-hosted)
2. Personal Access Token with `api` scope

### Creating a Personal Access Token

1. Go to GitLab → Preferences → Access Tokens
2. Create a new token with the following scopes:
   - `api` (required for SSH key management; note that this scope grants full read/write access to the GitLab API according to your account permissions, not just SSH key operations)
3. Copy the token and save it in your cassh config, storing it securely (for example, in an encrypted secrets store) and treating it as a high‑privilege credential

> **Note:** GitLab currently exposes SSH key management only under the `api` scope. This is why cassh requires a token with `api` scope, even though this scope is broader than strictly necessary for SSH key management.
### How It Works

1. **Key Generation**: cassh generates an Ed25519 SSH key
2. **Upload**: Key is uploaded to GitLab via the API
3. **Rotation**: When rotation is due:
   - Old key is deleted from GitLab
   - New key is generated locally
   - New key is uploaded to GitLab
   - Config is updated with new key ID and timestamp

### SSH Key Management

The GitLab API client (`internal/gitlab/client.go`) provides:

- `ListSSHKeys()` - List all SSH keys for authenticated user
- `GetSSHKeyByTitle(title)` - Find a key by its title
- `CreateSSHKey(title, publicKey, expiresAt)` - Add new SSH key
- `DeleteSSHKey(keyID)` - Remove SSH key by ID
- `GetCurrentUser()` - Get authenticated user info
- `ValidateToken()` - Check if token is valid

### Rotation Logic

Same as GitHub personal mode:

1. Check if rotation is needed based on `key_rotation_hours`
2. If needed:
   - Delete old key from GitLab (using `gitlab_key_id`)
   - Delete local key files
   - Generate new Ed25519 key
   - Upload to GitLab
   - Update connection config with new key ID and timestamp

## GitLab Enterprise Mode

### Server Configuration

The cassh server supports GitLab Enterprise via the same OIDC authentication flow.

**Environment Variables:**

```bash
export CASSH_PLATFORM="gitlab"
export CASSH_GITLAB_ENTERPRISE_URL="https://gitlab.yourcompany.com"
export CASSH_GITLAB_PRINCIPAL_SOURCE="email_prefix"
export CASSH_CA_PRIVATE_KEY="$(cat ca_key)"
export CASSH_OIDC_CLIENT_ID="..."
export CASSH_OIDC_CLIENT_SECRET="..."
export CASSH_OIDC_TENANT="..."
```

**Policy TOML:**

```toml
server_base_url = "https://cassh.yourcompany.com"
cert_validity_hours = 12
platform = "gitlab"

[gitlab]
enterprise_url = "https://gitlab.yourcompany.com"
allowed_groups = ["engineering", "devops"]
principal_source = "email_prefix"  # email_prefix, email, or username

[ca]
private_key_path = "ca_key"

[oidc]
client_id = "..."
client_secret = "..."
tenant = "..."
```

### Certificate Generation

The server uses `SignPublicKeyForGitLab()` in `internal/ca/ca.go` to create certificates with:

- Standard SSH certificate extensions
- GitLab login extension: `login@gitlab.yourcompany.com=username`
- Configurable validity period (default: 12 hours)
- OIDC-derived principal (from email, username, or custom claim)

### Client Flow

1. User clicks "Generate / Renew" in menu bar
2. Browser opens to `https://cassh.yourcompany.com/?pubkey=...`
3. User authenticates via OIDC (Microsoft Entra, etc.)
4. Server signs certificate and returns via `cassh://install-cert` URL scheme
5. Certificate is installed to `~/.ssh/cassh_gitlab_work_id_ed25519-cert.pub`
6. Certificate is loaded into ssh-agent

## SSH Configuration

### GitLab.com (Personal)

```ssh
Host gitlab.com
    User git
    HostName gitlab.com
    IdentityFile ~/.ssh/cassh_gitlab_id_ed25519
    IdentitiesOnly yes
```

### GitLab Enterprise (Self-Hosted)

```ssh
Host gitlab.yourcompany.com
    User git
    HostName gitlab.yourcompany.com
    IdentityFile ~/.ssh/cassh_gitlab_work_id_ed25519
    CertificateFile ~/.ssh/cassh_gitlab_work_id_ed25519-cert.pub
    IdentitiesOnly yes
```

## API Reference

### GitLab API Client

Located in `internal/gitlab/client.go`:

```go
// Create client
client := gitlab.NewClient("https://gitlab.com", "glpat-xxxxxxxxxxxxxxxxxxxx")

// Validate token
err := client.ValidateToken()

// List SSH keys
keys, err := client.ListSSHKeys()

// Add SSH key
key, err := client.CreateSSHKey("cassh-key", pubKeyContent, nil)

// Delete SSH key
err := client.DeleteSSHKey(keyID)

// Get current user
user, err := client.GetCurrentUser()
```

### Certificate Signing

Located in `internal/ca/ca.go`:

```go
// Sign for GitLab
cert, err := ca.SignPublicKeyForGitLab(
    userPublicKey,
    "cassh:user@example.com:1234567890",
    "jdoe",                          // GitLab username
    "gitlab.yourcompany.com",        // GitLab hostname
)
```

## Security Considerations

### Personal Access Token Storage

**Current:** Tokens are stored in plaintext in `~/.config/cassh/config.toml`

**Future:** Consider using system keychain:
- macOS: Keychain Access
- Linux: gnome-keyring or secret-service
- Windows: Credential Manager

### Token Scopes

Personal Access Tokens should have **minimal required scopes**:
- `api` - For SSH key management only
- Do NOT grant `write_repository`, `sudo`, or other elevated permissions

### Token Rotation

GitLab PATs can have expiration dates. cassh should:
- Detect expired tokens (via API error responses)
- Notify user to refresh token
- Optionally support automatic token refresh (if using OAuth flow)

## Migration from GitHub

To migrate a GitHub connection to GitLab:

1. Update the `platform` field:
   ```toml
   platform = "gitlab"  # was: empty or "github"
   ```

2. Update the `host` field:
   ```toml
   host = "gitlab.com"  # was: "github.com"
   ```

3. For personal mode, add `gitlab_token`:
   ```toml
   gitlab_token = "glpat-xxxxxxxxxxxxxxxxxxxx"
   ```

4. Remove GitHub-specific fields:
   ```toml
   # Remove these:
   # github_key_id = "..."
   ```

5. Regenerate SSH key to upload to GitLab

## Backwards Compatibility

The configuration supports backwards compatibility:

- Empty `platform` field defaults to `"github"`
- Deprecated fields (`github_host`, `github_username`) are migrated to new fields (`host`, `username`)
- Migration happens automatically via `MigrateDeprecatedFields()`

## Troubleshooting

### Personal Mode Issues

**Error: "Failed to upload key"**
- Check that PAT has `api` scope
- Verify token is not expired
- Check GitLab instance is reachable

**Error: "Key already exists"**
- cassh will automatically find and use existing key
- Or manually delete old keys from GitLab UI

### Enterprise Mode Issues

**Error: "Certificate rejected"**
- Verify GitLab has the CA public key configured
- Check certificate principal matches GitLab username
- Verify certificate validity period

**Error: "OIDC authentication failed"**
- Check OIDC configuration in server config
- Verify redirect URL matches server URL
- Check OIDC tenant/client ID/secret are correct

## Known Limitations

> **Current Status**: The GitLab backend infrastructure is complete, but UI integration is still in progress.

### ✅ What's Working

**API & Infrastructure:**
- ✅ GitLab API client (`internal/gitlab/client.go`)
  - SSH key listing, creation, deletion
  - Token validation
  - User info retrieval
- ✅ Configuration support for GitLab connections
  - Personal and enterprise mode configs
  - Platform-agnostic connection management
  - Backwards compatibility with GitHub-only configs
- ✅ Certificate authority GitLab support
  - GitLab-specific SSH certificate signing
  - Proper certificate extensions for GitLab
  - OIDC-derived principals

**Manual Usage:**
- ✅ You can manually configure GitLab connections in `~/.config/cassh/config.toml`
- ✅ Server-side certificate signing works for GitLab Enterprise (OIDC flow)

### 🚧 Not Yet Implemented

**Menu Bar App:**
- ❌ GitLab personal mode integration (SSH key upload/rotation)
- ❌ GitLab connection status display in menu bar
- ❌ "Refresh Key" action for GitLab personal connections
- ❌ Automatic key rotation for GitLab personal mode
- ❌ GitLab token validation in UI

**Setup Wizard:**
- ❌ "Personal GitLab" option in setup wizard
- ❌ "Enterprise GitLab" option in setup wizard
- ❌ GitLab token input and validation UI
- ❌ GitLab URL parsing from clone URLs

**Server:**
- ❌ Platform detection from query parameters
- ❌ GitLab-specific landing page branding
- ❌ Multi-platform support in auth callback

**Additional Features:**
- ❌ GitLab SSH config auto-generation
- ❌ GitLab git config management (includeIf directives)
- ❌ Comprehensive test coverage for GitLab features

### 📋 Workaround (Until UI is Complete)

If you want to use GitLab support today, you can:

1. **Manually create the connection** in `~/.config/cassh/config.toml`:
   ```toml
   [[connections]]
   id = "personal-gitlab"
   platform = "gitlab"
   type = "personal"
   name = "Personal GitLab"
   host = "gitlab.com"
   username = "yourusername"
   ssh_key_path = "~/.ssh/cassh_gitlab_id_ed25519"
   key_rotation_hours = 168
   gitlab_token = "your-gitlab-personal-access-token-here"
   ```
   
   > **Security Note:** Replace `your-gitlab-personal-access-token-here` with your actual GitLab Personal Access Token. Keep this file secure and never commit it to version control.

2. **Manually generate and upload the SSH key**:
   ```bash
   # Generate key
   ssh-keygen -t ed25519 -f ~/.ssh/cassh_gitlab_id_ed25519 -N ""
   
   # Upload to GitLab (via UI or CLI)
   # GitLab UI: Settings → SSH Keys → Add new key
   # Or use the GitLab API directly
   ```

3. **Manually configure SSH config** (`~/.ssh/config`):
   ```ssh
   Host gitlab.com
       User git
       HostName gitlab.com
       IdentityFile ~/.ssh/cassh_gitlab_id_ed25519
       IdentitiesOnly yes
   ```

For GitLab Enterprise with certificates, the server-side infrastructure is ready, but you'll need to manually initiate the OIDC flow until the UI integration is complete.

### 📊 Implementation Status

For detailed implementation status and developer notes, see [GITLAB_IMPLEMENTATION_STATUS.md](../GITLAB_IMPLEMENTATION_STATUS.md) in the repository root.

## GitLab Resources

- [GitLab SSH Documentation](https://docs.gitlab.com/ee/user/ssh.html)
- [GitLab API - SSH Keys](https://docs.gitlab.com/ee/api/users.html#list-ssh-keys-for-user)
- [GitLab SSH Certificates](https://docs.gitlab.com/ee/user/ssh.html#ssh-certificates)
- [GitLab Personal Access Tokens](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
