#!/bin/bash
# Build MailPong Flatpak using Docker (hybrid approach: build binary on host in container, then package)

set -e

cd "$(dirname "$0")/../.."

echo "=== MailPong Flatpak Docker Builder ==="
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed"
    echo ""
    echo "Install it with:"
    echo "  Ubuntu/Debian: sudo apt install docker.io"
    echo "  Fedora:        sudo dnf install docker"
    echo "  Arch:          sudo pacman -S docker"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    echo "❌ Docker daemon is not running"
    echo ""
    echo "Start it with:"
    echo "  sudo systemctl start docker"
    exit 1
fi

# Check for OAuth credentials
if [ -z "$GOOGLE_CLIENT_ID" ] && [ -z "$MICROSOFT_CLIENT_ID" ]; then
    echo "⚠️  Warning: No OAuth credentials found"
    echo "Gmail and Outlook OAuth will not work in the built app"
    echo ""
fi

echo "Building Docker image (this may take a few minutes on first run)..."
docker build -t mailpong-flatpak-builder -f build/flatpak/Dockerfile build/flatpak

echo ""
echo "Building MailPong in Docker container..."
echo ""

# Get version from git tag
VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "dev")

# Run the build in Docker as the host user so files written to the bind
# mount stay owned by that user. HOME points at a writable path for the Go
# and npm caches.
docker run --rm --privileged \
    --user "$(id -u):$(id -g)" \
    -e HOME=/tmp/home \
    -v "$(pwd):/workspace" \
    -w /workspace \
    -e GOOGLE_CLIENT_ID="${GOOGLE_CLIENT_ID}" \
    -e GOOGLE_CLIENT_SECRET="${GOOGLE_CLIENT_SECRET}" \
    -e MICROSOFT_CLIENT_ID="${MICROSOFT_CLIENT_ID}" \
    -e GOOGLE_TESTING_CLIENT_ID="${GOOGLE_TESTING_CLIENT_ID}" \
    -e GOOGLE_TESTING_CLIENT_SECRET="${GOOGLE_TESTING_CLIENT_SECRET}" \
    mailpong-flatpak-builder \
    bash -c "
        set -e
        # Do not leave the generated credentials file in the checkout.
        trap 'rm -f internal/oauth2/credentials_gen.go' EXIT

        echo 'Installing frontend dependencies...'
        (cd frontend && npm ci)

        echo ''
        echo 'Building MailPong binary...'
        make build-linux

        echo ''
        echo 'Packaging into Flatpak...'
        # The dev manifest packages the binary built above; the flathub
        # manifest would rebuild a tagged upstream release from git instead.
        flatpak-builder --force-clean --disable-rofiles-fuse --repo=repo build-dir build/flatpak/io.github.caseylabs.MailPong-dev.yml

        echo ''
        echo 'Creating .flatpak bundle...'
        mkdir -p build/bin
        flatpak build-bundle repo build/bin/MailPong-${VERSION}.flatpak io.github.caseylabs.MailPong
    "

echo ""
echo "✅ Build complete!"
echo ""
echo "Flatpak bundle created: build/bin/MailPong-${VERSION}.flatpak"
echo ""
echo "To install locally:"
echo "  flatpak install --user build/bin/MailPong-${VERSION}.flatpak"
echo ""
echo "To run:"
echo "  flatpak run io.github.caseylabs.MailPong"
