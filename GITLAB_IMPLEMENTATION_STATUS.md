# GitLab Implementation Status

This document tracks the implementation of GitLab support in cassh.

## Change Type Classification

**Primary Type:** New Feature (non-breaking)

This PR introduces GitLab platform support alongside existing GitHub functionality. The implementation is designed with the following characteristics:

### ✅ New Feature
- Adds comprehensive GitLab API client (`internal/gitlab/client.go`)
- Introduces platform-agnostic configuration structure
- Implements GitLab SSH certificate signing
- Provides GitLab-specific documentation

### ✅ Refactoring (Backwards Compatible)
- Migrates `GitHubHost` → `Host` and `GitHubUsername` → `Username` in config structs
- Implements automatic migration via `MigrateDeprecatedFields()` to preserve existing configs
- Adds helper methods (`GetHost()`, `GetUsername()`) that handle both old and new field names
- Maintains full backwards compatibility - existing GitHub-only configurations continue to work without modification

### ❌ Not a Bug Fix
- Does not address any existing issues or defects
- No corrective changes to broken functionality

### ❌ Not a Breaking Change
- **Backwards compatible:** Old `GitHubHost` and `GitHubUsername` fields are automatically migrated
- **Default behavior preserved:** Platform defaults to `github` when not specified
- **Existing workflows unchanged:** All GitHub functionality continues to work identically
- **No API changes:** Server endpoints and CLI interface remain the same

The refactoring aspect involves restructuring configuration fields to be platform-agnostic, but the automatic migration and helper methods ensure zero breaking changes for existing users.

## ✅ Completed

### 1. GitLab API Client (`internal/gitlab/client.go`)
- [x] HTTP client with authentication
- [x] `NewClient(baseURL, token)` - Create GitLab API client
- [x] `ListSSHKeys()` - List all SSH keys
- [x] `GetSSHKeyByTitle(title)` - Find key by title
- [x] `CreateSSHKey(title, publicKey, expiresAt)` - Add SSH key
- [x] `DeleteSSHKey(keyID)` - Remove SSH key
- [x] `GetCurrentUser()` - Get authenticated user info
- [x] `ValidateToken()` - Check token validity
- [x] `ExtractHostFromURL(url)` - Parse GitLab URL

### 2. Configuration Updates (`internal/config/config.go`)
- [x] Added `Platform` type (`github` or `gitlab`)
- [x] Updated `Connection` struct with:
  - [x] `Platform` field
  - [x] `Host` field (replaces `GitHubHost`)
  - [x] `Username` field (replaces `GitHubUsername`)
  - [x] `GitLabKeyID` field (integer ID)
  - [x] `GitLabToken` field (Personal Access Token)
- [x] Added helper methods:
  - [x] `IsGitHub()` - Check if GitHub connection
  - [x] `IsGitLab()` - Check if GitLab connection
  - [x] `GetHost()` - Get platform host (with backwards compat)
  - [x] `GetUsername()` - Get username (with backwards compat)
  - [x] `MigrateDeprecatedFields()` - Migrate old configs
- [x] Updated `PolicyConfig` struct:
  - [x] `Platform` field
  - [x] `GitLabEnterpriseURL` field
- [x] Updated `ServerConfig` struct:
  - [x] `Platform` field
  - [x] `GitLabEnterpriseURL` field
  - [x] `GitLabAllowedGroups` field
  - [x] `GitLabPrincipalSource` field
- [x] Updated `LoadServerConfig()` to support:
  - [x] File-based GitLab configuration
  - [x] Environment variables: `CASSH_PLATFORM`, `CASSH_GITLAB_ENTERPRISE_URL`, `CASSH_GITLAB_PRINCIPAL_SOURCE`

### 3. Certificate Authority Updates (`internal/ca/ca.go`)
- [x] `SignPublicKeyForGitLab(pubKey, keyID, username, host)` - Sign certs for GitLab
- [x] GitLab-specific certificate extensions
- [x] Login extension format: `login@gitlab.com=username` or `login@gitlab.yourcompany.com=username`

### 4. Documentation (`docs/gitlab-support.md`)
- [x] Overview of GitLab personal vs enterprise modes
- [x] Configuration examples
- [x] Personal Access Token setup guide
- [x] SSH key rotation logic documentation
- [x] Server configuration for GitLab Enterprise
- [x] Certificate generation flow
- [x] SSH config examples
- [x] API reference
- [x] Security considerations
- [x] Migration guide from GitHub
- [x] Troubleshooting guide

## 🚧 Remaining Work

### 5. Menu Bar App GitLab Integration (`cmd/cassh-menubar/main.go`)

**Priority: HIGH**

The menubar app needs GitLab-specific functionality parallel to the existing GitHub code.

#### 5.1 GitLab Token Validation (lines ~2000-2100)
Add functions similar to `checkGHAuth()`:
```go
func checkGitLabToken(conn *config.Connection) GitLabAuthStatus {
    // Validate token using gitlab.Client.ValidateToken()
    // Return status with username, token validity, etc.
}
```

#### 5.2 SSH Key Generation for GitLab Personal (lines ~2100-2200)
Update `generateSSHKeyForPersonal()` or create GitLab-specific version:
```go
func generateSSHKeyForGitLabPersonal(conn *config.Connection) error {
    // Generate Ed25519 key
    // Save to conn.SSHKeyPath
    // Read public key
    // Upload via gitlab.Client.CreateSSHKey()
    // Save key ID to conn.GitLabKeyID
}
```

#### 5.3 SSH Key Upload to GitLab (new function)
```go
func uploadSSHKeyToGitLab(conn *config.Connection, keyPath string, title string) (int, error) {
    client, err := gitlab.NewClient(getGitLabURL(conn), conn.GitLabToken)
    if err != nil {
        return 0, err
    }
    pubKeyPath := keyPath + ".pub"
    pubKeyData, err := os.ReadFile(pubKeyPath)
    // ...
    key, err := client.CreateSSHKey(title, string(pubKeyData), nil)
    return key.ID, err
}
```

#### 5.4 SSH Key Deletion from GitLab (new function)
```go
func deleteSSHKeyFromGitLab(conn *config.Connection, keyID int) error {
    client, err := gitlab.NewClient(getGitLabURL(conn), conn.GitLabToken)
    if err != nil {
        return err
    }
    return client.DeleteSSHKey(keyID)
}
```

#### 5.5 Key Rotation for GitLab Personal (lines ~2200-2300)
Update `rotatePersonalGitHubSSH()` or create GitLab version:
```go
func rotateGitLabPersonalSSH(conn *config.Connection) error {
    // 1. Delete old key from GitLab using conn.GitLabKeyID
    // 2. Delete local key files
    // 3. Generate new key
    // 4. Upload new key to GitLab
    // 5. Update conn.GitLabKeyID and conn.KeyCreatedAt
    // 6. Save config
}
```

#### 5.6 Automatic Rotation Check (lines ~2270-2312)
Update `checkAndRotateExpiredKeys()` to handle GitLab:
```go
func checkAndRotateExpiredKeys() {
    for i := range cfg.User.Connections {
        conn := &cfg.User.Connections[i]

        if conn.Type != config.ConnectionTypePersonal {
            continue
        }

        if conn.IsGitHub() {
            // Existing GitHub logic...
        } else if conn.IsGitLab() {
            // New GitLab logic
            if needsKeyRotation(conn) {
                rotateGitLabPersonalSSH(conn)
            }
        }
    }
}
```

#### 5.7 Refresh Key Action (lines ~481-507)
Update `refreshKeyForConnection()` to handle GitLab:
```go
func refreshKeyForConnection(connIdx int) {
    conn := &cfg.User.Connections[connIdx]

    if conn.IsGitHub() {
        // Existing GitHub logic
    } else if conn.IsGitLab() {
        // Validate token
        if conn.GitLabToken == "" {
            showNotification("GitLab token missing", ...)
            return
        }

        // Rotate key
        if err := rotateGitLabPersonalSSH(conn); err != nil {
            log.Printf("Failed to rotate GitLab key: %v", err)
            return
        }

        showNotification("GitLab SSH Key Refreshed", ...)
    }
}
```

#### 5.8 Connection Status Update (lines ~510-616)
Update `updateConnectionStatus()` to display GitLab status:
```go
func updateConnectionStatus(connIdx int) {
    conn := &cfg.User.Connections[connIdx]

    if conn.Type == config.ConnectionTypeEnterprise {
        // Existing enterprise logic (works for both platforms)
    } else {
        // Personal mode
        if conn.IsGitHub() {
            // Existing GitHub logic
        } else if conn.IsGitLab() {
            // Check if token is valid
            client, err := gitlab.NewClient(getGitLabURL(conn), conn.GitLabToken)
            if err != nil {
                setConnectionStatusInvalid(connIdx, "Invalid GitLab token", true)
                return
            }
            if err := client.ValidateToken(); err != nil {
                setConnectionStatusInvalid(connIdx, "Invalid GitLab token", true)
                return
            }

            // Display rotation status
            if conn.KeyRotationHours > 0 && conn.KeyCreatedAt > 0 {
                // Calculate time until rotation
                // Display status with countdown
            }
        }
    }
}
```

#### 5.9 Certificate Generation for GitLab Enterprise (lines ~448-478)
Update `generateCertForConnection()` to support both platforms:
```go
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

    // Build URL to cassh server (same for both platforms)
    authURL := fmt.Sprintf("%s/?pubkey=%s&platform=%s",
        conn.ServerURL,
        url.QueryEscape(string(pubKeyData)),
        conn.Platform,
    )

    // Open in browser
    if runtime.GOOS == "darwin" {
        openNativeWebView(authURL, fmt.Sprintf("Sign in - %s", conn.Name), 800, 700)
    }
}
```

#### 5.10 Helper Functions
```go
func getGitLabURL(conn *config.Connection) string {
    host := conn.GetHost()
    if host == "gitlab.com" {
        return "https://gitlab.com"
    }
    return "https://" + host
}

func getGitLabUsername(conn *config.Connection) string {
    if conn.Username != "" {
        return conn.Username
    }

    // Try to get from API
    client, err := gitlab.NewClient(getGitLabURL(conn), conn.GitLabToken)
    if err != nil {
        return ""
    }
    user, err := client.GetCurrentUser()
    if err != nil {
        return ""
    }

    if username, ok := user["username"].(string); ok {
        conn.Username = username
        config.SaveUserConfig(&cfg.User)
        return username
    }

    return ""
}
```

### 6. Server Updates (`cmd/cassh-server/main.go`)

**Priority: MEDIUM**

#### 6.1 Platform Detection (lines ~293-358)
Update `handleAuthCallback()` to detect platform:
```go
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
    // ... existing OIDC auth ...

    // Detect platform from query param or server config
    platform := r.URL.Query().Get("platform")
    if platform == "" {
        platform = s.config.Platform
    }
    if platform == "" {
        platform = "github" // default
    }

    // Sign cert based on platform
    var cert *ssh.Certificate
    var err error

    if platform == "gitlab" {
        gitlabHost := config.ExtractHostFromURL(s.config.GitLabEnterpriseURL)
        cert, err = s.ca.SignPublicKeyForGitLab(sshPubKey, keyID, principal, gitlabHost)
    } else {
        githubHost := config.ExtractHostFromURL(s.config.GitHubEnterpriseURL)
        cert, err = s.ca.SignPublicKeyForGitHub(sshPubKey, keyID, principal, githubHost)
    }

    if err != nil {
        http.Error(w, "Failed to generate certificate", http.StatusInternalServerError)
        return
    }

    // ... rest of handler ...
}
```

#### 6.2 Landing Page Updates
Update landing page template to detect platform and show appropriate branding.

### 7. Setup Wizard Updates (`cmd/cassh-menubar/webview_darwin.go` + HTML templates)

**Priority: MEDIUM**

#### 7.1 Add GitLab Option to Setup Wizard
- Add "Personal GitLab" and "Enterprise GitLab" options
- Collect GitLab-specific fields:
  - GitLab URL (defaults to gitlab.com)
  - Personal Access Token (for personal mode)
  - Username (auto-detected from token)

#### 7.2 GitLab Personal Setup Flow
```javascript
// In setup wizard JavaScript
async function setupGitLabPersonal() {
    const token = document.getElementById('gitlab-token').value;
    const url = document.getElementById('gitlab-url').value || 'https://gitlab.com';

    // Validate token via API
    const response = await fetch(`${url}/api/v4/user`, {
        headers: { 'PRIVATE-TOKEN': token }
    });

    if (!response.ok) {
        showError('Invalid GitLab token');
        return;
    }

    const user = await response.json();

    // Create connection config
    const connection = {
        id: `gitlab-personal-${Date.now()}`,
        platform: 'gitlab',
        type: 'personal',
        name: 'Personal GitLab',
        host: extractHost(url),
        username: user.username,
        gitlab_token: token,
        ssh_key_path: `~/.ssh/cassh_gitlab_${user.username}_id_ed25519`,
        key_rotation_hours: 168  // 7 days default
    };

    // Save via backend API
    await fetch('http://localhost:8765/api/add-connection', {
        method: 'POST',
        body: JSON.stringify(connection)
    });

    showSuccess('GitLab connection added!');
}
```

### 8. SSH Config Management

**Priority: LOW**

Update SSH config generation to handle GitLab:
- `Host gitlab.com` for personal
- `Host gitlab.yourcompany.com` for enterprise
- Correct `User` field (always `git` for GitLab)

### 9. Git Config Management

**Priority: LOW**

Update `includeIf` directives to match GitLab remotes:
```ini
[includeIf "hasconfig:remote.*.url:git@gitlab.com:**"]
    path = /Users/user/.config/cassh/gitconfig-gitlab-personal
```

### 10. Testing

**Priority: HIGH**

Create tests in `internal/gitlab/client_test.go`:
- [x] Mock GitLab API responses
- [x] Test SSH key creation
- [x] Test SSH key deletion
- [x] Test token validation
- [x] Test error handling

Create tests in `internal/ca/ca_test.go`:
- [ ] Test GitLab certificate generation
- [ ] Test certificate extensions
- [ ] Test certificate validity

### 11. Update CLAUDE.md

**Priority: LOW**

Document GitLab support in the project overview:
- Add GitLab to architecture overview
- Update configuration examples
- Add GitLab-specific environment variables

## Implementation Priority

1. **Phase 1 (Critical):**
   - [x] GitLab API client
   - [x] Configuration structs
   - [x] CA certificate signing
   - [ ] Menu bar GitLab personal mode (5.1-5.7)

2. **Phase 2 (Important):**
   - [ ] Server platform detection (6.1)
   - [ ] Setup wizard GitLab support (7.1-7.2)
   - [ ] Connection status display (5.8)

3. **Phase 3 (Nice to have):**
   - [ ] SSH config management (8)
   - [ ] Git config management (9)
   - [ ] Tests (10)
   - [ ] Documentation updates (11)

## Testing Checklist

### GitLab Personal Mode
- [ ] Token validation
- [ ] SSH key generation
- [ ] SSH key upload to GitLab
- [ ] SSH key rotation
- [ ] Automatic rotation on startup
- [ ] Manual refresh via menu bar
- [ ] Status indicator (green/yellow/red)
- [ ] Notification on key refresh

### GitLab Enterprise Mode
- [ ] Certificate generation via OIDC
- [ ] Certificate installation
- [ ] Certificate loading into ssh-agent
- [ ] Certificate status display
- [ ] Certificate expiration warning
- [ ] Certificate renewal

### Both Modes
- [ ] SSH config generation
- [ ] Git config management
- [ ] Multi-connection support (GitHub + GitLab)
- [ ] Backwards compatibility with GitHub-only configs
- [ ] Migration from deprecated fields

## File Modifications Summary

| File | Status | Changes |
|------|--------|---------|
| `internal/gitlab/client.go` | ✅ Complete | New file - GitLab API client |
| `internal/config/config.go` | ✅ Complete | Platform support, GitLab fields |
| `internal/ca/ca.go` | ✅ Complete | GitLab certificate signing |
| `cmd/cassh-menubar/main.go` | 🚧 In Progress | GitLab personal mode integration |
| `cmd/cassh-server/main.go` | 🚧 Pending | Platform detection in auth flow |
| `cmd/cassh-menubar/webview_darwin.go` | 🚧 Pending | Setup wizard GitLab support |
| `docs/gitlab-support.md` | ✅ Complete | GitLab documentation |
| `CLAUDE.md` | 🚧 Pending | Architecture updates |

## Next Steps

1. Implement menu bar GitLab personal mode (Section 5.1-5.7)
2. Test with a real GitLab account
3. Add server-side platform detection (Section 6.1)
4. Update setup wizard for GitLab (Section 7.1-7.2)
5. Write tests for GitLab functionality
6. Update documentation with examples
