#!/bin/bash
# Copy Mailven files to Flathub repository for submission/update
# Usage: ./release.sh /path/to/flathub/io.github.caseylabs.Mailven

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <flathub-repo-path>"
    echo "Example: $0 ~/flathub/io.github.caseylabs.Mailven"
    exit 1
fi

FLATHUB_DIR="$1"

if [ ! -d "$FLATHUB_DIR" ]; then
    echo "ERROR: Directory not found: $FLATHUB_DIR"
    exit 1
fi

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "=========================================="
echo "Mailven Flathub Release Helper"
echo "=========================================="
echo "Source: $SCRIPT_DIR"
echo "Target: $FLATHUB_DIR"
echo ""
echo "NOTE: This is a from-source build. The Flathub repo contains the manifest"
echo "      plus vendored dependency files. The app is built from source during"
echo "      the Flathub build process. No OAuth credentials are bundled."
echo ""
echo "Copying files..."
echo ""

# Copy manifest
echo "Copying manifest..."
cp "${SCRIPT_DIR}/io.github.caseylabs.Mailven.yml" \
   "${FLATHUB_DIR}/io.github.caseylabs.Mailven.yml"
echo "   io.github.caseylabs.Mailven.yml"

# Copy Go module vendoring sources
echo "Copying Go module sources..."
cp "${SCRIPT_DIR}/go.mod.yml" "${FLATHUB_DIR}/go.mod.yml"
cp "${SCRIPT_DIR}/modules.txt" "${FLATHUB_DIR}/modules.txt"
echo "   go.mod.yml"
echo "   modules.txt"

# Copy npm package vendoring sources
echo "Copying npm package sources..."
cp "${SCRIPT_DIR}/node-sources.json" "${FLATHUB_DIR}/node-sources.json"
echo "   node-sources.json"


echo ""
echo "=========================================="
echo "All files copied successfully!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. cd $FLATHUB_DIR"
echo "2. git status  # Review changes"
echo "3. git add ."
echo "4. git commit -m \"Update to vX.X.XX\""
echo "5. git push"
echo "=========================================="
