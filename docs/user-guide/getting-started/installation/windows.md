---
sidebar_position: 3
---

# Windows

Mailven has no published releases yet, so build it from source. You need Go 1.26, Node.js 24, the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) and, for the installer, [NSIS](https://nsis.sourceforge.io/).

```bash
git clone https://github.com/CaseyLabs/mailven.git
cd mailven
make build
```

`make build` writes `build/bin/mailven.exe`. Move it somewhere safe, create a shortcut for your taskbar or start menu, and run it. Windows SmartScreen may warn about an unsigned app: click "More info" and then "Run anyway".

To build the installer, run `make build-windows-installer`. It produces `mailven-<arch>-installer.exe` in `build/bin/`.

## WebView2 Runtime

Mailven requires the Microsoft Edge WebView2 Runtime. On Windows 11 and recent Windows 10 updates, this is pre-installed. If needed, download it from [Microsoft](https://developer.microsoft.com/en-us/microsoft-edge/webview2/).

Your data is stored in `%APPDATA%\Mailven` and `%LOCALAPPDATA%\Mailven\Cache`.

## Packages from other sources

Mailven is not packaged by Chocolatey or any other package manager. Packages named "Aerion" belong to the upstream project and do not contain Mailven.
