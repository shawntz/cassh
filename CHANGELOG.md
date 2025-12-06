# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-12-05

### Added

- **Personal GitHub.com support**: Manage SSH keys for personal GitHub accounts without a server
  - Automatic key generation (Ed25519)
  - Key upload via `gh` CLI integration
  - Configurable rotation policies (4 hours to 90 days)
  - Automatic SSH config management
- **Multi-account support**: Manage multiple GitHub accounts (enterprise + personal) from a single menu bar app
- **Setup wizard**: First-run configuration wizard for adding connections
- **Automatic SSH config setup**: Automatically configures `~/.ssh/config` for both enterprise and personal connections
- **System notifications**: macOS notifications for certificate/key activation, expiring soon, and expired states
- **GitHub Enterprise URL in policy**: Added `[github] enterprise_url` field to policy config for SSH config auto-setup
- **Build release script**: Added `scripts/build-release` one-liner script to build all packages after configuring policy
- **Web page footer**: Added footer to landing and success pages with GitHub, Docs, Sponsor links, and copyright
- **Setup CTA banner**: Added "Deploy `cassh` for your team" call-to-action on landing page linking to getting started guide

### Core Features

- macOS menu bar app with status indicator (green/yellow/red)
- **Enterprise mode**: SSH certificates signed by internal CA (12-hour default validity)
- **Personal mode**: SSH keys via `gh` CLI with automatic rotation
- OIDC authentication with Microsoft Entra ID (enterprise)
- Web server with meme landing page (LSP & Flash Slothmore)
- Development mode for local testing with mock authentication

### Menu Bar App

- Terminal icon with macOS template icon support (auto dark/light mode)
- Status indicators in dropdown menu (green/yellow/red)
- One-click certificate generation/renewal
- Auto SSH key generation (Ed25519)
- ssh-agent integration for certificate loading
- Multi-connection dropdown with individual status per account

### Configuration

- Environment variable support for cloud deployment
- Split configuration model (policy vs user preferences)
- TOML-based configuration files
- Configurable certificate validity period
- User-configurable key rotation policies for personal accounts

### Deployment

- Dockerfile for containerized deployments
- `render.yaml` for Render.com infrastructure-as-code
- Makefile with comprehensive build targets
- Support for Fly.io, Railway, Render, and self-hosted VPS

### Distribution

- PKG installer for MDM deployment and Homebrew (Jamf, Kandji, etc.)
- macOS app bundle with embedded policy
- LaunchAgent for auto-start on login
- Homebrew Cask support (`brew install --cask cassh`)
- App icon (`cassh.icns`)

### Documentation

- MkDocs Material documentation site
- GitHub Pages deployment via GitHub Actions
- Comprehensive guides: getting started, server setup, deployment, client distribution
- Configuration reference with all options
- Security best practices and threat model
- Project roadmap with planned features

### CI/CD

- GitHub Actions workflow for releases (triggered on `v*` tags)
- GitHub Actions workflow for documentation deployment
- Automated changelog parsing for release notes
- macOS PKG signing and notarization

### Community

- Apache 2.0 license
- Code of Conduct (Contributor Covenant 2.0)
- Contributing guidelines
- Security policy with vulnerability reporting
- Issue templates (bug report, feature request)
- Pull request template
- GitHub Sponsors funding configuration

### Security

- Split configuration model (IT policy vs user preferences)
- CSRF protection using cryptographic state parameter
- Nonce verification to prevent replay attacks
- Policy bundled in signed app bundles (enterprise mode)
- Sensitive values loaded from environment variables in production
- Loopback listener restricted to localhost connections

### Fixed

- golangci-lint v2 configuration: Added `//go:build darwin` build tag to menubar app to fix Linux CI builds
- CA private key parsing: Handle escaped newlines (`\n`) in environment variables for cloud deployments (Render, etc.)
