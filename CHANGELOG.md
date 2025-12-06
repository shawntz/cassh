# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-12-05

#### Added

- **Automatic SSH config setup**: When generating a certificate, `cassh` now automatically adds the appropriate Host entry to `~/.ssh/config` for GitHub Enterprise
- **System notifications**: macOS notifications for certificate activation, expiring soon (< 1 hour), and expired states
- **GitHub Enterprise URL in policy**: Added `[github] enterprise_url` field to policy config for SSH config auto-setup
- **Build release script**: Added `scripts/build-release` one-liner script to build all packages after configuring policy
- **Web page footer**: Added footer to landing and success pages with GitHub, Docs, Sponsor links, and copyright
- **Setup CTA banner**: Added "Deploy `cassh` for your team" call-to-action on landing page linking to getting started guide

#### Core Features

- macOS menu bar app with certificate status indicator
- OIDC authentication with Microsoft Entra ID
- 12-hour SSH certificate signing (configurable)
- Web server with meme landing page (LSP & Flash Slothmore)
- Development mode for local testing with mock authentication

#### Menu Bar App

- Terminal icon with macOS template icon support (auto dark/light mode)
- Status indicators in dropdown menu (green/yellow/red)
- One-click certificate generation/renewal
- Auto SSH key generation (Ed25519)
- ssh-agent integration for certificate loading

#### Configuration

- Environment variable support for cloud deployment
- Split configuration model (policy vs user preferences)
- TOML-based configuration files
- Configurable certificate validity period

#### Deployment

- Dockerfile for containerized deployments
- `render.yaml` for Render.com infrastructure-as-code
- Makefile with comprehensive build targets
- Support for Fly.io, Railway, Render, and self-hosted VPS

#### Distribution

- DMG installer with drag-and-drop installation
- PKG installer for MDM deployment (Jamf, Kandji, etc.)
- macOS app bundle with embedded policy
- LaunchAgent for auto-start on login
- App icon (`cassh.icns`) generated from terminal SVG

#### Documentation

- MkDocs Material documentation site
- GitHub Pages deployment via GitHub Actions
- Comprehensive guides: getting started, server setup, deployment, client distribution
- Configuration reference with all options
- Security best practices and threat model
- Project roadmap with planned features

#### CI/CD

- GitHub Actions workflow for releases (triggered on `v*` tags)
- GitHub Actions workflow for documentation deployment
- Automated changelog parsing for release notes
- macOS DMG and Linux binary builds

#### Community

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
