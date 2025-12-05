# 💰 `cassh` 💰

### Ephemeral SSH Certificates for GitHub Enterprise

`cassh` is an ephemeral SSH certificate system designed for GitHub Enterprise access.
Developed for [@ghostehq](https://github.com/ghostehq). Inspired by internal tooling [@slackhq](https://github.com/slackhq).

**How it works:**

1. Authenticate with Microsoft Entra (Azure AD)
2. Obtain a short-lived SSH certificate (12 hours)
3. Certificate auto-expires - no revocation needed

## Why cassh?

Permanent SSH keys are a liability. If a laptop is lost, stolen, or compromised:

- **With permanent keys:** Manual revocation required, often missed
- **With cassh:** Certificate expires automatically, zero action needed

## Features

- **Short-lived SSH certificates** - Signed by your internal CA, valid for 12 hours
- **Entra SSO Integration** - Sign in with your Microsoft identity
- **macOS Menu Bar App** - Shows cert status (green = valid, red = expired)
- **CLI for servers/CI** - Headless certificate generation
- **Meme Landing Page** - LSP or Flash Slothmore from the DMV to greet you on login


- **🟢 macOS Menu Bar Status Dot**
  - Green — your cert is valid, go forth and merge.
  - Red — regenerate before you `git push` sadness.

- **🎭 Meme Landing Page Rotation**
  On login, enjoy a random blessing from:
  - **Lumpy Space Princess from Adventure Time**
  - **DMV Sloth from Zootopia**  
  - (They cannot help you, but they will emotionally define you and hopefully leave you with a smile.)

- **💨 Zero Key Rot**
  Every 12 hours, certs evaporate into the void.

## Setup & Installation

### Login Experience (artistically cursed)

- You click "Generate cert"
- Browser opens to cassh portal
- LSP: *"Oh my GLOB just click the button"*
- OR Sloth: *"pl…ease… wai…t…"*

Then boom — new SSH cert, green dot, go commit code.

---


---

## Security

> [!CAUTION]
> `cassh` is a privileged authentication system.
>
> - **Protect your CA private key** - it can sign certificates for anyone
> - **Use HTTPS** - OAuth tokens are transmitted
> - **Restrict Entra app** - limit which users can authenticate
> - Review access logs regularly
