# 💰 `cassh` 💰

### Ephemeral SSH Certificates, 12-Hour Access, and Unnecessary Cartoon-Level Drama.

`cassh` is an ephemeral SSH certificate system designed for GitHub Enterprise access.
Developed for [@ghostehq](https://github.com/ghostehq). Inspired by internal tooling [@slackhq](https://github.com/slackhq).

#### 1. Authenticate with Microsoft Entra.  
#### 2. Obtain a short-lived SSH cert.  
#### 3. Admire Lumpy Space Princess or the DMV Sloth from Zootopia.  
#### 4. Repeat every 12 hours because security and vibes.

## Why I spent time building `cassh`

Permanent SSH keys are a liability.

cassh issues **time-bound SSH certs** signed by an internal CA,
valid only for GitHub Enterprise access, and **only for 12 hours**.

Even if a laptop is lost, stolen, or emotionally compromised:
- cert expires
- access dies automatically
- zero revocation events

## Features

- **🔐 Short-lived SSH certificates**
  Signed by your internal CA and registered in GitHub Enterprise.

- **🪪 Entra SSO Integration**
  You sign in using your normal Microsoft identity provider.

- **🟢 macOS Menu Bar Status Dot**
  - Green — your cert is valid, go forth and merge.
  - Red — regenerate before you `git push` sadness.

- **🎭 Meme Landing Page Rotation**
  On login, enjoy a random blessing from:
  - **Lumpy Space Princess from Adventure Time**
  - **DMV Sloth from Zootopia**  
  (They cannot help you, but they will emotionally define you and hopefully leave you with a smile.)

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

> [!CAUTION]
> `cassh` is a privileged authentication system.
> If you fork it, operate it, or remix it, you are fully responsible for any 
> security outcomes, access grant failures, or certificate misuse.
