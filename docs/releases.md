# Release Process

cassh uses automated tooling to generate releases with changelogs based on git commits.

## Overview

The release process consists of:

1. **Commit convention**: Using conventional commits for automatic changelog generation
2. **Automated changelog**: GitHub Actions keeps the `[Unreleased]` section up-to-date
3. **Release creation**: Script generates changelog and creates tagged release
4. **Automated builds**: GitHub Actions builds and publishes release artifacts

## Commit Message Convention

We use [Conventional Commits](https://www.conventionalcommits.org/) to automatically generate changelogs:

```
<type>: <description>

[optional body]

[optional footer]
```

### Types

| Type | Description | Changelog Section |
|------|-------------|-------------------|
| `feat:` | New feature | Added |
| `fix:` | Bug fix | Fixed |
| `perf:` | Performance improvement | Performance |
| `docs:` | Documentation changes | Documentation |
| `refactor:` | Code refactoring | Changed |
| `test:` | Test changes | Tests |
| `chore:` | Maintenance tasks | Maintenance |
| `ci:` | CI/CD changes | CI/CD |

### Examples

```bash
# New feature
git commit -m "feat: Add dark mode support to menu bar app"

# Bug fix
git commit -m "fix: Resolve certificate expiration notification timing"

# Breaking change
git commit -m "feat!: Change configuration file format

BREAKING CHANGE: Config format changed from JSON to TOML.
Migration guide: https://..."

# Multiple paragraphs
git commit -m "refactor: Simplify SSH key rotation logic

- Extract rotation policy into separate module
- Add unit tests for edge cases
- Update documentation with rotation flow diagram"
```

## Automated Changelog Updates

The `.github/workflows/changelog.yml` workflow automatically:

1. Triggers on every push to `main` (except docs/markdown changes)
2. Generates changelog from commits since last tag
3. Updates the `[Unreleased]` section in `CHANGELOG.md`
4. Commits the updated changelog

This keeps the changelog current without manual intervention.

## Creating a Release

### Prerequisites

- Clean working directory (no uncommitted changes)
- Push access to the repository
- Semantic versioning format: `MAJOR.MINOR.PATCH`

### Using the Release Script

```bash
# Create a new release
./scripts/create-release 1.2.0
```

The script will:

1. Validate version format
2. Generate changelog from commits since last tag
3. Show preview and ask for confirmation
4. Update `CHANGELOG.md`
5. Create and push git tag `v1.2.0`
6. Trigger GitHub Actions release workflow

### Manual Process

If you prefer to create releases manually:

```bash
# 1. Generate changelog preview
./scripts/generate-changelog v1.1.0 HEAD > /tmp/changelog.md

# 2. Edit CHANGELOG.md
vim CHANGELOG.md

# 3. Commit changelog
git add CHANGELOG.md
git commit -m "chore: Release v1.2.0"

# 4. Create annotated tag
git tag -a v1.2.0 -m "Release 1.2.0"

# 5. Push tag and commit
git push origin main
git push origin v1.2.0
```

## Release Workflow

When a tag matching `v*` is pushed, `.github/workflows/release.yml` automatically:

### macOS Build

1. Builds universal binary (Intel + Apple Silicon)
2. Creates app bundle with embedded policy
3. Signs with Developer ID Application certificate
4. Creates PKG installer
5. Signs PKG with Developer ID Installer certificate
6. Notarizes with Apple
7. Uploads artifact

### Linux Build

1. Builds server binary for Linux AMD64
2. Uploads artifact

### GitHub Release

1. Downloads all build artifacts
2. Extracts changelog for this version from `CHANGELOG.md`
3. Creates GitHub Release with:
   - Version tag (e.g., `v1.2.0`)
   - Changelog as release notes
   - Attached build artifacts:
     - `cassh-1.2.0.pkg` (macOS installer)
     - `cassh-server-linux-amd64` (Linux server)

### Homebrew Tap Update

1. Calculates SHA256 of macOS PKG
2. Updates `homebrew-cassh` tap repository
3. Users can install via `brew install --cask shawntz/cassh/cassh`

## Version Numbering

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes (e.g., config format change, API removal)
- **MINOR**: New features, backwards-compatible
- **PATCH**: Bug fixes, backwards-compatible

### Examples

| Version | Change Type | Example |
|---------|-------------|---------|
| `1.0.0` → `2.0.0` | Major | Changed policy config format |
| `1.2.0` → `1.3.0` | Minor | Added Windows support |
| `1.2.3` → `1.2.4` | Patch | Fixed notification bug |

## Release Checklist

Before creating a release:

- [ ] All tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation is up-to-date
- [ ] `CHANGELOG.md` `[Unreleased]` section is current
- [ ] Version number follows semantic versioning
- [ ] Breaking changes are clearly documented
- [ ] Migration guide provided for breaking changes

## Hotfix Releases

For urgent bug fixes:

```bash
# 1. Create hotfix branch from tag
git checkout -b hotfix/1.2.1 v1.2.0

# 2. Fix bug with conventional commit
git commit -m "fix: Resolve critical security issue in OIDC flow"

# 3. Create release from hotfix branch
./scripts/create-release 1.2.1

# 4. Merge back to main
git checkout main
git merge hotfix/1.2.1
git push origin main
```

## Troubleshooting

### Tag already exists

```bash
# Delete local tag
git tag -d v1.2.0

# Delete remote tag
git push origin :refs/tags/v1.2.0

# Recreate release
./scripts/create-release 1.2.0
```

### Release workflow failed

1. Check GitHub Actions logs
2. Fix issue (e.g., missing secrets)
3. Re-run failed jobs from Actions UI
4. Or delete tag and recreate:

```bash
git push origin :refs/tags/v1.2.0
./scripts/create-release 1.2.0
```

### Changelog generation issues

If commits aren't categorized correctly:

1. Check commit message format
2. Amend recent commits if needed:

```bash
git rebase -i HEAD~3
# Mark commits for 'reword'
# Fix commit messages to follow convention
```

## Scripts Reference

### `scripts/generate-changelog`

Generate changelog from commit range.

```bash
# Usage
./scripts/generate-changelog [FROM_REF] [TO_REF]

# Examples
./scripts/generate-changelog v1.0.0 v1.1.0
./scripts/generate-changelog v1.1.0 HEAD
./scripts/generate-changelog  # Uses last tag to HEAD
```

Output format:

```markdown
## [1.2.0] - 2025-12-19

### Added
- New feature description (abc1234)

### Fixed
- Bug fix description (def5678)

### Contributors
- John Doe
- Jane Smith

**Full Changelog**: https://github.com/user/repo/compare/v1.1.0...v1.2.0
```

### `scripts/create-release`

Create a release with automated changelog.

```bash
# Usage
./scripts/create-release VERSION

# Example
./scripts/create-release 1.2.0
```

Interactive prompts:

1. Shows changelog preview
2. Asks for confirmation
3. Shows `CHANGELOG.md` diff
4. Asks for final confirmation
5. Commits, tags, and pushes

## GitHub Actions Workflows

### `.github/workflows/changelog.yml`

Automatically updates `[Unreleased]` section on every push to main.

- **Trigger**: Push to `main` (excluding docs)
- **Action**: Generate and commit changelog updates
- **Skips**: If no changes since last tag

### `.github/workflows/release.yml`

Builds and publishes releases when version tags are pushed.

- **Trigger**: Push tags matching `v*`
- **Builds**:
  - macOS PKG (signed and notarized)
  - Linux server binary
- **Creates**: GitHub Release with changelog
- **Updates**: Homebrew tap

## Best Practices

1. **Write descriptive commits**: Explain *why*, not just *what*
2. **Use conventional format**: Enables automatic changelog generation
3. **Test before release**: Run full test suite
4. **Document breaking changes**: Include migration guide
5. **Review changelog**: Preview before confirming release
6. **Tag semantically**: Follow semantic versioning strictly

## Resources

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Semantic Versioning](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [GitHub Releases Guide](https://docs.github.com/en/repositories/releasing-projects-on-github)
