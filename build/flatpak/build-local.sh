#!/bin/bash
# Build MailPong Flatpak locally (no Docker) from the Flathub manifest, which
# compiles the release pinned in build/flatpak/flathub/, not this checkout.
# Use build-flatpak.sh to package this checkout instead.

set -e

cd "$(dirname "$0")/../.."

echo "=== MailPong Flatpak Local Builder ==="
echo ""

# Check if flatpak-builder is installed
if ! command -v flatpak-builder &> /dev/null; then
    echo "❌ flatpak-builder is not installed"
    echo ""
    echo "Install it with:"
    echo "  Ubuntu/Debian: sudo apt install flatpak-builder"
    echo "  Fedora:        sudo dnf install flatpak-builder"
    echo "  Arch:          sudo pacman -S flatpak-builder"
    exit 1
fi

# Add flathub remote if not present
echo "Checking Flathub remote..."
if ! flatpak remote-list --user --columns=name | grep -qx "flathub"; then
    echo "⚠️  Flathub remote not found. Adding..."
    flatpak remote-add --if-not-exists --user flathub https://flathub.org/repo/flathub.flatpakrepo
fi

# Check if runtimes are installed
echo "Checking for required runtimes..."
if ! flatpak list --runtime | grep -q "org.gnome.Platform.*50"; then
    echo "⚠️  GNOME Platform 50 not found. Installing..."
    flatpak install -y --user flathub org.gnome.Platform//50 org.gnome.Sdk//50
fi

if ! flatpak list | grep -q "org.freedesktop.Sdk.Extension.golang"; then
    echo "⚠️  Go SDK extension not found. Installing..."
    flatpak install -y --user flathub org.freedesktop.Sdk.Extension.golang//25.08
fi

if ! flatpak list | grep -q "org.freedesktop.Sdk.Extension.node24"; then
    echo "⚠️  Node.js 24 SDK extension not found. Installing..."
    flatpak install -y --user flathub org.freedesktop.Sdk.Extension.node24//25.08
fi

echo "✅ All runtimes installed"

# Package into Flatpak
echo ""
echo "Packaging into Flatpak..."
echo "This will take a few minutes..."
echo ""

flatpak-builder --force-clean --user --install-deps-from=flathub \
    --repo=repo build-dir build/flatpak/flathub/io.github.caseylabs.MailPong.yml

# Create bundle for distribution
echo ""
echo "Creating .flatpak bundle..."
mkdir -p build/bin

# Get version from git tag, fallback to "dev" if no tag
VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "dev")
BUNDLE_NAME="MailPong-${VERSION}.flatpak"

flatpak build-bundle repo "build/bin/${BUNDLE_NAME}" io.github.caseylabs.MailPong

echo ""
echo "✅ Build complete!"
echo ""
echo "Flatpak bundle created: build/bin/${BUNDLE_NAME}"
echo ""
echo "To install locally:"
echo "  flatpak install --user build/bin/${BUNDLE_NAME}"
echo ""
echo "To run:"
echo "  flatpak run io.github.caseylabs.MailPong"
