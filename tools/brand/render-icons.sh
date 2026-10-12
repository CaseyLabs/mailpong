#!/usr/bin/env bash
# Regenerate every raster icon from the master SVG (brand/mailven.svg).
# Needs rsvg-convert (preferred) or headless Chromium, plus ImageMagick.
set -euo pipefail

cd "$(dirname "$0")/../.."
SRC=brand/mailven.svg
ID=io.github.caseylabs.Mailven
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# render <size> <out.png>
if command -v rsvg-convert >/dev/null; then
  render() { rsvg-convert -w "$1" -h "$1" -o "$2" "$SRC"; }
else
  chrome=${CHROME:-$(command -v chromium chromium-browser google-chrome 2>/dev/null | head -n1 || true)}
  [ -z "$chrome" ] && [ -x /opt/pw-browsers/chromium ] && chrome=/opt/pw-browsers/chromium
  [ -n "$chrome" ] || { echo "need rsvg-convert or Chromium" >&2; exit 1; }
  render() {
    printf '<body style="margin:0;background:transparent"><img src="file://%s/%s" width=%s height=%s></body>' \
      "$PWD" "$SRC" "$1" "$1" >"$tmp/r.html"
    "$chrome" --headless --no-sandbox --allow-file-access-from-files --hide-scrollbars \
      --default-background-color=00000000 --window-size="$1,$1" \
      --screenshot="$2" "file://$tmp/r.html" >/dev/null 2>&1
  }
fi

mkdir -p build/linux/icons/scalable build/windows frontend/src/assets/images
render 1024 build/appicon.png
for s in 32 48 64 128 256; do
  mkdir -p "build/linux/icons/${s}x${s}"
  render "$s" "build/linux/icons/${s}x${s}/$ID.png"
done
cp "$SRC" "build/linux/icons/scalable/$ID.svg"
render 256 build/linux/mailven.png
render 256 frontend/src/assets/images/logo-universal.png

for s in 16 24 32 48 64 128 256; do render "$s" "$tmp/$s.png"; done
convert "$tmp"/16.png "$tmp"/24.png "$tmp"/32.png "$tmp"/48.png "$tmp"/64.png \
  "$tmp"/128.png "$tmp"/256.png build/windows/icon.ico
echo "icons regenerated from $SRC"
