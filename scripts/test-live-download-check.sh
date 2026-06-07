#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

BIN_DIR="$TEMP_DIR/bin"
LOG_FILE="$TEMP_DIR/commands.log"
SUMMARY_FILE="$TEMP_DIR/summary.md"
mkdir -p "$BIN_DIR"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

cat >"$BIN_DIR/orphion" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
    search)
        printf '  wrong-id\tShirokuma Cafe (Dub)\n'
        printf '  show-id\tShirokuma Cafe\n'
        ;;
    download)
        output=""
        title=""
        while [ "$#" -gt 0 ]; do
            case "$1" in
                --output) output="$2"; shift 2 ;;
                --title) title="$2"; shift 2 ;;
                *) shift ;;
            esac
        done
        ffmpeg_path="$(sed -n 's/^ffmpeg_path: //p' "$HOME/.config/orphion/config.yaml")"
        printf 'download ffmpeg=%s output=%s\n' "$ffmpeg_path" "$output" >>"$LIVE_CHECK_LOG"
        mkdir -p "$output/$title"
        printf 'fake mkv\n' >"$output/$title/Episode 01.mkv"
        ;;
    *) exit 2 ;;
esac
EOF

cat >"$BIN_DIR/ffmpeg" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

cat >"$BIN_DIR/ffprobe" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'ffprobe %s\n' "$*" >>"$LIVE_CHECK_LOG"
printf '12.500000\n'
EOF

chmod +x "$BIN_DIR/orphion" "$BIN_DIR/ffmpeg" "$BIN_DIR/ffprobe"

LIVE_CHECK_LOG="$LOG_FILE" \
ORPHION_BIN="$BIN_DIR/orphion" \
FFMPEG_BIN="$BIN_DIR/ffmpeg" \
FFPROBE_BIN="$BIN_DIR/ffprobe" \
GITHUB_STEP_SUMMARY="$SUMMARY_FILE" \
bash "$ROOT_DIR/scripts/live-download-check.sh"

[ "$(grep -c '^download ' "$LOG_FILE")" -eq 2 ] ||
    fail "expected short and full downloads"
grep -q 'ffmpeg=.*/probe-ffmpeg ' "$LOG_FILE" ||
    fail "short download did not use probe FFmpeg wrapper"
grep -q "ffmpeg=$BIN_DIR/ffmpeg " "$LOG_FILE" ||
    fail "full download did not use configured FFmpeg"
[ "$(grep -c '^ffprobe ' "$LOG_FILE")" -eq 2 ] ||
    fail "expected both outputs to be validated"
grep -q 'Live Download Check' "$SUMMARY_FILE" ||
    fail "summary heading is missing"
grep -q 'Shirokuma Cafe' "$SUMMARY_FILE" ||
    fail "summary title is missing"
if grep -Eq 'https?://|signed=' "$SUMMARY_FILE"; then
    fail "summary contains a URL"
fi

printf 'live download harness tests passed\n'
