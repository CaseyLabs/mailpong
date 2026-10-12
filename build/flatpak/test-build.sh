#!/bin/bash
# Test Flatpak from-source build in a container matching GitHub Actions environment
# Usage: ./test-build.sh v0.1.18
#
# This simulates the GA workflow: runs calculate-hashes.sh to resolve the git tag/commit,
# then builds the Flatpak from source with vendored dependencies.

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <version-tag>"
    echo "Example: $0 v0.1.18"
    exit 1
fi

VERSION="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=========================================="
echo "Flatpak From-Source Build Test"
echo "=========================================="
echo "Version: $VERSION"
echo "Repo: $REPO_DIR"
echo ""

# Create a test script to run inside the container
cat > /tmp/flatpak-build-test-inner.sh <<'INNERSCRIPT'
#!/bin/bash
set -e

VERSION="$1"

# Install packages as root, then re-run this script as a non-root host user so
# files written to the bind mount stay owned by that user. A root host user
# keeps running as root.
if [ "$(id -u)" = 0 ]; then
echo "Installing dependencies..."
apt-get update
# Normal install first; fall back to exact frozen noble release-pocket
# versions when -updates/-security carry an unsatisfiable dependency (e.g.
# the 2026-09-17 bubblewrap pull) — same guard as the release workflow.
# Versions are noble-specific; refresh alongside the workflow if the base
# image changes.
apt-get install -y -o APT::Get::Always-Include-Phased-Updates=true flatpak flatpak-builder wget git \
  || apt-get install -y flatpak=1.14.6-1 libflatpak0=1.14.6-1 bubblewrap=0.9.0-1build1 flatpak-builder=1.4.2-1build2 wget git
fi

if [ "$(id -u)" = 0 ] && [ -n "$HOST_UID" ] && [ "$HOST_UID" != 0 ]; then
mkdir -p /tmp/home
chown "$HOST_UID:$HOST_GID" /tmp/home
exec env HOME=/tmp/home setpriv --reuid="$HOST_UID" --regid="$HOST_GID" --clear-groups "$0" "$@"
fi

echo ""
echo "Adding Flathub repository..."
flatpak remote-add --if-not-exists --user flathub https://flathub.org/repo/flathub.flatpakrepo

echo ""
echo "Installing Flatpak runtimes and SDK extensions..."
flatpak install -y --user flathub \
  org.gnome.Platform//50 \
  org.gnome.Sdk//50 \
  org.freedesktop.Sdk.Extension.golang//25.08 \
  org.freedesktop.Sdk.Extension.node24//25.08

echo ""
echo "=========================================="
echo "Environment ready. Starting build process..."
echo "=========================================="
echo ""

cd /workspace

echo "Cleaning flatpak-builder cache..."
rm -rf .flatpak-builder

echo "Checking that tag ${VERSION} exists..."
if ! git ls-remote --exit-code --tags https://github.com/CaseyLabs/mailven.git "refs/tags/${VERSION}" >/dev/null; then
    echo ""
    echo "ERROR: Tag ${VERSION} not found at https://github.com/CaseyLabs/mailven"
    echo ""
    exit 1
fi

echo ""
echo "Calculating hashes and updating manifest..."
cd build/flatpak/flathub
chmod +x calculate-hashes.sh
./calculate-hashes.sh "$VERSION"

echo ""
echo "=========================================="
echo "Building Flatpak from source..."
echo "=========================================="
cd /workspace

flatpak-builder --user --force-clean --repo=repo \
  --install-deps-from=flathub \
  build-dir build/flatpak/flathub/io.github.caseylabs.Mailven.yml

echo ""
echo "Creating bundle..."
mkdir -p build/bin
flatpak build-bundle repo build/bin/Mailven-${VERSION}.flatpak io.github.caseylabs.Mailven

echo ""
echo "=========================================="
echo "Build successful!"
echo "=========================================="
echo "Output: build/bin/Mailven-${VERSION}.flatpak"
ls -lh build/bin/Mailven-${VERSION}.flatpak
INNERSCRIPT

chmod +x /tmp/flatpak-build-test-inner.sh

echo "Starting Docker container (ubuntu:24.04)..."
echo ""

docker run --rm -it --privileged \
  -v "$REPO_DIR:/workspace" \
  -v /tmp/flatpak-build-test-inner.sh:/build-script.sh \
  -w /workspace \
  -e HOST_UID="$(id -u)" \
  -e HOST_GID="$(id -g)" \
  ubuntu:noble-20260113 \
  /build-script.sh "$VERSION"

echo ""
echo "=========================================="
echo "Test completed!"
echo "=========================================="
echo ""
echo "If successful, the flatpak bundle is at:"
echo "  $REPO_DIR/build/bin/Mailven-${VERSION}.flatpak"
echo ""
