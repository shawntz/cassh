# cassh Test Documentation

This document provides an index of all tests in the cassh codebase, organized by package.

## Quick Reference

| Command | Description |
|---------|-------------|
| `make test` | Run all tests (platform-aware) |
| `make test-race` | Run tests with race detection |
| `make test-coverage` | Generate coverage report |
| `make test-ci` | Run full CI-equivalent suite |
| `make test-list` | List all test files |

### Package-Specific Tests

| Command | Package |
|---------|---------|
| `make test-ca` | Certificate Authority |
| `make test-config` | Configuration |
| `make test-memes` | Memes/Quotes |
| `make test-menubar` | macOS Menubar (macOS only) |

## Test Index

### internal/ca/ca_test.go

**SSH Certificate Authority Tests**

| Test Function | Description |
|--------------|-------------|
| `TestNewCA` | Validates CA creation with valid/invalid keys and principals |
| `TestSignPublicKey` | Tests certificate signing with correct properties, validity period, and extensions |
| `TestSignPublicKeyWithPrincipals` | Tests signing with custom principals configuration |
| `TestGenerateKeyPair` | Validates Ed25519 key pair generation |
| `TestMarshalCertificate` | Tests certificate serialization and deserialization |
| `TestParsePublicKey` | Tests SSH public key parsing |
| `TestParsePublicKeyInvalid` | Tests error handling for invalid public keys |
| `TestParseCertificate` | Tests certificate parsing from marshaled bytes |
| `TestParseCertificateInvalid` | Tests error handling for non-certificate keys |
| `TestGetCertInfo` | Tests certificate info extraction (KeyID, principals, expiry) |
| `TestGetCertInfoExpired` | Tests expired certificate detection |

**Helper Functions:**
- `generateTestCAKey` - Creates test CA key pair
- `generateTestUserKey` - Creates test user key pair

---

### internal/config/config_test.go

**Configuration Management Tests**

| Test Function | Description |
|--------------|-------------|
| `TestDefaultUserConfig` | Validates default user configuration values |
| `TestServerConfigIsDevMode` | Tests dev mode detection for server config |
| `TestPolicyConfigIsDevMode` | Tests dev mode detection for policy config |
| `TestServerConfigValidate` | Validates server config validation logic (dev/prod modes) |
| `TestLoadServerConfigFromEnv` | Tests loading config from environment variables |
| `TestLoadServerConfigDefaults` | Tests default values when env vars not set |
| `TestLoadServerConfigFromFile` | Tests loading config from TOML files |
| `TestMergeConfigs` | Tests merging policy and user configs |
| `TestVerifyPolicyIntegrity` | Tests policy signature verification |
| `TestUserConfigPath` | Tests user config path resolution |
| `TestPolicyPath` | Tests policy config path resolution |
| `TestLoadUserConfigNonExistent` | Tests graceful handling of missing config |

**Helper Functions:**
- `setEnv` - Sets environment variable with cleanup
- `unsetEnv` - Unsets environment variable with cleanup

---

### internal/memes/memes_test.go

**Meme Character and Quote Tests**

| Test Function | Description |
|--------------|-------------|
| `TestGetRandomCharacter` | Tests random character selection (LSP or Sloth) |
| `TestGetCharacterByName` | Tests character lookup by name |
| `TestGetRandomQuote` | Tests random quote selection from character |
| `TestGetMemeData` | Tests complete meme data generation |
| `TestCharacterData` | Validates predefined character data integrity |
| `TestCharactersSlice` | Validates Characters slice contains expected entries |

---

### cmd/cassh-menubar/keytitle_test.go

**macOS Menubar Key Management Tests** (requires macOS, CGO)

| Test Function | Description |
|--------------|-------------|
| `TestGetKeyTitle` | Tests SSH key title generation with hostname |
| `TestGetLegacyKeyTitle` | Tests legacy key title format (without hostname) |
| `TestGetKeyTitleHostnameTruncation` | Tests hostname truncation to 30 chars |
| `TestNeedsKeyRotation` | Tests key rotation detection logic |
| `TestKeyTitleConsistency` | Tests that key title generation is deterministic |
| `TestLegacyAndNewTitlesDifferent` | Tests that new format differs from legacy |

**Test Cases for `TestNeedsKeyRotation`:**
- Enterprise connection (never rotates)
- Personal with no rotation configured
- Personal with no creation time recorded
- Personal with fresh key (within rotation window)
- Personal with expired key (past rotation window)
- Personal at exact rotation boundary
- Various rotation hour configurations (4h, 24h)

---

## Platform Requirements

| Package | Linux | macOS | Windows | Notes |
|---------|-------|-------|---------|-------|
| `internal/ca` | Yes | Yes | Yes* | Pure Go |
| `internal/config` | Yes | Yes | Yes* | Pure Go |
| `internal/memes` | Yes | Yes | Yes* | Pure Go |
| `cmd/cassh-menubar` | No | Yes | No | Requires CGO, macOS frameworks |

* **Windows Note**: While cross-platform packages can technically be tested on Windows using `go test` directly, the Makefile does not support Windows (it uses `uname` for platform detection). Windows support is planned for the future (see [roadmap](docs/roadmap.md)). For now, Windows users should:
- Use Go commands directly: `go test ./internal/... ./cmd/cassh-server/... ./cmd/cassh-cli/...`
- Or build and test via GitHub Actions
- The menubar app is macOS-only and will not be ported to Windows (a separate Windows system tray app is planned)

## CI/CD Integration

The GitHub Actions workflow (`.github/workflows/build.yml`) runs:

1. **test-linux**: All tests except `cmd/cassh-menubar/*` on Ubuntu
2. **test-macos**: All tests including menubar on macOS with `CGO_ENABLED=1`
3. **lint**: golangci-lint checks
4. **coverage**: Combined coverage report uploaded to Codecov
5. **ci-success**: Final gate that blocks merge if any job fails

### Branch Protection

The `ci-success` job must pass before PRs can be merged to `main`.

## Running Tests Locally

### macOS and Linux

```bash
# Full test suite (recommended before committing)
make test-ci

# Quick test run
make test

# With coverage report
make test-coverage
open coverage.html

# Specific package
make test-ca
make test-config
make test-memes
make test-menubar  # macOS only

# List all test files
make test-list
```

### Windows

The Makefile is not supported on Windows. Use Go commands directly:

```powershell
# Run cross-platform tests (excludes menubar)
go test -v ./internal/... ./cmd/cassh-server/... ./cmd/cassh-cli/...

# With race detection
go test -v -race ./internal/... ./cmd/cassh-server/... ./cmd/cassh-cli/...

# With coverage
go test -coverprofile=coverage.out ./internal/... ./cmd/cassh-server/... ./cmd/cassh-cli/...
go tool cover -html=coverage.out -o coverage.html
```

**Note**: The menubar app (`cmd/cassh-menubar`) requires macOS and cannot be built or tested on Windows.

## Adding New Tests

When adding new tests:

1. Follow Go convention: place `*_test.go` files alongside source files
2. Use table-driven tests where appropriate
3. Add build tags if platform-specific: `//go:build darwin`
4. Update this document with new test functions
5. Ensure CI runs the new tests (menubar tests need macOS runner)

### Test Naming Convention

- Test functions: `TestFunctionName` or `TestTypeName_MethodName`
- Helper functions: `helperFunctionName` (unexported) or as test helpers with `t.Helper()`
- Table-driven test cases: Use descriptive names in the test struct

---

*Last updated: 2024-12-10*
*Test count: 35 tests across 4 packages*
