#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export ORPHION_INSTALLER_TEST=1

# shellcheck source=../install.sh
source "$ROOT_DIR/install.sh"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_eq() {
    local want="$1"
    local got="$2"
    local message="$3"
    [ "$want" = "$got" ] || fail "$message: want $want, got $got"
}

test_detect_target() {
    ORPHION_UNAME_S=Darwin ORPHION_UNAME_M=arm64 detect_target
    assert_eq darwin "$TARGET_OS" "macOS target OS"
    assert_eq arm64 "$TARGET_ARCH" "Apple Silicon target architecture"

    ORPHION_UNAME_S=Linux ORPHION_UNAME_M=x86_64 detect_target
    assert_eq linux "$TARGET_OS" "Linux target OS"
    assert_eq amd64 "$TARGET_ARCH" "x86_64 target architecture"

    if ORPHION_UNAME_S=FreeBSD ORPHION_UNAME_M=amd64 detect_target 2>/dev/null; then
        fail "unsupported operating system was accepted"
    fi
}

test_archive_name() {
    assert_eq \
        "orphion_0.1.0_darwin_arm64.tar.gz" \
        "$(archive_name v0.1.0 darwin arm64)" \
        "release archive name"
}

test_resolve_version() {
    local got
    got="$(
        curl() {
            printf '%s\n' "https://github.com/bibimoni/orphion/releases/tag/v1.2.3"
        }
        resolve_version
    )"
    assert_eq "v1.2.3" "$got" "latest release tag"
}

test_checksum_and_install() {
    local temp_dir archive checksums install_dir
    temp_dir="$(mktemp -d)"
    archive="$temp_dir/orphion_0.1.0_darwin_arm64.tar.gz"
    checksums="$temp_dir/checksums.txt"
    install_dir="$temp_dir/bin"

    printf '#!/usr/bin/env sh\nprintf "orphion version v0.1.0\\n"\n' >"$temp_dir/orphion"
    chmod +x "$temp_dir/orphion"
    tar -C "$temp_dir" -czf "$archive" orphion
    (
        cd "$temp_dir"
        sha256_file "$(basename "$archive")" >"$checksums"
    )

    verify_checksum "$archive" "$checksums"
    install_archive "$archive" "$install_dir"
    [ -x "$install_dir/orphion" ] || fail "installed binary is not executable"
    assert_eq \
        "orphion version v0.1.0" \
        "$("$install_dir/orphion")" \
        "installed binary output"

    printf '0%.0s' {1..64} >"$checksums"
    printf '  %s\n' "$(basename "$archive")" >>"$checksums"
    if verify_checksum "$archive" "$checksums" 2>/dev/null; then
        fail "checksum mismatch was accepted"
    fi

    rm -rf "$temp_dir"
}

test_detect_target
test_archive_name
test_resolve_version
test_checksum_and_install

printf 'installer tests passed\n'
