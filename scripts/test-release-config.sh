#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GORELEASER="$ROOT_DIR/.goreleaser.yaml"
WORKFLOW="$ROOT_DIR/.github/workflows/release.yml"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

require_text() {
    local file="$1"
    local pattern="$2"
    local message="$3"
    grep -Fq -- "$pattern" "$file" || fail "$message"
}

[ -f "$GORELEASER" ] || fail ".goreleaser.yaml is missing"
[ -f "$WORKFLOW" ] || fail "release workflow is missing"

require_text "$GORELEASER" "goos:" "GoReleaser OS targets are missing"
require_text "$GORELEASER" "- darwin" "darwin release target is missing"
require_text "$GORELEASER" "- linux" "linux release target is missing"
require_text "$GORELEASER" "- amd64" "amd64 release target is missing"
require_text "$GORELEASER" "- arm64" "arm64 release target is missing"
require_text "$GORELEASER" "checksums.txt" "release checksum file is missing"
require_text "$GORELEASER" "github.com/distiled/orphion/internal/cli.Version={{.Tag}}" \
    "version linker flag is missing"
require_text "$GORELEASER" "draft: false" "release must not remain a draft"
require_text "$GORELEASER" "prerelease: false" "release must not be a prerelease"

require_text "$WORKFLOW" "tags:" "tag trigger is missing"
require_text "$WORKFLOW" "- \"v*\"" "v* tag trigger is missing"
require_text "$WORKFLOW" "go test -race ./..." "race tests are missing"
require_text "$WORKFLOW" "contents: write" "release write permission is missing"
require_text "$WORKFLOW" "goreleaser/goreleaser-action@v7" "GoReleaser v7 action is missing"
require_text "$WORKFLOW" "release --clean" "GoReleaser publish command is missing"

printf 'release configuration tests passed\n'
