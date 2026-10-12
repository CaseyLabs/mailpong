FROM ubuntu:noble-20260113

ENV DEBIAN_FRONTEND=noninteractive

# Toolchain versions. Keep in sync with go.mod and .github/workflows/ci.yml.
ARG GO_VERSION=1.25.14
ARG NODE_VERSION=24.21.0
ARG WAILS_VERSION=v2.16.0
ARG GNOME_RUNTIME=50

# Install build dependencies
RUN apt-get update && apt-get install -y \
    flatpak \
    flatpak-builder \
    git \
    libgtk-3-dev \
    libwebkit2gtk-4.1-dev \
    build-essential \
    pkg-config \
    wget \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Install Go and Node.js for the build architecture (amd64 or arm64)
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
        amd64) go_arch=amd64; node_arch=x64 ;; \
        arm64) go_arch=arm64; node_arch=arm64 ;; \
        *) echo "Unsupported architecture: $(dpkg --print-architecture)" >&2; exit 1 ;; \
    esac; \
    wget -qO- "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" | tar -xz -C /usr/local; \
    mkdir -p /usr/local/node; \
    wget -qO- "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" \
        | tar -xJ -C /usr/local/node --strip-components=1

ENV PATH="/usr/local/go/bin:/usr/local/node/bin:${PATH}"

# Install the Wails CLI version pinned in go.mod. GOBIN keeps it readable when
# build-flatpak-docker.sh runs the container as the host user.
RUN GOBIN=/usr/local/bin go install "github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}" \
    && rm -rf /root/go /root/.cache

# Add Flathub repository
RUN flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo

# Install the runtime used by build/flatpak/io.github.caseylabs.Mailven-dev.yml.
# That manifest packages the prebuilt binary, so no SDK extensions are needed.
RUN flatpak install -y --noninteractive flathub \
    "org.gnome.Platform//${GNOME_RUNTIME}" \
    "org.gnome.Sdk//${GNOME_RUNTIME}"

WORKDIR /workspace
