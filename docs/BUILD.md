# BUILD

Set .env file

### 🔨 Building from Source
---

**Flatpak (Recommended for Linux):**

```bash
# Install flatpak-builder
sudo dnf install flatpak-builder  # Fedora
sudo apt install flatpak-builder  # Ubuntu/Debian
sudo pacman -S flatpak-builder    # Arch

# Optional: set your own Microsoft and Google OAuth creds for dev and test builds (MailPong bundles none)
cp .env.example .env
# Fill in your own creds, or skip this to use Custom credentials in Settings

# Build from this checkout and install for your user
make flatpak-install

# Run
flatpak run io.github.caseylabs.MailPong
```

`make flatpak-install` installs into your user Flatpak scope and also writes a bundle under `build/bin/`. See [build/flatpak/README.md](../build/flatpak/README.md) for other build paths and Flathub submission details.

**Native Binary:**

```bash
# Install dependencies (Ubuntu/Debian)
sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev

# Optional: set your own Microsoft and Google OAuth creds for dev and test builds (MailPong bundles none)
cp .env.example .env
# Fill in your own creds, or skip this to use Custom credentials in Settings

# Build
make build

# Run
./build/bin/mailpong
```

The `make` build and dev targets write the OAuth credentials from `.env` / `.env.local` into the gitignored `internal/oauth2/credentials_gen.go` and compile it with the `mailpong_creds` build tag, so the values never appear in build output. `make clean` removes the file.

