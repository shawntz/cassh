# cassh Scripts

## Release Management

- **generate-changelog** - Generate changelog from git commits
  ```bash
  ./scripts/generate-changelog                    # From last tag to HEAD
  ./scripts/generate-changelog v1.0.0 v1.1.0     # Between specific refs
  ```

  Uses [conventional commits](https://www.conventionalcommits.org/) to categorize changes:
  - `feat:` → Added
  - `fix:` → Fixed
  - `docs:` → Documentation
  - `chore:`, `test:`, `ci:`, etc.

- **create-release** - Create a new release with automated changelog
  ```bash
  ./scripts/create-release 1.2.0
  ```

  Interactive script that:
  1. Generates changelog from commits
  2. Updates `CHANGELOG.md`
  3. Creates git tag `v1.2.0`
  4. Pushes tag to trigger release workflow

  See [docs/releases.md](../docs/releases.md) for full documentation.

## Build & Test

- **test-release** - Test release builds locally before publishing
  ```bash
  ./scripts/test-release              # Build unsigned
  ./scripts/test-release --sign       # Sign with Developer ID
  ./scripts/test-release --fresh      # Clear config for fresh install
  ./scripts/test-release --pkg        # Open PKG installer
  ```

## Cloud Deployment

One-command deployment scripts for major cloud providers. See the [Deployment Guide](https://shawnschwartz.com/cassh/deployment/) for full documentation.

**Prerequisites:** Set environment variables before running any script:
```bash
export CASSH_OIDC_CLIENT_ID='your-client-id'
export CASSH_OIDC_CLIENT_SECRET='your-client-secret'
export CASSH_OIDC_TENANT='your-tenant-id'
export CASSH_CA_PRIVATE_KEY="$(cat ca_key)"
```

**Available Scripts:**

- **[Google Cloud Run](https://shawnschwartz.com/cassh/deployment/#google-cloud-run)** - `./scripts/deploy-gcp.sh`
- **[AWS App Runner](https://shawnschwartz.com/cassh/deployment/#aws-app-runner)** - `./scripts/deploy-aws.sh`
- **[Azure Container Apps](https://shawnschwartz.com/cassh/deployment/#azure-container-apps)** - `./scripts/deploy-azure.sh`

Other deployment options (Render, Fly.io, Railway, Self-Hosted) are documented in the [Deployment Guide](https://shawnschwartz.com/cassh/deployment/).
