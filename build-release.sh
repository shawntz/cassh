#!/bin/bash
#
# build-release.sh - Build all cassh packages for distribution
#
# Usage: ./build-release.sh
#
# Prerequisites:
#   1. Configure cassh.policy.toml with your settings
#   2. Install create-dmg: brew install create-dmg (optional, for nicer DMG)
#
# This script will:
#   - Validate your policy config exists
#   - Build the menubar binary
#   - Create the app bundle with embedded policy
#   - Create DMG installer (requires sudo for disk mounting)
#   - Create PKG installer for MDM deployment
#
# Output files are placed in ./dist/

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║     cassh Release Build Script        ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════╝${NC}"
echo ""

# Check for policy file
POLICY_FILE="cassh.policy.toml"
if [ ! -f "$POLICY_FILE" ]; then
    echo -e "${RED}Error: $POLICY_FILE not found${NC}"
    echo ""
    echo "Please create your policy file first:"
    echo "  cp cassh.policy.example.toml cassh.policy.toml"
    echo "  # Edit cassh.policy.toml with your settings"
    exit 1
fi

echo -e "${GREEN}✓${NC} Found $POLICY_FILE"

# Validate required fields in policy
if ! grep -q "server_base_url" "$POLICY_FILE"; then
    echo -e "${RED}Error: server_base_url not set in $POLICY_FILE${NC}"
    exit 1
fi

if ! grep -q "github_enterprise_url" "$POLICY_FILE"; then
    echo -e "${YELLOW}Warning: github_enterprise_url not set - SSH config auto-setup will be disabled${NC}"
fi

echo -e "${GREEN}✓${NC} Policy config validated"
echo ""

# Check if running with sudo for DMG creation
if [ "$EUID" -ne 0 ]; then
    echo -e "${YELLOW}Note: DMG creation requires sudo. You may be prompted for your password.${NC}"
    echo ""
fi

# Step 1: Build menubar binary
echo -e "${GREEN}[1/4]${NC} Building menubar binary..."
make menubar
echo -e "${GREEN}✓${NC} Menubar binary built"
echo ""

# Step 2: Create app bundle
echo -e "${GREEN}[2/4]${NC} Creating app bundle..."
make app-bundle
echo -e "${GREEN}✓${NC} App bundle created"
echo ""

# Step 3: Create DMG
echo -e "${GREEN}[3/4]${NC} Creating DMG installer..."
if [ "$EUID" -ne 0 ]; then
    sudo make dmg
else
    make dmg
fi
echo -e "${GREEN}✓${NC} DMG created"
echo ""

# Step 4: Create PKG
echo -e "${GREEN}[4/4]${NC} Creating PKG installer..."
make pkg
echo -e "${GREEN}✓${NC} PKG created"
echo ""

# Summary
echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Build Complete!              ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════╝${NC}"
echo ""
echo "Output files:"
echo ""
ls -lh dist/*.dmg dist/*.pkg 2>/dev/null | awk '{print "  " $NF " (" $5 ")"}'
echo ""
echo "Distribution options:"
echo "  • DMG: Share directly with users for manual installation"
echo "  • PKG: Upload to MDM (Jamf, Kandji, Mosyle) for enterprise deployment"
echo ""
echo -e "${YELLOW}Optional: Sign and notarize for Gatekeeper${NC}"
echo "  make sign      # Requires APPLE_DEVELOPER_ID"
echo "  make notarize  # Requires Apple ID credentials"
