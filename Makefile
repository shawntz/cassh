# cassh Makefile
# Builds both OSS template and enterprise locked distributions

BINARY_SERVER = cassh-server
BINARY_MENUBAR = cassh-menubar
BINARY_CLI = cassh

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

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
        app-bundle app-bundle-oss app-bundle-enterprise \
        dmg dmg-only pkg pkg-only \
        sign notarize \
        test lint

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

	# Copy icon if exists
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
test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

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
