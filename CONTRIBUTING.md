# Contributing to `cassh`

Thank you for your interest in contributing to `cassh`! This document provides guidelines and information for contributors.

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When creating a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Describe the behavior you observed and what you expected**
- **Include your environment details** (OS version, Go version, etc.)
- **Include relevant logs** (redact any sensitive information)

### Suggesting Features

Feature suggestions are welcome! Please:

- **Use a clear and descriptive title**
- **Provide a detailed description of the proposed feature**
- **Explain why this feature would be useful**
- **Consider how it fits with the project's security model**

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Follow the coding style** of the project
3. **Write meaningful commit messages**
4. **Include tests** for new functionality
5. **Update documentation** as needed
6. **Ensure all tests pass** before submitting

## Development Setup

### Prerequisites

- Go 1.22 or later
- macOS (for menu bar app development)
- `make` (build automation)

### Getting Started

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/cassh.git
cd cassh

# Install dependencies
make deps

# Generate a development CA key
make dev-ca

# Run the server in dev mode
make dev-server

# In another terminal, build and run the menu bar app
make menubar
./build/cassh-menubar
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint
```

### Project Structure

```
cmd/
  cassh-server/     # Web server (OIDC + cert signing)
  cassh-menubar/    # macOS menu bar app
  cassh-cli/        # Headless CLI

internal/
  ca/               # Certificate authority logic
  config/           # Configuration handling
  memes/            # Meme content for landing page
  oidc/             # Microsoft Entra ID integration
```

## Security

### Reporting Security Issues

**Do not report security vulnerabilities through public GitHub issues.**

Instead, please email **<SecOps@shawnschwartz.com>** with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

I will try my best to respond within 48-72 hours and work with you to understand and address the issue.

### Security Considerations for Contributors

When contributing, please consider:

- **Never commit secrets** (API keys, private keys, etc.)
- **Validate all user input** at system boundaries
- **Follow the principle of least privilege**
- **Be cautious with cryptographic code** - prefer well-tested libraries
- **Consider the split config model** - policy settings should not be user-overridable

## Style Guide

### Go Code

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and reasonably sized
- Handle errors explicitly

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) for automatic changelog generation:

```
<type>: <description>

[optional body]

[optional footer]
```

**Types:**

- `feat:` - New feature (appears in changelog under "Added")
- `fix:` - Bug fix (appears in changelog under "Fixed")
- `perf:` - Performance improvement (appears in changelog under "Performance")
- `docs:` - Documentation changes (appears in changelog under "Documentation")
- `refactor:` - Code refactoring (appears in changelog under "Changed")
- `test:` - Test changes (appears in changelog under "Tests")
- `chore:` - Maintenance tasks (appears in changelog under "Maintenance")
- `ci:` - CI/CD changes (appears in changelog under "CI/CD")

**Examples:**

```bash
# Feature
git commit -m "feat: Add dark mode support"

# Bug fix
git commit -m "fix: Resolve certificate expiration notification timing"

# Breaking change
git commit -m "feat!: Change config format to TOML

BREAKING CHANGE: Config format changed from JSON to TOML."

# With issue reference
git commit -m "fix: Handle SSH username extraction from clone URLs

Fixes #42"
```

**Additional guidelines:**

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters
- Reference issues and PRs in the body when relevant

See [docs/releases.md](docs/releases.md) for more on the release process.

### Documentation

- Update `README.md` for user-facing changes
- Update `CLAUDE.md` for architectural changes
- Keep code comments current with the code

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Questions?

Feel free to open an issue for any questions about contributing!
