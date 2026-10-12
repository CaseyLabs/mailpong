---
sidebar_position: 1
pagination_next: getting-started/verify
---

# Linux

Mailven has no published releases yet, so build it from source. Clone the repository first:

```bash
git clone https://github.com/CaseyLabs/mailven.git
cd mailven
```

## Flatpak (local build)
---

Install `flatpak-builder` (for example `sudo apt install flatpak-builder`, `sudo dnf install flatpak-builder` or `sudo pacman -S flatpak-builder`), then build from the checkout and install for your user:

```bash
make flatpak-install
flatpak run io.github.caseylabs.Mailven
```

`make flatpak-install` also writes a `.flatpak` bundle under `build/bin/`. Mailven is not on Flathub. See [build/flatpak/README.md](../../../../build/flatpak/README.md) for the other build paths.

If Mailven doesn't launch or it behaves weirdly, try disabling DMABUF, which is known to be buggy for certain hardware combinations:

```bash
flatpak run --env=WEBKIT_DISABLE_DMABUF_RENDERER=1 io.github.caseylabs.Mailven
```

If that solves the issue, make it permanent with:

```bash
flatpak override --user --env=WEBKIT_DISABLE_DMABUF_RENDERER=1 io.github.caseylabs.Mailven
```

Flatpak data lives in `~/.var/app/io.github.caseylabs.Mailven/`.

## Native binary
---

1. Install the build dependencies and the WebKit2GTK runtime (Debian/Ubuntu shown; use `webkit2gtk4.1-devel` and `gtk3-devel` on Fedora, `webkit2gtk-4.1` and `gtk3` on Arch):

```bash
sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
```

2. You also need Go 1.26, Node.js 24 and the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation).

3. Build, then install the binary, desktop file and icon (`make install-linux PREFIX=$HOME/.local` installs for your user only):

```bash
make build
make install-linux
```

Run `make uninstall-linux` to remove it. Your data stays in `~/.local/share/mailven`, `~/.config/mailven` and `~/.cache/mailven` until you delete it yourself.

If you run into issues running Mailven, try:

```bash
export WEBKIT_DISABLE_DMABUF_RENDERER=1 && mailven
```

If this solves the issues, make the environment variable permanent for your user.

## AppImage
---

An AppImage for a `webkit2gtk-4.1` app that works on every distribution is not feasible, so Mailven does not provide one.

## Packages from other sources

Mailven is not packaged by any distribution or package manager. Packages named "Aerion" belong to the upstream project and do not contain Mailven.
