---
sidebar_position: 2
pagination_next: getting-started/verify
---

# macOS

Mailven has no published releases yet, so build it from source. You need Go 1.26, Node.js 24, the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) and the Xcode command line tools.

1. Clone and build:

```bash
git clone https://github.com/CaseyLabs/mailven.git
cd mailven
make build
```

2. The result is `build/bin/mailven.app`. Run `make install-darwin` to copy it to your Applications folder, or drag it there yourself.
3. The app you build is not signed or notarized, so macOS may show a security warning on first launch.

## Bypassing Gatekeeper

If macOS blocks the app:

1. Right-click (or Control-click) on `mailven.app`
2. Select "Open" from the context menu
3. Click "Open" in the dialog that appears

Alternatively, run this command:

```bash
xattr -cr /Applications/mailven.app
```

Your data is stored in `~/Library/Application Support/Mailven` and `~/Library/Caches/Mailven`.
