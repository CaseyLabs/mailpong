# Flatpak Packaging for Mailven

This directory contains files for building and distributing Mailven as a Flatpak.

## Files

- `io.github.caseylabs.Mailven-dev.yml` - Dev manifest (packages pre-built host binary, no compilation)
- `io.github.caseylabs.Mailven.metainfo.xml` - AppStream metadata
- `build-flatpak.sh` - Dev build script (uses `-dev.yml`; `--install` also installs the result)
- `build-local.sh` - From-source local build script (uses flathub manifest, which builds its pinned release rather than this checkout)
- `test-build.sh` - CI build test script (Docker container)
- `build-flatpak-docker.sh`, `Dockerfile` - Docker-based build (builds the binary in a container, then packages it with `-dev.yml`; toolchain versions in `Dockerfile` track `go.mod` and CI)
- `flathub/` - Flathub submission files (from-source manifests + vendored deps)

## Prerequisites

Install flatpak-builder:

```bash
# Fedora
sudo dnf install flatpak-builder

# Ubuntu/Debian
sudo apt install flatpak-builder

# Arch
sudo pacman -S flatpak-builder
```

Add Flathub repository (if not already added):

```bash
flatpak remote-add --if-not-exists --user flathub https://flathub.org/repo/flathub.flatpakrepo
```

Install required runtimes and SDKs:

```bash
flatpak install --user flathub org.gnome.Platform//50 org.gnome.Sdk//50
flatpak install --user flathub org.freedesktop.Sdk.Extension.golang//25.08
flatpak install --user flathub org.freedesktop.Sdk.Extension.node24//25.08
```

The local-checkout build also needs the host build prerequisites listed in [the project build guide](../../docs/BUILD.md), including WebKitGTK 4.1 development files.

## Building Locally

### Build and install this checkout

This builds the Linux binary on the host, packages it with the local dev manifest, installs it into your user Flatpak scope, and writes a bundle to `build/bin/Mailven-dev.flatpak`.

```bash
make flatpak-install
```

Run the installed app with:

```bash
flatpak run io.github.caseylabs.Mailven
```

`make flatpak-dev` runs the same local-checkout build but only writes the bundle; it does not install it.

The dev manifest copies only the built binary, `build/linux/`, and the metainfo file into the Flatpak build, so `.env` files and the generated OAuth credentials source never reach `.flatpak-builder/`.

### Build the Flathub source manifest

`make flatpak` uses the Flathub manifest and builds its pinned release (not this checkout) inside the Flatpak sandbox, with no bundled OAuth credentials (users enter their own in Settings). It writes a bundle under `build/bin/` but does not install it.

```bash
make flatpak
```

For a direct script invocation, use:

```bash
./build/flatpak/build-local.sh
```

## Validation

Validate the metainfo file:

```bash
# Install appstream-util
sudo dnf install libappstream-glib  # Fedora
sudo apt install appstream-util      # Ubuntu/Debian

# Validate (from project root)
appstream-util validate build/flatpak/io.github.caseylabs.Mailven.metainfo.xml
```

Validate the desktop file:

```bash
desktop-file-validate build/linux/io.github.caseylabs.Mailven.desktop
```

## Submitting to Flathub

See [`flathub/README.md`](flathub/README.md) for complete Flathub submission instructions.

## Additional Resources

- [Flatpak Documentation](https://docs.flatpak.org/)
- [Flathub Submission Guide](https://github.com/flathub/flathub/wiki/App-Submission)
- [AppStream Guidelines](https://www.freedesktop.org/software/appstream/docs/)
- [Flatpak Builder Manifest](https://docs.flatpak.org/en/latest/flatpak-builder-command-reference.html)

## Advantages Over AppImage

- WebKit provided by GNOME runtime (no bundling needed)
- Works on ALL Linux distros consistently
- Sandboxing is properly implemented
- Automatic updates via Flatpak
- Centralized distribution through Flathub
- Better integration with desktop environments
- Shared runtime = smaller download size

## Optional OAuth credentials shim

The manifests bundle no OAuth credentials. A maintainer who wants Flatpak builds
with built-in Google or Microsoft sign-in can add a `type: file` source that
downloads a credentials binary from a release and an `install -Dm755` build
command for it, the way earlier Aerion manifests did. Install it next to the app as `/app/bin/mailven-creds`, where the runtime looks for it. Nothing here ships one today.
