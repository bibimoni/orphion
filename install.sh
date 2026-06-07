#!/usr/bin/env bash
# ==============================================================================
# Orphion Installer
# ==============================================================================
# Orphion is a Go CLI that searches an AllAnime-derived source and downloads
# selected episodes as MKV files through system FFmpeg.
#
# Quick one-liner:
#   curl -fsSL https://raw.githubusercontent.com/bibimoni/orphion/main/install.sh | bash
#
# What this script does:
#   1. Detects your OS (macOS, Linux, WSL)
#   2. Installs Go 1.24+ if missing (brew, apt, or official tarball)
#   3. Clones the Orphion repository
#   4. Builds the binary to /usr/local/bin/orphion
#   5. Installs golangci-lint for development
#   6. Prints FFmpeg installation instructions
# ==============================================================================
# License: MIT
# Repository: https://github.com/bibimoni/orphion
# ==============================================================================

set -euo pipefail

# --- Colors ------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# --- Helper Functions ---------------------------------------------------------
info()    { echo -e "  ${CYAN}[INFO]${NC}    $*"; }
ok()      { echo -e "  ${GREEN}[OK]${NC}      $*"; }
warn()    { echo -e "  ${YELLOW}[WARN]${NC}    $*"; }
error()   { echo -e "  ${RED}[ERROR]${NC}   $*" >&2; }
step()    { echo -e "\n${BOLD}==> $*${NC}"; }

# --- Banner -------------------------------------------------------------------
banner() {
    echo -e "${CYAN}"
    echo "   ___        _     _           "
    echo "  / _ \\\\ _ __| |__ (_) ___  _ __  "
    echo " | | | | '__| '_ \\\\| |/ _ \\\\| '_ \\\\ "
    echo " | |_| | |  | | | | | (_) | | | |"
    echo "  \\\\___/|_|  |_| |_|_|\\___/|_| |_|"
    echo "                                  "
    echo -e "${NC}Anime & Drama Downloader"
    echo "==========================================="
    echo ""
}

# ==============================================================================
# Step 0: OS Detection
# ==============================================================================
detect_os() {
    step "Detecting operating system..."

    UNAME="$(uname -s)"
    case "$UNAME" in
        Darwin)
            OS="macOS"
            PM="homebrew"
            info "Detected macOS $(sw_vers -productVersion 2>/dev/null || echo 'unknown')"
            ;;
        Linux)
            OS="linux"
            if   command -v apt-get &>/dev/null; then PM="apt"
            elif command -v pacman  &>/dev/null; then PM="pacman"
            elif command -v dnf     &>/dev/null; then PM="dnf"
            elif command -v yum     &>/dev/null; then PM="yum"
            else PM="unknown"; fi
            info "Detected Linux ($(uname -r))  |  Package manager: ${PM}"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            OS="windows"
            PM="manual"
            info "Detected Windows (${UNAME})"
            ;;
        *)
            OS="unknown"
            PM="unknown"
            error "Unrecognized OS: $UNAME"
            ;;
    esac
    echo ""
}

# --- Determine Go download URL -------------------------------------------------
get_go_download_url() {
    local os_go
    case "$OS" in
        macOS)   os_go="darwin";;
        linux)   os_go="linux";;
        windows) os_go="windows";;
        *)       error "Unsupported OS for Go download: $OS"; return 1;;
    esac

    local arch="$(uname -m)"
    local arch_go
    case "$arch" in
        x86_64|amd64)  arch_go="amd64";;
        aarch64|arm64)  arch_go="arm64";;
        *)              error "Unsupported architecture: $arch"; return 1;;
    esac

    echo "https://golang.org/dl/go1.24.4.${os_go}-${arch_go}.tar.gz"
}

# ==============================================================================
# Step 1: Install Go
# ==============================================================================
install_go() {
    step "Checking Go installation..."

    # Already installed?
    if command -v go &>/dev/null; then
        current="$(go version 2>/dev/null | awk '{print $3}')"
        ok "Go already installed: ${current}"
        return 0
    fi

    warn "Go is not installed. Installing Go 1.24.4..."
    echo ""

    # macOS: Homebrew
    if [ "$PM" = "homebrew" ]; then
        info "Installing Go via Homebrew..."
        brew install go@1.24 || {
            error "Installation via Homebrew failed."
            return 1
        }
        ok "Go installed via Homebrew: $(go version)"
        return 0
    fi

    # Linux: apt
    if [ "$PM" = "apt" ]; then
        info "Installing Go via apt..."
        sudo apt-get update -qq 2>/dev/null || true
        sudo apt-get install -y -qq golang-go 2>/dev/null || {
            warn "apt-provided Go may be outdated; falling back to manual tarball install..."
        }
        if command -v go &>/dev/null; then
            ok "Go installed via apt: $(go version)"
            return 0
        fi
    fi

    # Manual tarball install (Linux / macOS fallback)
    if [ "$OS" = "linux" ] || [ "$OS" = "macOS" ]; then
        info "Installing Go from the official tarball..."
        GO_URL="$(get_go_download_url)" || exit 1
        info "Downloading Go from: $GO_URL"

        TMPDIR="$(mktemp -d)"
        trap "rm -rf $TMPDIR" EXIT

        curl -fsSL "$GO_URL" -o "$TMPDIR/go.tar.gz"
        sudo tar -C /usr/local -xzf "$TMPDIR/go.tar.gz"
        export PATH="/usr/local/go/bin:$PATH"

        ok "Go installed: $(go version)"
        return 0
    fi

    error "Cannot install Go automatically on this OS."
    echo ""
    echo "  Please install Go 1.24+ from https://go.dev/dl/"
    echo ""
    return 1
}

# ==============================================================================
# Step 2: FFmpeg Check (print instructions, don't auto-install)
# ==============================================================================
check_ffmpeg() {
    step "Checking FFmpeg..."

    if command -v ffmpeg &>/dev/null; then
        ok "FFmpeg already installed: $(ffmpeg -version 2>/dev/null | head -1)"
        return 0
    fi

    warn "FFmpeg is not installed (required for downloads)."
    echo ""
    echo -e "  ${BOLD}Install FFmpeg for your system:${NC}"
    echo ""

    case "$OS" in
        macOS)
            echo    "    brew install ffmpeg"
            ;;
        linux)
            case "$PM" in
                apt)    echo "    sudo apt install ffmpeg";;
                pacman) echo "    sudo pacman -S ffmpeg";;
                dnf)    echo "    sudo dnf install ffmpeg";;
                *)      echo "    See https://ffmpeg.org/download.html";;
            esac
            ;;
        windows)
            echo "    Download from https://ffmpeg.org/download.html"
            ;;
        *)
            echo "    See https://ffmpeg.org/download.html"
            ;;
    esac
    echo ""
}

# ==============================================================================
# Step 3: Install golangci-lint
# ==============================================================================
install_golangci_lint() {
    step "Checking golangci-lint..."

    if command -v golangci-lint &>/dev/null; then
        ok "golangci-lint already installed: $(golangci-lint --version 2>/dev/null | head -1)"
        return 0
    fi

    warn "golangci-lint not installed; installing..."
    if command -v brew &>/dev/null; then
        brew install golangci-lint
    else
        # Official install script (Linux/macOS)
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
            sh -s -- -b "$(go env GOPATH)/bin" v1.58.0 2>/dev/null || {
            warn "Could not install golangci-lint automatically."
            return 0
        }
    fi

    if command -v golangci-lint &>/dev/null; then
        ok "golangci-lint installed: $(golangci-lint --version 2>/dev/null | head -1)"
    else
        warn "golangci-lint installation could not be verified."
    fi
}

# ==============================================================================
# Step 4: Clone Repository
# ==============================================================================
clone_repo() {
    step "Setting up Orphion..."

    local repo_url="https://github.com/bibimoni/orphion.git"
    local clone_dir="${ORPHION_HOME:-$HOME/Projects/orphion}"

    if [ -d "$clone_dir/.git" ]; then
        info "Repository already exists at $clone_dir"
        if [ "${ORPHION_UPDATE:-}" = "1" ]; then
            info "Pulling latest changes..."
            git -C "$clone_dir" pull || warn "Could not pull from remote; using cached version."
        fi
    else
        info "Cloning repository to $clone_dir..."
        git clone "$repo_url" "$clone_dir" 2>/dev/null || {
            error "Failed to clone $repo_url to $clone_dir"
            exit 2
        }
        ok "Repository cloned to $clone_dir"
    fi

    echo "$clone_dir"
}

# ==============================================================================
# Step 5: Build Binary
# ==============================================================================
build_binary() {
    local clone_dir="$1"
    step "Building Orphion..."

    # Ensure we're using the correct Go binary
    if [ -d /usr/local/go/bin ]; then
        export PATH="/usr/local/go/bin:$PATH"
    fi

    cd "$clone_dir"
    info "Building..."

    go build -trimpath -ldflags="-s -w" -o /usr/local/bin/orphion ./cmd/orphion || {
        error "Build failed."
        exit 3
    }

    chmod +x /usr/local/bin/orphion
    ok "Binary installed to /usr/local/bin/orphion"
}

# ==============================================================================
# Step 6: Verify Installation
# ==============================================================================
verify_installation() {
    step "Verifying installation..."

    if ! command -v /usr/local/bin/orphion &>/dev/null; then
        error "Binary not found at /usr/local/bin/orphion"
        return 1
    fi

    local version
    version="$(/usr/local/bin/orphion version 2>&1 || echo 'unknown')"
    echo -e "  ${GREEN}Version:${NC} ${version}"
}

# ==============================================================================
# Summary
# ==============================================================================
print_summary() {
    echo ""
    echo "==========================================="
    echo -e "  ${GREEN}${BOLD}Orphion installation complete!${NC}"
    echo ""
    echo -e "  To start, run:  ${CYAN}orphion${NC}"
    echo "  For help:       orphion --help"
    echo "  To configure:    orphion config init"
    echo ""
    echo "  Quick start:"
    echo "    1. orphion config init       -- write default config"
    echo "    2. orphion search Sorcery   -- search for titles"
    echo -e "    3. orphion download --title-id ID --episodes \\"1-3\\""
    echo ""
}

# ==============================================================================
# Main
# ==============================================================================
main() {
    banner
    detect_os
    install_go
    check_ffmpeg
    install_golangci_lint

    local clone_dir
    clone_dir="$(clone_repo)"
    build_binary "$clone_dir"
    verify_installation
    print_summary
}

main "$@"
