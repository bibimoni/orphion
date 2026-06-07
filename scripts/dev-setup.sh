#!/usr/bin/env bash
# ==============================================================================
# Orphion — Manual Build, Install & Test
# ==============================================================================
# Usage:
#   bash scripts/dev-setup.sh          # build + install + verify
#   bash scripts/dev-setup.sh --test   # also run full test suite
#   bash scripts/dev-setup.sh --clean  # remove installed binary first
#
# This script builds Orphion from source, installs it to a local path
# (defaults to /usr/local/bin), and verifies the binary works.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALL_DIR="${ORPHION_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="orphion"
RUN_TESTS=0
CLEAN_FIRST=0

# --- Colors -------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { printf '  %b[INFO]%b    %s\n' "$CYAN" "$NC" "$*"; }
ok()    { printf '  %b[OK]%b      %s\n' "$GREEN" "$NC" "$*"; }
warn()  { printf '  %b[WARN]%b    %s\n' "$YELLOW" "$NC" "$*"; }
error() { printf '  %b[ERROR]%b   %s\n' "$RED" "$NC" "$*" >&2; }
step()  { printf '\n%b==> %s%b\n' "$BOLD" "$*" "$NC"; }

# --- Parse Args ----------------------------------------------------------------
for arg in "$@"; do
    case "$arg" in
        --test)  RUN_TESTS=1 ;;
        --clean) CLEAN_FIRST=1 ;;
        *)       warn "Unknown argument: $arg" ;;
    esac
done

# --- Preflight Checks ----------------------------------------------------------
step "Preflight checks"

if ! command -v go &>/dev/null; then
    error "Go is not installed. Install Go 1.24+ from https://go.dev/dl/"
    exit 1
fi
ok "Go: $(go version)"

if ! command -v ffmpeg &>/dev/null; then
    warn "FFmpeg not found — downloads will fail. Install with:"
    info "  macOS:  brew install ffmpeg"
    info "  Ubuntu: sudo apt install ffmpeg"
    info "  Arch:   sudo pacman -S ffmpeg"
else
    ok "FFmpeg: $(ffmpeg -version 2>/dev/null | head -1)"
fi

# --- Clean ---------------------------------------------------------------------
if [ "$CLEAN_FIRST" -eq 1 ]; then
    step "Cleaning previous installation"
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        if [ -w "$INSTALL_DIR/$BINARY_NAME" ]; then
            rm "$INSTALL_DIR/$BINARY_NAME"
        elif command -v sudo &>/dev/null; then
            sudo rm "$INSTALL_DIR/$BINARY_NAME"
        else
            error "Cannot remove $INSTALL_DIR/$BINARY_NAME (no write access, no sudo)"
            exit 1
        fi
        ok "Removed $INSTALL_DIR/$BINARY_NAME"
    else
        info "No previous installation found"
    fi
fi

# --- Test ----------------------------------------------------------------------
if [ "$RUN_TESTS" -eq 1 ]; then
    step "Running tests"
    cd "$ROOT_DIR"
    go test -race ./...
    ok "All tests passed"
fi

# --- Build ---------------------------------------------------------------------
step "Building Orphion"

cd "$ROOT_DIR"

# Get the version from git, or fall back to "dev"
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo "dev")"
LDFLAGS="-s -w -X github.com/distiled/orphion/internal/cli.Version=$VERSION"

go build -trimpath -ldflags "$LDFLAGS" -o "$ROOT_DIR/dist/$BINARY_NAME" ./cmd/orphion
ok "Built dist/$BINARY_NAME (version: $VERSION)"

# --- Install -------------------------------------------------------------------
step "Installing to $INSTALL_DIR"

if [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "$ROOT_DIR/dist/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
elif command -v sudo &>/dev/null; then
    info "Administrator access required for $INSTALL_DIR"
    sudo install -m 0755 "$ROOT_DIR/dist/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
    error "Cannot write to $INSTALL_DIR — set ORPHION_INSTALL_DIR to a writable directory"
    info "Example: ORPHION_INSTALL_DIR=~/.local/bin bash scripts/dev-setup.sh"
    exit 1
fi
ok "Installed to $INSTALL_DIR/$BINARY_NAME"

# --- Verify --------------------------------------------------------------------
step "Verifying installation"

INSTALLED_VERSION="$("$INSTALL_DIR/$BINARY_NAME" version 2>&1 || echo 'unknown')"
ok "$INSTALLED_VERSION"

# --- Config Check --------------------------------------------------------------
step "Checking config"

CONFIG_FILE="${HOME}/.config/orphion/config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    ok "Config exists at $CONFIG_FILE"
else
    info "No config file yet — it will be auto-created on first run"
fi

# --- Summary -------------------------------------------------------------------
printf '\n'
echo "==========================================="
printf '  %b%bOrphion is ready!%b\n' "$GREEN" "$BOLD" "$NC"
printf '\n'
printf '  %borphion%b                    Interactive mode\n' "$CYAN" "$NC"
printf '  %borphion search "Frieren"%b   Search for titles\n' "$CYAN" "$NC"
printf '  %borphion download --title "Frieren" --episodes 1%b\n' "$CYAN" "$NC"
printf '  %borphion version%b           Show version\n' "$CYAN" "$NC"
printf '  %borphion --help%b             Show all commands\n' "$CYAN" "$NC"
printf '\n'
echo "==========================================="
