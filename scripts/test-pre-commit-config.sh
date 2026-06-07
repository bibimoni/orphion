#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="$ROOT_DIR/.pre-commit-config.yaml"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

require_text() {
    local pattern="$1"
    local message="$2"
    grep -Fq -- "$pattern" "$CONFIG" || fail "$message"
}

[ "$(grep -c '^[[:space:]]*- id:' "$CONFIG")" -eq 1 ] ||
    fail "pre-commit must define exactly one hook"

require_text "repo: local" "hook must use the local repository"
require_text "id: golangci-lint" "golangci-lint hook is missing"
require_text "entry: golangci-lint run --new-from-rev=HEAD ./..." \
    "hook must lint changes relative to HEAD"
require_text "language: system" "hook must use the installed linter"
require_text "pass_filenames: false" "hook must not receive file arguments"
require_text "always_run: true" "hook must run for staged deletions"

if grep -Eq 'go-unit-tests|go-build|go-mod-tidy|go-fmt|go-imports|check-yaml' "$CONFIG"; then
    fail "pre-commit contains hooks other than golangci-lint"
fi

printf 'pre-commit configuration tests passed\n'
