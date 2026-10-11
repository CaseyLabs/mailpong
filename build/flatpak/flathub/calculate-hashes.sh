#!/bin/bash
# Pin the from-source Flathub manifest to a release tag and its commit
# Usage: ./calculate-hashes.sh v0.1.17

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.1.17"
    exit 1
fi

VERSION="$1"
REPO="https://github.com/caseylabs/mailpong"

echo "=========================================="
echo "Flathub Manifest Updater (from-source)"
echo "=========================================="
echo "Version: $VERSION"
echo "Repository: $REPO"
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
MANIFEST="${SCRIPT_DIR}/io.github.caseylabs.MailPong.yml"

if [ ! -f "$MANIFEST" ]; then
    echo "ERROR: Manifest file not found: $MANIFEST"
    exit 1
fi

# Create backup
cp "$MANIFEST" "${MANIFEST}.backup"

# ----- Step 1: Update git tag and commit hash -----
echo "Resolving commit hash for tag ${VERSION}..."

COMMIT_HASH=$(git ls-remote "${REPO}.git" "refs/tags/${VERSION}" | tail -1 | awk '{print $1}')
if [ -z "$COMMIT_HASH" ]; then
    echo "ERROR: Could not resolve commit hash for tag ${VERSION}"
    exit 1
fi
echo "   Tag: ${VERSION}"
echo "   Commit: ${COMMIT_HASH}"
echo ""

# Update tag in manifest
sed -i "s|tag: v[0-9.]*[-a-zA-Z0-9]*|tag: ${VERSION}|" "$MANIFEST"

# Update commit hash in manifest
sed -i "s|commit: [0-9a-f]\{40\}|commit: ${COMMIT_HASH}|" "$MANIFEST"

echo ""
echo "Manifest updated successfully!"
echo "   Backup saved: ${MANIFEST}.backup"
echo ""
echo "=========================================="
echo "Summary:"
echo "=========================================="
echo ""
echo "Git source:"
echo "  tag: ${VERSION}"
echo "  commit: ${COMMIT_HASH}"
echo ""
echo "=========================================="
echo "Next steps:"
echo "1. Review changes: diff ${MANIFEST}.backup ${MANIFEST}"
echo "2. Test build: flatpak-builder --force-clean build-dir ${MANIFEST}"
echo "=========================================="
