# Mailven Email Client - Build System
# 
# Usage:
#   make build    - Build production binary
#   make dev      - Run in development mode
#   make help     - Show all available targets
#
# OAuth credentials are loaded from .env or .env.local files
# See .env.example for required variables

.PHONY: all build build-linux dev dev-race credentials generate clean test lint lint-brand lint-contrast help \
        install uninstall install-linux uninstall-linux \
        install-darwin uninstall-darwin build-windows-installer flatpak flatpak-install flatpak-dev

# Load environment variables from .env files.
# .env.local overrides .env. All OAuth credentials live in the root .env —
# extension packages no longer carry their own OAuth client vars.
-include .env
-include .env.local
export

# OAuth credentials compiled into the app. The `credentials` target writes
# them to the gitignored internal/oauth2/credentials_gen.go (built only with
# the mailven_creds tag) instead of passing -ldflags, so Wails and Go never
# print them.
#
#   GOOGLE_CLIENT_ID/SECRET   — mail's Google-verified client. Also backs
#                               first-party extensions for any scopes their
#                               manifest declares in
#                               first_party_uses_core_for_scopes (today:
#                               contacts.readonly). Surfaced as
#                               "Mailven – Google" in the picker.
#   MICROSOFT_CLIENT_ID       — mail's Azure AD app registration. Also
#                               backs microsoft-contacts and
#                               microsoft-calendar (Microsoft Graph
#                               doesn't gate scopes behind verification).
#                               Surfaced as "Mailven – Microsoft".
#   GOOGLE_TESTING_CLIENT_ID/SECRET — shared un-Google-verified test
#                               project for extensions that need broader
#                               scopes than the mail project carries
#                               (contacts.readwrite, full Calendar).
#                               Single client backs google-contacts AND
#                               google-calendar slots. Surfaced as
#                               "Mailven – Google (Testing)".
# Wails build tags
BUILD_TAGS := webkit2_41,mailven_creds

# NOTE: AppImage build target has been removed due to webkit bundling incompatibility.
# See archive/AppImage/README.md for details on what was tried and why it didn't work.
# Use Flatpak packaging instead for cross-distro distribution.

# Installation directories (can be overridden)
PREFIX ?= /usr/local
DESTDIR ?=

# Platform detection
UNAME_S := $(shell uname -s)

# Default target
all: build

## Build Targets

# Write OAuth credentials from .env into a gitignored Go source file
credentials:
	@GOOS= GOARCH= go run build/gencreds.go

# Build production binary
build: credentials
	@echo "Building Mailven..."
	@if [ -z "$(GOOGLE_CLIENT_ID)" ] && [ -z "$(MICROSOFT_CLIENT_ID)" ]; then \
		echo "Warning: No OAuth credentials configured. Gmail/Outlook OAuth will not work."; \
		echo "See .env.example for required variables."; \
	fi
	@wails build -tags $(BUILD_TAGS)
ifeq ($(UNAME_S),Darwin)
	@echo "Bundling LICENSE, NOTICE and THIRD_PARTY_NOTICES.md into Mailven.app..."
	cp LICENSE NOTICE THIRD_PARTY_NOTICES.md build/bin/Mailven.app/Contents/Resources/
	@echo "Ad-hoc signing Mailven.app (required for macOS notifications)..."
	codesign --force --deep --sign - build/bin/Mailven.app
endif

# Build for Linux specifically
build-linux: credentials
	@echo "Building Mailven for Linux..."
	@wails build -tags $(BUILD_TAGS),linux,production

# Build Flatpak from the Flathub manifest's pinned release (not this checkout)
flatpak:
	@echo "Building Flatpak..."
	./build/flatpak/build-local.sh

# Build a Flatpak bundle from this checkout and install it for the current user
flatpak-install:
	@echo "Building and installing Mailven Flatpak for the current user..."
	./build/flatpak/build-flatpak.sh --install

# Build a Flatpak bundle from this checkout without installing it
flatpak-dev:
	@echo "Building Flatpak from local source..."
	./build/flatpak/build-flatpak.sh

# Run in development mode with hot reload
dev: credentials
	@echo "Starting Mailven in development mode..."
	@wails dev -tags $(BUILD_TAGS)

# Run in development mode with Go's race detector enabled. Builds significantly
# slower and adds ~5-10x runtime overhead, but instruments every memory access
# and prints exactly which line + goroutines collide on any unsynchronized
# shared-memory access. Use this when chasing a suspected data race —
# reproduce the crash and the detector report points right at it.
dev-race: credentials
	@echo "Starting Mailven in development mode with -race..."
	@wails dev -tags $(BUILD_TAGS) -race

# Generate Wails TypeScript bindings
generate:
	@echo "Generating Wails bindings..."
	wails generate module

## Code Quality

# Run Go tests
test:
	@echo "Running tests..."
	go test ./...

# Run all linters (Go + frontend)
lint: lint-go lint-frontend lint-brand lint-contrast

# Run Go linter (requires golangci-lint)
lint-go:
	@echo "Running Go linter..."
	golangci-lint run

# Run frontend linter (ESLint)
lint-frontend:
	@echo "Running frontend linter..."
	cd frontend && npm run lint

# Brand check: stale old-name mentions outside tools/brand-check.allow
lint-brand:
	@echo "Running brand check..."
	./tools/brand-check.sh

# Contrast check: theme text tiers vs pane surfaces (>= 4.5:1)
lint-contrast:
	@echo "Running contrast check..."
	node tools/contrast-check.mjs

# Format Go code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

## Maintenance

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf build/bin
	rm -rf frontend/dist
	rm -rf AppDir
	rm -f mailven
	rm -f internal/oauth2/credentials_gen.go

# Clean downloaded tools (deprecated - AppImage removed)
tools-clean:
	@echo "Note: AppImage support has been removed. See archive/AppImage/ for details."
	@echo "Use 'make clean' to clean build artifacts."

# Install frontend dependencies
frontend-deps:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

# Update frontend dependencies
frontend-update:
	@echo "Updating frontend dependencies..."
	cd frontend && npm update

## Installation (Cross-Platform)

# Auto-detect platform and install
install:
ifeq ($(UNAME_S),Linux)
	$(MAKE) install-linux
else ifeq ($(UNAME_S),Darwin)
	$(MAKE) install-darwin
else
	@echo "For Windows, use 'make build-windows-installer' and run the generated installer."
	@echo "Or manually copy build/bin/mailven.exe to your preferred location."
endif

# Auto-detect platform and uninstall
uninstall:
ifeq ($(UNAME_S),Linux)
	$(MAKE) uninstall-linux
else ifeq ($(UNAME_S),Darwin)
	$(MAKE) uninstall-darwin
else
	@echo "For Windows, use Add/Remove Programs in Windows Settings."
endif

## Linux Installation

# hicolor icon sizes shipped on Linux (#395). Keep in sync with
# build/linux/install.sh ICON_SIZES and .github/workflows/release.yml.
ICON_SIZES ?= 32 48 64 128 256

# Install Mailven on Linux
install-linux: build
	@echo "Installing Mailven to $(DESTDIR)$(PREFIX)..."
	install -Dm755 build/bin/mailven "$(DESTDIR)$(PREFIX)/bin/mailven"
	@for sz in $(ICON_SIZES); do \
		install -Dm644 "build/linux/icons/$${sz}x$${sz}/io.github.caseylabs.Mailven.png" \
			"$(DESTDIR)$(PREFIX)/share/icons/hicolor/$${sz}x$${sz}/apps/io.github.caseylabs.Mailven.png"; \
	done
	install -Dm644 build/linux/icons/scalable/io.github.caseylabs.Mailven.svg "$(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/io.github.caseylabs.Mailven.svg"
	install -Dm644 build/linux/io.github.caseylabs.Mailven.desktop "$(DESTDIR)$(PREFIX)/share/applications/io.github.caseylabs.Mailven.desktop"
	@echo "Updating icon cache..."
	-gtk-update-icon-cache -f -t "$(DESTDIR)$(PREFIX)/share/icons/hicolor" 2>/dev/null || true
	@echo ""
	@echo "Installation complete!"
	@echo "You may need to log out and back in for the application to appear in your menu."
	@echo ""
	@echo "To set Mailven as your default email client:"
	@echo "  xdg-mime default io.github.caseylabs.Mailven.desktop x-scheme-handler/mailto"

# Uninstall Mailven from Linux
uninstall-linux:
	@echo "Uninstalling Mailven from $(DESTDIR)$(PREFIX)..."
	rm -f "$(DESTDIR)$(PREFIX)/bin/mailven"
	@for sz in $(ICON_SIZES); do \
		rm -f "$(DESTDIR)$(PREFIX)/share/icons/hicolor/$${sz}x$${sz}/apps/io.github.caseylabs.Mailven.png"; \
	done
	rm -f "$(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/io.github.caseylabs.Mailven.svg"
	rm -f "$(DESTDIR)$(PREFIX)/share/applications/io.github.caseylabs.Mailven.desktop"
	-gtk-update-icon-cache -f -t "$(DESTDIR)$(PREFIX)/share/icons/hicolor" 2>/dev/null || true
	@echo "Uninstallation complete!"

## macOS Installation

# Install Mailven on macOS
install-darwin: build
	@echo "Installing Mailven.app to /Applications..."
	@if [ -d "/Applications/Mailven.app" ]; then \
		echo "Removing existing installation..."; \
		rm -rf "/Applications/Mailven.app"; \
	fi
	cp -R "build/bin/Mailven.app" "/Applications/"
	@echo "Re-signing installed copy..."
	codesign --force --deep --sign - "/Applications/Mailven.app"
	@echo ""
	@echo "Installation complete!"
	@echo "Mailven is now available in /Applications."

# Uninstall Mailven from macOS
uninstall-darwin:
	@echo "Uninstalling Mailven from /Applications..."
	rm -rf "/Applications/Mailven.app"
	@echo "Uninstallation complete!"

## Windows Installation

# Build Windows installer (requires NSIS)
build-windows-installer: credentials
	@echo "Building Windows installer..."
	@wails build -tags $(BUILD_TAGS) -nsis
	@echo ""
	@echo "Installer created at build/bin/mailven-amd64-installer.exe"

## Help

# Show available targets
help:
	@echo "Mailven Email Client - Build System"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build        - Build production binary"
	@echo "  make build-linux  - Build for Linux with production tags"
	@echo "  make flatpak      - Build Flatpak from the Flathub manifest's pinned release"
	@echo "  make flatpak-install - Build Flatpak from this checkout and install it"
	@echo "  make flatpak-dev  - Build Flatpak bundle from this checkout (no install)"
	@echo "  make dev          - Run in development mode with hot reload"
	@echo "  make generate     - Generate Wails TypeScript bindings"
	@echo ""
	@echo "Installation (auto-detects platform):"
	@echo "  make install      - Install Mailven (Linux/macOS)"
	@echo "  make uninstall    - Uninstall Mailven (Linux/macOS)"
	@echo ""
	@echo "Platform-Specific Installation:"
	@echo "  make install-linux      - Install on Linux to $(PREFIX)"
	@echo "  make uninstall-linux    - Uninstall from Linux"
	@echo "  make install-darwin     - Install on macOS to /Applications"
	@echo "  make uninstall-darwin   - Uninstall from macOS"
	@echo "  make build-windows-installer - Build NSIS installer for Windows"
	@echo ""
	@echo "Code Quality:"
	@echo "  make test          - Run Go tests"
	@echo "  make lint          - Run all linters (Go + frontend)"
	@echo "  make lint-go       - Run Go linter only (requires golangci-lint)"
	@echo "  make lint-frontend - Run frontend linter only (ESLint)"
	@echo "  make fmt           - Format Go code"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make frontend-deps   - Install frontend dependencies"
	@echo "  make frontend-update - Update frontend dependencies"
	@echo ""
	@echo "Environment Variables:"
	@echo "  PREFIX             - Installation prefix (default: /usr/local)"
	@echo "  DESTDIR            - Staging directory for packaging"
	@echo "  GOOGLE_CLIENT_ID     - Google OAuth Client ID"
	@echo "  GOOGLE_CLIENT_SECRET - Google OAuth Client Secret (optional)"
	@echo "  MICROSOFT_CLIENT_ID  - Microsoft OAuth Client ID"
	@echo "  GOOGLE_TESTING_CLIENT_ID/SECRET - Google testing client for extensions"
	@echo ""
	@echo "See .env.example for details on obtaining OAuth credentials."
