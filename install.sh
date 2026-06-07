#!/usr/bin/env bash

set -euo pipefail

REPOSITORY="${ORPHION_REPOSITORY:-bibimoni/orphion}"
INSTALL_DIR="${ORPHION_INSTALL_DIR:-/usr/local/bin}"
TEMP_DIR=""

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

banner() {
    printf '%b\n' "$CYAN"
    printf '%s\n' '   ___        _     _'
    printf '%s\n' '  / _ \ _ __| |__ (_) ___  _ __'
    printf '%s\n' " | | | | '__| '_ \| |/ _ \| '_ \\"
    printf '%s\n' ' | |_| | |  | | | | | (_) | | | |'
    printf '%s\n' '  \___/|_|  |_| |_|_|\___/|_| |_|'
    printf '%bAnime & Drama Downloader%b\n' "$NC" "$NC"
    printf '%s\n\n' '==========================================='
}

detect_target() {
    local uname_s="${ORPHION_UNAME_S:-$(uname -s)}"
    local uname_m="${ORPHION_UNAME_M:-$(uname -m)}"

    case "$uname_s" in
        Darwin) TARGET_OS="darwin" ;;
        Linux)  TARGET_OS="linux" ;;
        *)
            error "Unsupported operating system: $uname_s"
            return 1
            ;;
    esac

    case "$uname_m" in
        x86_64|amd64) TARGET_ARCH="amd64" ;;
        arm64|aarch64) TARGET_ARCH="arm64" ;;
        *)
            error "Unsupported architecture: $uname_m"
            return 1
            ;;
    esac
}

resolve_version() {
    if [ -n "${ORPHION_VERSION:-}" ]; then
        printf '%s\n' "$ORPHION_VERSION"
        return
    fi

    local latest_url resolved_url version
    latest_url="https://github.com/$REPOSITORY/releases/latest"
    resolved_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$latest_url")"
    version="${resolved_url##*/}"

    case "$version" in
        v[0-9]*.[0-9]*.[0-9]*) printf '%s\n' "$version" ;;
        *)
            error "Could not determine the latest Orphion release"
            return 1
            ;;
    esac
}

archive_name() {
    local version="${1#v}"
    local target_os="$2"
    local target_arch="$3"
    printf 'orphion_%s_%s_%s.tar.gz\n' "$version" "$target_os" "$target_arch"
}

sha256_file() {
    local file="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file"
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$file"
    else
        error "SHA-256 tool not found (need sha256sum or shasum)"
        return 1
    fi
}

verify_checksum() {
    local archive="$1"
    local checksums="$2"
    local filename expected actual
    filename="$(basename "$archive")"
    expected="$(awk -v filename="$filename" '$2 == filename { print $1; exit }' "$checksums")"

    if [ -z "$expected" ]; then
        error "No checksum found for $filename"
        return 1
    fi

    actual="$(sha256_file "$archive" | awk '{ print $1 }')"
    if [ "$actual" != "$expected" ]; then
        error "Checksum verification failed for $filename"
        return 1
    fi
}

install_archive() {
    local archive="$1"
    local install_dir="$2"
    local extract_dir
    extract_dir="$(mktemp -d)"
    tar -xzf "$archive" -C "$extract_dir"

    if [ ! -f "$extract_dir/orphion" ]; then
        rm -rf "$extract_dir"
        error "Release archive does not contain the orphion binary"
        return 1
    fi

    if mkdir -p "$install_dir" 2>/dev/null && [ -w "$install_dir" ]; then
        install -m 0755 "$extract_dir/orphion" "$install_dir/orphion"
    elif command -v sudo >/dev/null 2>&1; then
        info "Administrator access is required for $install_dir"
        sudo mkdir -p "$install_dir"
        sudo install -m 0755 "$extract_dir/orphion" "$install_dir/orphion"
    else
        rm -rf "$extract_dir"
        error "Cannot write to $install_dir; set ORPHION_INSTALL_DIR to a writable directory"
        return 1
    fi

    rm -rf "$extract_dir"
}

check_ffmpeg() {
    if command -v ffmpeg >/dev/null 2>&1; then
        ok "FFmpeg found: $(ffmpeg -version 2>/dev/null | head -1)"
        return
    fi

    warn "FFmpeg is required for downloads but is not installed"
    case "$TARGET_OS" in
        darwin) info "Install it with: brew install ffmpeg" ;;
        linux)  info "Install it with your package manager, for example: sudo apt install ffmpeg" ;;
    esac
}

main() {
    local version archive base_url

    banner
    step "Detecting platform"
    detect_target
    ok "Target: ${TARGET_OS}/${TARGET_ARCH}"

    step "Finding latest release"
    version="$(resolve_version)"
    archive="$(archive_name "$version" "$TARGET_OS" "$TARGET_ARCH")"
    ok "Release: $version"

    TEMP_DIR="$(mktemp -d)"
    trap 'rm -rf "$TEMP_DIR"' EXIT
    base_url="https://github.com/$REPOSITORY/releases/download/$version"

    step "Downloading Orphion"
    curl -fsSL "$base_url/$archive" -o "$TEMP_DIR/$archive"
    curl -fsSL "$base_url/checksums.txt" -o "$TEMP_DIR/checksums.txt"
    verify_checksum "$TEMP_DIR/$archive" "$TEMP_DIR/checksums.txt"
    ok "Checksum verified"

    step "Installing Orphion"
    install_archive "$TEMP_DIR/$archive" "$INSTALL_DIR"
    ok "Installed to $INSTALL_DIR/orphion"
    printf '  Version: %s\n' "$version"

    step "Checking runtime dependency"
    check_ffmpeg

    printf '\nRun %borphion --help%b to get started.\n' "$CYAN" "$NC"
}

if [ "${ORPHION_INSTALLER_TEST:-}" != "1" ]; then
    main "$@"
fi
