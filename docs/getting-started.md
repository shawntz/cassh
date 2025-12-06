# Getting Started

This guide will walk you through setting up `cassh` from scratch.

## Choose Your Use Case

`cassh` supports two modes of operation:

| Mode | For | Authentication | Server Required |
|------|-----|----------------|-----------------|
| **GitHub Enterprise** | Organizations | SSH Certificates (CA-signed) | Yes |
| **GitHub.com Personal** | Individuals | SSH Keys (via `gh` CLI) | No |

You can use both modes simultaneously - for example, enterprise access for work and personal access for side projects.

## Prerequisites

### For GitHub.com Personal Use

- **macOS** - Menu bar app is macOS-only (for now)
- **GitHub CLI (`gh`)** - [Install with Homebrew](https://cli.github.com/): `brew install gh`
- **Authenticated with `gh`** - Run `gh auth login` before using cassh

That's it! Download the PKG from [Releases](https://github.com/shawntz/cassh/releases) or install via Homebrew:

```bash
brew install --cask cassh
```

### For GitHub Enterprise Use

- **Go 1.21+** - [Download Go](https://golang.org/dl/) (for building)
- **Microsoft Entra ID tenant** - For SSO authentication
- **GitHub Enterprise instance** - Where your repos live
- **A server** - To run cassh-server (see [Deployment](deployment.md))

## Quick Start: Personal GitHub.com

1. **Install** via Homebrew or download PKG from [Releases](https://github.com/shawntz/cassh/releases):
   ```bash
   brew install --cask cassh
   ```
2. **Launch** cassh - the setup wizard opens automatically
3. **Add Personal Account** - Enter your GitHub username, choose rotation policy
4. **Done!** - cassh generates a key and uploads it via `gh` CLI


Your SSH config is automatically updated. Just `git clone` and go!

> **Tip:** For shared or work computers, use a shorter rotation policy (4-24 hours). For personal machines, 7-90 days is usually fine.

## Quick Start: Development (Enterprise)

For local development and testing:

```bash
# Clone the repository
git clone https://github.com/shawntz/cassh.git
cd cassh

# Install dependencies
make deps

# Generate a development CA key
make dev-ca

# Run the server in dev mode (mock auth)
make dev-server
```

The server starts at `http://localhost:8080` with mock authentication enabled.

## Quick Start: Production (Enterprise)

For production deployment:

1. **Generate CA keys** (see [Server Setup](server-setup.md#generate-ca-keys))
2. **Create Entra app** (see [Server Setup](server-setup.md#create-microsoft-entra-app))
3. **Configure server** (see [Server Setup](server-setup.md#server-configuration))
4. **Deploy** (see [Deployment](deployment.md))
5. **Distribute client** (see [Client Distribution](client.md))

## Project Structure

```
cassh/
├── cmd/
│   ├── cassh-server/    # Web server (OIDC + cert signing)
│   ├── cassh-menubar/   # macOS menu bar app
│   └── cassh-cli/       # Headless CLI
├── internal/
│   ├── ca/              # Certificate authority logic
│   ├── config/          # Configuration handling
│   ├── memes/           # Meme content for landing page
│   └── oidc/            # Microsoft Entra ID integration
├── packaging/
│   └── macos/           # macOS distribution files
└── docs/                # Documentation (you are here)
```

## Build Commands

```bash
# Build all binaries
make build

# Build individual components
make server      # cassh-server
make menubar     # cassh-menubar (macOS)
make cli         # cassh CLI

# Run tests
make test

# Build macOS app bundle
make app-bundle          # OSS build (setup wizard)
make app-bundle-enterprise  # Enterprise build (locked config)

# Create PKG installer
make pkg
```

## Next Steps

### For Personal Use
- Download and install - you're ready to go!
- See [Configuration Reference](configuration.md) for customization options

### For Enterprise Use
1. [Server Setup](server-setup.md) - Configure CA and Entra
2. [Deployment](deployment.md) - Deploy to production
3. [Client Distribution](client.md) - Distribute to users
