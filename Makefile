# cassh Makefile
# Builds both OSS template and enterprise locked distributions

BINARY_SERVER = cassh-server
BINARY_MENUBAR = cassh-menubar
BINARY_CLI = cassh

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.buildCommit=$(BUILD_COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Directories
BUILD_DIR = build
DIST_DIR = dist
APP_BUNDLE = $(BUILD_DIR)/cassh.app

# macOS code signing (set these in environment or CI)
APPLE_DEVELOPER_ID ?=
APPLE_TEAM_ID ?=
APPLE_KEYCHAIN_PROFILE ?= cassh-notarize

# Policy file to bundle (OSS uses example, enterprise uses real config)
# Override with: make app-bundle POLICY_FILE=cassh.policy.toml
POLICY_FILE ?= cassh.policy.example.toml

.PHONY: all clean deps build build-oss build-enterprise \
        server menubar cli \
        icon app-bundle app-bundle-oss app-bundle-enterprise \
        dmg dmg-only pkg pkg-only \
        sign notarize \
        test test-race test-coverage test-ci test-list \
        test-ca test-config test-memes test-menubar \
        lint

# Default: build all binaries
all: deps build

# Install dependencies
deps:
	go mod download
	go mod tidy

# Build all binaries for current platform
build: server menubar cli

server:
	@echo "Building cassh-server..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_SERVER) ./cmd/cassh-server

menubar:
	@echo "Building cassh-menubar..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_MENUBAR) ./cmd/cassh-menubar

cli:
	@echo "Building cassh CLI..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLI) ./cmd/cassh-cli

# =============================================================================
# OSS Template Build (includes setup wizard, no locked policy)
# =============================================================================
build-oss: build
	@echo "Building OSS template distribution..."
	@mkdir -p $(DIST_DIR)/oss
	@cp $(BUILD_DIR)/$(BINARY_SERVER) $(DIST_DIR)/oss/
	@cp $(BUILD_DIR)/$(BINARY_CLI) $(DIST_DIR)/oss/
	@cp cassh.policy.toml $(DIST_DIR)/oss/cassh.policy.toml.example
	@echo "OSS build complete: $(DIST_DIR)/oss/"

# =============================================================================
# Enterprise Build (locked policy, no setup wizard)
# =============================================================================
build-enterprise: build
	@echo "Building enterprise distribution..."
	@mkdir -p $(DIST_DIR)/enterprise
	$(MAKE) app-bundle-enterprise
	@echo "Enterprise build complete: $(DIST_DIR)/enterprise/"

# =============================================================================
# macOS Icon Compilation (Liquid Glass support)
# =============================================================================
# Compiles .icon bundle (from Icon Composer) into Assets.car and fallback .icns
# Requires Xcode command line tools (actool)
icon:
	@echo "Compiling icon assets..."
	@if [ ! -d packaging/macos/icon/cassh.icon ]; then \
		echo "Error: packaging/macos/icon/cassh.icon not found"; \
		echo "Create it using Icon Composer (Xcode 16+)"; \
		exit 1; \
	fi
	@actool packaging/macos/icon/cassh.icon \
		--compile packaging/macos \
		--output-format human-readable-text \
		--notices --warnings --errors \
		--output-partial-info-plist packaging/macos/assetcatalog_generated_info.plist \
		--app-icon cassh \
		--include-all-app-icons \
		--enable-on-demand-resources NO \
		--development-region en \
		--target-device mac \
		--minimum-deployment-target 15.0 \
		--platform macosx
	@rm -f packaging/macos/assetcatalog_generated_info.plist
	@# Copy macOS Default PNG to docs assets for README and docs site
	@if [ -f packaging/macos/icon/icon_exports/cassh-macOS-Default-1024x1024@1x.png ]; then \
		cp packaging/macos/icon/icon_exports/cassh-macOS-Default-1024x1024@1x.png docs/assets/logo.png; \
	fi
	@echo "Icon assets compiled:"
	@echo "  - packaging/macos/Assets.car (liquid glass)"
	@echo "  - packaging/macos/cassh.icns (fallback)"
	@echo "  - docs/assets/logo.png (for README/docs)"

# =============================================================================
# macOS App Bundle
# =============================================================================
# Default app-bundle uses POLICY_FILE variable (defaults to example for OSS)
app-bundle: menubar
	@echo "Creating macOS app bundle (policy: $(POLICY_FILE))..."
	@rm -rf $(APP_BUNDLE)
	@mkdir -p $(APP_BUNDLE)/Contents/MacOS
	@mkdir -p $(APP_BUNDLE)/Contents/Resources

	# Copy binary
	@cp $(BUILD_DIR)/$(BINARY_MENUBAR) $(APP_BUNDLE)/Contents/MacOS/cassh

	# Copy policy file
	@cp $(POLICY_FILE) $(APP_BUNDLE)/Contents/Resources/cassh.policy.toml

	# Create Info.plist
	@cat packaging/macos/Info.plist.template | \
		sed 's/{{VERSION}}/$(VERSION)/g' | \
		sed 's/{{BUILD_TIME}}/$(BUILD_TIME)/g' > $(APP_BUNDLE)/Contents/Info.plist

	# Copy Assets.car (compiled icon with liquid glass support) and fallback icns
	@if [ -f packaging/macos/Assets.car ]; then \
		cp packaging/macos/Assets.car $(APP_BUNDLE)/Contents/Resources/; \
	fi
	@if [ -f packaging/macos/cassh.icns ]; then \
		cp packaging/macos/cassh.icns $(APP_BUNDLE)/Contents/Resources/; \
	fi

	# Copy LaunchAgent plist for PKG postinstall
	@cp packaging/macos/com.shawnschwartz.cassh.plist $(APP_BUNDLE)/Contents/Resources/

	# Make policy read-only
	@chmod 444 $(APP_BUNDLE)/Contents/Resources/cassh.policy.toml

	@echo "App bundle created: $(APP_BUNDLE)"

# OSS app bundle (blank policy, shows setup wizard on first run)
app-bundle-oss: POLICY_FILE = cassh.policy.example.toml
app-bundle-oss: app-bundle
	@echo "OSS app bundle created (will show setup wizard)"

# Enterprise app bundle (requires cassh.policy.toml with real config)
app-bundle-enterprise: POLICY_FILE = cassh.policy.toml
app-bundle-enterprise: app-bundle
	@echo "Enterprise app bundle created (locked policy)"

# =============================================================================
# DMG Creation (requires sudo for disk image mounting)
# Usage: sudo make dmg
# =============================================================================
dmg: app-bundle dmg-only

dmg-only:
	@echo "Creating DMG installer..."
	@mkdir -p $(DIST_DIR)
	@rm -f $(DIST_DIR)/cassh-$(VERSION).dmg
	# Ensure any previously mounted volume with same name is detached
	@hdiutil detach "/Volumes/cassh Installer" >/dev/null 2>&1 || true

	# Create DMG (background disabled due to macOS Tahoe compatibility issues)
	@if command -v create-dmg &> /dev/null; then \
		create-dmg \
			--volname "cassh Installer" \
			--volicon "packaging/macos/cassh.icns" \
			--window-pos 400 200 \
			--window-size 600 400 \
			--icon-size 128 \
			--icon "cassh.app" 150 200 \
			--hide-extension "cassh.app" \
			--app-drop-link 450 200 \
			--no-internet-enable \
			"$(DIST_DIR)/cassh-$(VERSION).dmg" \
			"$(APP_BUNDLE)"; \
	else \
		hdiutil create -volname "cassh Installer" -srcfolder $(APP_BUNDLE) \
			-ov -format UDZO "$(DIST_DIR)/cassh-$(VERSION).dmg"; \
	fi

	@echo "DMG created: $(DIST_DIR)/cassh-$(VERSION).dmg"

# =============================================================================
# PKG Creation (for MDM deployment)
# =============================================================================
pkg: app-bundle pkg-only

pkg-only:
	@echo "Creating PKG installer for MDM..."
	@mkdir -p $(DIST_DIR)
	@rm -f $(DIST_DIR)/cassh-$(VERSION).pkg

	# Build component package
	pkgbuild \
		--root $(APP_BUNDLE) \
		--identifier com.shawnschwartz.cassh \
		--version $(VERSION) \
		--install-location /Applications/cassh.app \
		--scripts packaging/macos/scripts \
		$(BUILD_DIR)/cassh-component.pkg

	# Build product archive
	productbuild \
		--distribution packaging/macos/distribution.xml \
		--package-path $(BUILD_DIR) \
		--resources packaging/macos/resources \
		$(DIST_DIR)/cassh-$(VERSION).pkg

	@echo "PKG created: $(DIST_DIR)/cassh-$(VERSION).pkg"

# =============================================================================
# Code Signing (macOS)
# =============================================================================
sign: app-bundle
ifdef APPLE_DEVELOPER_ID
	@echo "Signing app bundle..."
	codesign --force --deep --sign "Developer ID Application: $(APPLE_DEVELOPER_ID)" \
		--options runtime \
		--entitlements packaging/macos/entitlements.plist \
		$(APP_BUNDLE)
	@echo "Verifying signature..."
	codesign --verify --verbose $(APP_BUNDLE)
else
	@echo "Warning: APPLE_DEVELOPER_ID not set, skipping signing"
endif

# =============================================================================
# Notarization (macOS)
# =============================================================================
notarize: dmg
ifdef APPLE_KEYCHAIN_PROFILE
	@echo "Submitting for notarization..."
	xcrun notarytool submit $(DIST_DIR)/cassh-$(VERSION).dmg \
		--keychain-profile "$(APPLE_KEYCHAIN_PROFILE)" \
		--wait
	@echo "Stapling ticket..."
	xcrun stapler staple $(DIST_DIR)/cassh-$(VERSION).dmg
else
	@echo "Warning: APPLE_KEYCHAIN_PROFILE not set, skipping notarization"
endif

# =============================================================================
# LaunchAgent for auto-start
# =============================================================================
install-launchagent:
	@echo "Installing LaunchAgent..."
	@mkdir -p ~/Library/LaunchAgents
	@cp packaging/macos/com.shawnschwartz.cassh.plist ~/Library/LaunchAgents/
	@launchctl load ~/Library/LaunchAgents/com.shawnschwartz.cassh.plist
	@echo "cassh will now start automatically on login"

uninstall-launchagent:
	@echo "Removing LaunchAgent..."
	@launchctl unload ~/Library/LaunchAgents/com.shawnschwartz.cassh.plist 2>/dev/null || true
	@rm -f ~/Library/LaunchAgents/com.shawnschwartz.cassh.plist
	@echo "LaunchAgent removed"

# =============================================================================
# Testing & Linting
# =============================================================================

# Run all tests (cross-platform packages only on Linux, all on macOS)
test:
ifeq ($(shell uname),Darwin)
	@echo "Running all tests (macOS)..."
	CGO_ENABLED=1 go test -v ./...
else
	@echo "Running cross-platform tests (Linux)..."
	go test -v $$(go list ./... | grep -v /cmd/cassh-menubar)
endif

# Run tests with race detection (recommended for CI)
test-race:
ifeq ($(shell uname),Darwin)
	@echo "Running all tests with race detection (macOS)..."
	CGO_ENABLED=1 go test -v -race ./...
else
	@echo "Running cross-platform tests with race detection (Linux)..."
	go test -v -race $$(go list ./... | grep -v /cmd/cassh-menubar)
endif

# Run tests with coverage report
test-coverage:
ifeq ($(shell uname),Darwin)
	@echo "Running all tests with coverage (macOS)..."
	CGO_ENABLED=1 go test -coverprofile=coverage.out ./...
else
	@echo "Running cross-platform tests with coverage (Linux)..."
	go test -coverprofile=coverage.out $$(go list ./... | grep -v /cmd/cassh-menubar)
endif
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run specific package tests
test-ca:
	@echo "Running CA tests..."
	go test -v ./internal/ca/...

test-config:
	@echo "Running config tests..."
	go test -v ./internal/config/...

test-memes:
	@echo "Running memes tests..."
	go test -v ./internal/memes/...

test-menubar:
	@echo "Running menubar tests (macOS only)..."
ifeq ($(shell uname),Darwin)
	CGO_ENABLED=1 go test -v ./cmd/cassh-menubar/...
else
	@echo "Skipping: menubar tests require macOS"
endif

# Full CI-equivalent test suite with race detection and coverage
test-ci: lint test-race
	@echo "✅ All CI checks passed"

# List all test files
test-list:
	@echo "Test files in codebase:"
	@find . -name '*_test.go' -type f | sort
	@echo ""
	@echo "Approximate total test functions (from source):"
	@grep -R --include='*_test.go' -E '^[[:space:]]*func[[:space:]]+Test[[:upper:]][[:alnum:]_]*' . 2>/dev/null | wc -l

lint:
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, running go vet..."; \
		go vet ./...; \
	fi

# =============================================================================
# Development helpers
# =============================================================================
dev-server: server
	CASSH_POLICY_PATH=cassh.policy.dev.toml \
	CASSH_CA_KEY_PATH=dev/ca_key \
	CASSH_LISTEN_ADDR=:8080 \
	$(BUILD_DIR)/$(BINARY_SERVER)

dev-ca:
	@echo "Generating development CA key..."
	@mkdir -p dev
	ssh-keygen -t ed25519 -f dev/ca_key -N "" -C "cassh-dev-ca"
	@echo "CA key generated: dev/ca_key"
	@echo "CA public key: dev/ca_key.pub"

# =============================================================================
# Clean
# =============================================================================
clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	rm -f coverage.out coverage.html

# =============================================================================
# Release (builds everything)
# =============================================================================
release: clean deps lint test build-oss build-enterprise sign dmg pkg
	@echo "Release build complete!"
	@echo "Artifacts:"
	@ls -la $(DIST_DIR)/
