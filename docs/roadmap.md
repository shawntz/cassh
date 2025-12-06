# Roadmap

This page tracks the development roadmap for cassh. Features are organized by status and priority.

## Status Legend

| Icon | Meaning |
|------|---------|
| :white_check_mark: | Complete and released |
| :construction: | In active development |
| :memo: | Planned for future release |
| :bulb: | Under consideration |

---

## Completed :white_check_mark:

### Core Functionality

- [x] **SSH Certificate Signing** - Ed25519 CA signs user public keys
- [x] **12-hour Certificate Validity** - Configurable expiration time
- [x] **Microsoft Entra ID SSO** - OIDC authentication flow
- [x] **CSRF Protection** - State parameter validation
- [x] **Nonce Verification** - Replay attack prevention

### macOS Client

- [x] **Menu Bar App** - Native macOS status bar integration
- [x] **Certificate Status Indicator** - Visual feedback (green/yellow/red)
- [x] **One-click Renewal** - Browser-based authentication
- [x] **Auto SSH Key Generation** - Creates Ed25519 key if missing
- [x] **ssh-agent Integration** - Automatic certificate loading
- [x] **Setup Wizard** - First-run configuration for OSS users
- [x] **Multi-Connection Support** - Manage multiple GitHub accounts
- [x] **Personal GitHub.com Support** - SSH key management via `gh` CLI

### Server

- [x] **Meme Landing Page** - LSP and Sloth character rotation
- [x] **Development Mode** - Mock authentication for testing
- [x] **Environment Variable Config** - Cloud-friendly deployment
- [x] **Health Check Endpoint** - `/health` for load balancers
- [x] **Embedded Static Assets** - Single binary deployment

### Distribution

- [x] **PKG Installer** - MDM-compatible package and Homebrew support
- [x] **App Bundle** - Proper macOS app structure
- [x] **GitHub Actions Release** - Automated builds on tag with signing and notarization
- [x] **Homebrew Cask** - `brew install --cask cassh`

---

## In Progress :construction:

### Policy Integrity

- [ ] **VerifyPolicyIntegrity** - Verify cryptographic signature of policy files to prevent tampering

### CLI Client

- [ ] **Headless Authentication** - Token-based auth for CI/CD
- [ ] **Linux Support** - Native Linux binary

---

## Planned :memo:

### Multi-Platform Clients

| Platform | Priority | Notes |
|----------|----------|-------|
| Linux | High | GNOME/KDE system tray integration |
| Windows | Low | System tray app with similar UX |

### Enhanced Security

- [ ] **Group-based Access Policies** - Restrict by Entra groups
- [ ] **Certificate Revocation List** - Manual revocation capability
- [ ] **Hardware Key Support** - YubiKey/FIDO2 for CA signing
- [ ] **Audit Logging** - Structured logs for SIEM integration
- [ ] **mTLS for Server** - Client certificate authentication

### Notifications & Monitoring

- [ ] **Slack Integration** - Expiration reminders via Slack
- [ ] **Microsoft Teams Integration** - Teams notifications
- [ ] **Email Notifications** - Fallback notification method
- [ ] **Prometheus Metrics** - `/metrics` endpoint for monitoring

### Admin Features

- [ ] **Admin Dashboard** - Web UI for certificate management
- [ ] **User Activity Logs** - View certificate issuance history
- [ ] **Policy Editor** - Web-based policy configuration
- [ ] **Bulk Revocation** - Revoke all certs for a user

### Enterprise Features

- [ ] **Multi-CA Support** - Different CAs for different teams
- [ ] **SCIM Provisioning** - Automatic user sync from Entra
- [ ] **GitHub App Integration** - Fine-grained repo permissions
- [ ] **Okta Support** - Alternative to Entra ID
- [ ] **Google Workspace Support** - Google as identity provider

### Platform Extensibility

cassh is designed to be extensible beyond GitHub. Future platform support:

| Platform | Priority | Status | Notes |
|----------|----------|--------|-------|
| GitHub Enterprise | High | :white_check_mark: Complete | Full SSH certificate support |
| GitHub.com (Personal) | High | :white_check_mark: Complete | SSH key-based auth via `gh` CLI |
| GitLab | Medium | :memo: Planned | SSH certificates supported, needs OIDC integration |
| Bitbucket | Medium | :memo: Planned | SSH key management API available |
| Azure DevOps | Low | :bulb: Considering | SSH key support varies by plan |

#### GitLab Support Roadmap

GitLab supports SSH certificates with a user-scoped CA. Implementation would include:

- [ ] GitLab OIDC provider integration
- [ ] GitLab SSH CA configuration
- [ ] Support for self-hosted GitLab instances
- [ ] GitLab.com SaaS support

#### Bitbucket Support Roadmap

Bitbucket uses SSH keys (not certificates). Implementation would include:

- [ ] Bitbucket Cloud API integration for key management
- [ ] Bitbucket Data Center support
- [ ] Automatic key rotation

---

## Under Consideration :bulb:

These features are being evaluated but not yet committed to:

- **Certificate Templates** - Different validity periods per role
- **Geo-fencing** - Restrict certificate issuance by location
- **Device Trust** - Require managed/compliant devices
- **Offline Mode** - Generate certificates without network
- **SSH CA Rotation** - Automated CA key rotation workflow

---

## Contributing

Want to help implement a feature? Check out [CONTRIBUTING.md](https://github.com/shawntz/cassh/blob/main/CONTRIBUTING.md) for guidelines.

### Priority Features for Contributors

If you're looking to contribute, these are high-impact areas:

1. **CLI Client** - Expand headless functionality
2. **Linux Client** - Port menu bar app to GNOME/KDE system tray
3. **Group-based Policies** - Add Entra group filtering
4. **Prometheus Metrics** - Add observability

### Feature Requests

Have an idea not on the roadmap? [Open an issue](https://github.com/shawntz/cassh/issues/new?template=feature_request.md) with the feature request template.
