<div align="center">
  <h1>cassh</h1>
  
  <picture>
    <img src="docs/assets/logo.svg" alt="cassh logo" width="100" height="100">
  </picture>
  
  <p><strong>Ephemeral SSH Certificates for GitHub Enterprise</strong></p>

  <p>
    <code>cassh</code> is an ephemeral SSH certificate system designed for GitHub Enterprise access.<br />
    Developed for <a href="https://github.com/ghostehq" target="_blank">@ghostehq</a>. Inspired by internal tooling <a href="https://github.com/slackhq" target="_blank">@slackhq</a>.
  </p>

  <p>
    <a href="https://github.com/shawntz/cassh/releases"><img src="https://img.shields.io/github/v/release/shawntz/cassh" alt="Release"></a>
    <a href="https://github.com/shawntz/cassh/blob/main/LICENSE"><img src="https://img.shields.io/github/license/shawntz/cassh" alt="License"></a>
    <a href="https://shawnschwartz.com/cassh"><img src="https://img.shields.io/badge/docs-GitHub%20Pages-blue" alt="Documentation"></a>
  </p>
</div>

---





**How it works:**

1. Authenticate with Microsoft Entra (Azure AD)
2. Obtain a short-lived SSH certificate (12 hours)
3. Certificate auto-expires - no revocation needed

## Why `cassh`?

Permanent SSH keys are a liability. If a laptop is lost, stolen, or compromised:

- **With permanent keys:** Manual revocation required, often missed
- **With `cassh`:** Certificate expires automatically, zero action needed

## Features

- **Short-lived SSH certificates** - Signed by your internal CA, valid for 12 hours
- **Entra SSO Integration** - Sign in with your Microsoft identity
- **macOS Menu Bar App** - Shows cert status (green = valid, red = expired)
- **CLI for servers/CI** - Headless certificate generation
- **Meme Landing Page** - LSP or Flash Slothmore greet you on login

## Quick Start

```bash
# Clone and build
git clone https://github.com/shawntz/cassh.git
cd cassh
make build

# Generate dev CA and run locally
make dev-ca
make dev-server
```

See the **[Full Documentation](https://shawnschwartz.com/cassh)** for complete setup, deployment, and configuration guides.

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](https://shawnschwartz.com/cassh/getting-started/) | Prerequisites and initial setup |
| [Server Setup](https://shawnschwartz.com/cassh/server-setup/) | CA keys, Entra app, configuration |
| [Deployment](https://shawnschwartz.com/cassh/deployment/) | Fly.io, Render, Railway, VPS |
| [Client Distribution](https://shawnschwartz.com/cassh/client/) | MDM deployment, PKG/DMG builds |
| [Configuration Reference](https://shawnschwartz.com/cassh/configuration/) | All config options explained |

## Roadmap

See the [full roadmap](https://shawnschwartz.com/cassh/roadmap/) for details.

| Status | Feature |
|--------|---------|
| :white_check_mark: | macOS menu bar app with certificate status |
| :white_check_mark: | Microsoft Entra ID (Azure AD) SSO |
| :white_check_mark: | 12-hour ephemeral SSH certificates |
| :white_check_mark: | Meme landing page (LSP & Sloth) |
| :construction: | VerifyPolicyIntegrity: Verify cryptographic signature |
| :memo: | Slack expiration notifications |
| :memo: | Hardware key support (YubiKey) |
| :memo: | Group-based access policies |

**Legend:** :white_check_mark: Complete | :construction: In Progress | :memo: Planned

## Security

> [!CAUTION]
> `cassh` is a privileged authentication system.
>
> - **Protect your CA private key** - it can sign certificates for anyone
> - **Use HTTPS** - OAuth tokens are transmitted
> - **Restrict Entra app** - limit which users can authenticate
> - Review access logs regularly

---

## Architecture

```mermaid
flowchart LR
    subgraph Client
        A[Menu Bar App]
    end

    subgraph Server
        B[cassh Server]
        C[Internal CA]
    end

    subgraph External
        D[Microsoft Entra ID]
        E[GitHub Enterprise]
    end

    A -->|1. Request cert| B
    B -->|2. OIDC auth| D
    D -->|3. Identity verified| B
    B -->|4. Sign pubkey| C
    C -->|5. SSH certificate| B
    B -->|6. Return cert| A
    A -->|7. SSH with cert| E
```

---

## Support the Project

`cassh` is built and maintained by [Shawn Schwartz](https://shawnschwartz.com), a PhD candidate in Psychology at Stanford where he studies attention and memory using neuroimaging — and teaches courses on research methods, data science, and computer science. By day, he builds software and hardware interfaces for cognitive neuroscience research. By night (and weekends), he tinkers with DevSecOps and builds tools like this one.

I started `cassh` because managing SSH keys across a team is a nightmare, and I kept thinking "there has to be a better way." Then I spent 18 weeks working [@slackhq](https://github.com/slackhq), and there I learned that ephemeral certificates are that better way.

If `cassh` saved you time or made your infrastructure more secure, consider supporting its development:

- [GitHub Sponsors](https://github.com/sponsors/shawntz)
- Star this repo and share it with others

Every bit of support helps me justify the time spent on this free and open-source side project instead of my dissertation.

---

## Why Open Source?

`cassh` is open source because security tooling should be auditable. You shouldn't have to trust a black box with your SSH authentication infrastructure.

I've learned so much from reading other people's code, and I hope `cassh` can be useful to others — whether you're learning `Go`, building your own auth systems, or just curious how SSH certificates work under the hood.

Contributions are welcome! Check out [CONTRIBUTING.md](CONTRIBUTING.md) if you'd like to help out.

---

## License

Apache 2.0 - See [LICENSE](LICENSE) for details. © Shawn Schwartz, 2025.
