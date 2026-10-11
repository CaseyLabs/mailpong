#!/bin/bash
# Build MailPong Flatpak for local development/testing.
# Builds the binary on the host, then packages it into a Flatpak.
# Usage: build-flatpak.sh [--install]
#   --install  also install the result into the user Flatpak scope

set -e

INSTALL_FLAG=
case "${1:-}" in
    "") ;;
    --install) INSTALL_FLAG=--install ;;
    *) echo "Usage: $0 [--install]" >&2; exit 1 ;;
esac

# Change to project root
cd "$(dirname "$0")/../.."

echo "=== MailPong Flatpak Dev Builder ==="
echo ""

# Check if flatpak-builder is installed
if ! command -v flatpak-builder &> /dev/null; then
    echo "flatpak-builder is not installed"
    echo ""
    echo "Install it with:"
    echo "  Fedora:        sudo dnf install flatpak-builder"
    echo "  Ubuntu/Debian: sudo apt install flatpak-builder"
    echo "  Arch:          sudo pacman -S flatpak-builder"
    exit 1
fi

# Add flathub remote if not present
echo "Checking Flathub remote..."
if ! flatpak remote-list --user --columns=name | grep -qx "flathub"; then
    echo "Flathub remote not found. Adding..."
    flatpak remote-add --if-not-exists --user flathub https://flathub.org/repo/flathub.flatpakrepo
fi

# Check if runtimes are installed
echo "Checking for required runtimes..."
if ! flatpak list --runtime | grep -q "org.gnome.Platform.*50"; then
    echo "GNOME Platform 50 not found. Installing..."
    flatpak install -y --user flathub org.gnome.Platform//50 org.gnome.Sdk//50
fi

echo "All runtimes installed"
echo ""

# Check for OAuth credentials
if [ -z "$GOOGLE_CLIENT_ID" ] && [ -z "$MICROSOFT_CLIENT_ID" ]; then
    echo "Warning: No OAuth credentials found"
    echo "Gmail and Outlook OAuth will not work in the built app"
    echo ""
fi

# Build the binary on the host
echo "Building MailPong binary on host..."
make build-linux

# Package into Flatpak using the dev manifest (packaging only, no compilation)
echo ""
echo "Packaging into Flatpak..."
echo ""

flatpak-builder --force-clean --user $INSTALL_FLAG --install-deps-from=flathub \
    --repo=repo build-dir build/flatpak/io.github.caseylabs.MailPong-dev.yml

# Create bundle for distribution/testing on other machines
echo ""
echo "Creating .flatpak bundle..."
mkdir -p build/bin

flatpak build-bundle repo build/bin/MailPong-dev.flatpak io.github.caseylabs.MailPong

echo ""
echo "Build complete!"
echo ""
echo "Flatpak bundle: build/bin/MailPong-dev.flatpak"
echo ""
echo "To install:"
echo "  flatpak install --user build/bin/MailPong-dev.flatpak"
echo ""
echo "To run:"
echo "  flatpak run io.github.caseylabs.MailPong"
