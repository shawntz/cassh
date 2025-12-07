# cassh Scripts

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
