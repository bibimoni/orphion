#!/usr/bin/env bash
# dev-setup.sh — bootstrap an Orphion development environment.
# Usage: bash dev-setup.sh
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { printf '  %b[INFO]%b %s\n' "$CYAN" "$NC" "$*"; }
ok()    { printf '  %b[OK]%b   %s\n' "$GREEN" "$NC" "$*"; }
error() { printf '  %b[ERR]%b  %s\n' "$RED" "$NC" "$*" >&2; }

need() {
  if command -v "$1" >/dev/null 2>&1; then
    ok "$1 found"
  else
    error "$1 is required but not installed"
    exit 1
  fi
}

echo ""
echo "  Orphion Development Setup"
echo "  -------------------------"
echo ""

# ── Go ──────────────────────────────────────────────
info "Checking Go..."
need go
ok "Go $(go version | awk '{print $3}')"

# ── FFmpeg ──────────────────────────────────────────
info "Checking FFmpeg..."
if command -v ffmpeg >/dev/null 2>&1; then
  ok "FFmpeg $(ffmpeg -version 2>/dev/null | head -1 | awk '{print $3}')"
else
  info "FFmpeg not found (needed for integration tests)"
  case "$(uname -s)" in
    Darwin) info "Install with: brew install ffmpeg" ;;
    Linux)  info "Install with: sudo apt install ffmpeg" ;;
  esac
fi

# ── golangci-lint ────────────────────────────────────
info "Checking golangci-lint..."
if command -v golangci-lint >/dev/null 2>&1; then
  ok "golangci-lint $(golangci-lint version --short 2>/dev/null || echo installed)"
else
  info "Installing golangci-lint..."
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
  ok "golangci-lint installed"
fi

# ── Pre-commit ──────────────────────────────────────
info "Checking pre-commit..."
if command -v pre-commit >/dev/null 2>&1; then
  ok "pre-commit found"
  pre-commit install 2>/dev/null || true
  ok "pre-commit hooks installed"
else
  info "pre-commit not found (optional)"
  info "Install with: pip install pre-commit && pre-commit install"
fi

# ── Go tools ─────────────────────────────────────────
info "Installing Go tools..."
go install golang.org/x/tools/cmd/goimports@latest 2>/dev/null || true

# ── Download dependencies ────────────────────────────
info "Downloading Go module dependencies..."
go mod download

# ── Verify build ────────────────────────────────────
info "Verifying build..."
go build ./...
ok "Build succeeds"

# ── Run unit tests ───────────────────────────────────
info "Running unit tests..."
go test -race ./...
ok "Unit tests pass"

echo ""
ok "Development environment is ready!"
echo ""
echo "  Quick commands:"
echo "    go build ./...           # Build all packages"
echo "    go test -race ./...      # Run tests"
echo "    golangci-lint run ./...  # Run linter"
echo "    go run ./cmd/orphion     # Run locally"
echo ""
