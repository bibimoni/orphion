#!/usr/bin/env bash

set -euo pipefail

TITLE="${ORPHION_LIVE_TITLE:-Shirokuma Cafe}"
EPISODE="${ORPHION_LIVE_EPISODE:-1}"
PROBE_SECONDS="${ORPHION_PROBE_SECONDS:-15}"
PROVIDER="${ORPHION_LIVE_PROVIDER:-catalog}"
ORPHION_BIN="${ORPHION_BIN:-orphion}"
FFMPEG_BIN="${FFMPEG_BIN:-ffmpeg}"
FFPROBE_BIN="${FFPROBE_BIN:-ffprobe}"
SUMMARY_FILE="${GITHUB_STEP_SUMMARY:-}"
WORK_DIR="${ORPHION_LIVE_WORK_DIR:-$(mktemp -d)}"
REMOVE_WORK_DIR=1
STAGE="initialization"

if [ -n "${ORPHION_LIVE_WORK_DIR:-}" ]; then
    REMOVE_WORK_DIR=0
    mkdir -p "$WORK_DIR"
fi

append_summary() {
    [ -n "$SUMMARY_FILE" ] || return 0
    printf '%s\n' "$*" >>"$SUMMARY_FILE"
}

cleanup() {
    status=$?
    if [ "$status" -eq 0 ]; then
        append_summary "**Result:** Passed"
    else
        append_summary "**Result:** Failed during ${STAGE}"
    fi
    if [ "$REMOVE_WORK_DIR" -eq 1 ]; then
        rm -rf "$WORK_DIR"
    fi
    exit "$status"
}
trap cleanup EXIT

write_config() {
    home_dir="$1"
    output_dir="$2"
    ffmpeg_path="$3"
    mkdir -p "$home_dir/.config/orphion"
    {
        printf 'output_dir: %s\n' "$output_dir"
        printf 'preferred_quality: 1080p\n'
        printf 'concurrency: 1\n'
        printf 'provider: %s\n' "$PROVIDER"
        printf 'ffmpeg_path: %s\n' "$ffmpeg_path"
    } >"$home_dir/.config/orphion/config.yaml"
}

find_single_mkv() {
    output_dir="$1"
    files="$(find "$output_dir" -type f -name '*.mkv' ! -name '*.part.mkv' -print)"
    count="$(printf '%s\n' "$files" | sed '/^$/d' | wc -l | tr -d '[:space:]')"
    if [ "$count" -ne 1 ]; then
        printf 'expected one MKV in %s, found %s\n' "$output_dir" "$count" >&2
        return 1
    fi
    printf '%s\n' "$files"
}

validate_mkv() {
    file="$1"
    duration="$("$FFPROBE_BIN" \
        -v error \
        -show_entries format=duration \
        -of default=noprint_wrappers=1:nokey=1 \
        "$file")"
    awk -v duration="$duration" 'BEGIN { exit !(duration + 0 > 0) }'
    printf '%s\n' "$duration"
}

append_summary "# Live Download Check"
append_summary
append_summary "- Title: \`${TITLE}\`"
append_summary "- Episode: \`${EPISODE}\`"
append_summary "- Probe duration: \`${PROBE_SECONDS}s\`"
append_summary

SEARCH_HOME="$WORK_DIR/search-home"
SEARCH_OUTPUT="$WORK_DIR/search-output"
mkdir -p "$SEARCH_OUTPUT"
write_config "$SEARCH_HOME" "$SEARCH_OUTPUT" "$FFMPEG_BIN"

STAGE="title search"
search_output="$(HOME="$SEARCH_HOME" "$ORPHION_BIN" search --type anime "$TITLE")"
title_ids="$(
    printf '%s\n' "$search_output" |
        awk -F '\t' -v title="$TITLE" '$2 == title { gsub(/^[ \t]+|[ \t]+$/, "", $1); print $1 }'
)"
title_count="$(printf '%s\n' "$title_ids" | sed '/^$/d' | wc -l | tr -d '[:space:]')"
if [ "$title_count" -ne 1 ]; then
    printf 'expected one exact search result for %s, found %s\n' "$TITLE" "$title_count" >&2
    exit 1
fi
title_id="$title_ids"

PROBE_HOME="$WORK_DIR/probe-home"
PROBE_OUTPUT="$WORK_DIR/probe-output"
PROBE_FFMPEG="$WORK_DIR/probe-ffmpeg"
cat >"$PROBE_FFMPEG" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
args=("$@")
last_index=$((${#args[@]} - 1))
output="${args[$last_index]}"
unset 'args[$last_index]'
exec "$REAL_FFMPEG" "${args[@]}" -t "$PROBE_SECONDS" "$output"
EOF
chmod +x "$PROBE_FFMPEG"
write_config "$PROBE_HOME" "$PROBE_OUTPUT" "$PROBE_FFMPEG"

STAGE="short stream probe"
REAL_FFMPEG="$FFMPEG_BIN" \
PROBE_SECONDS="$PROBE_SECONDS" \
HOME="$PROBE_HOME" \
"$ORPHION_BIN" download \
    --type anime \
    --title-id "$title_id" \
    --title "$TITLE" \
    --episodes "$EPISODE" \
    --output "$PROBE_OUTPUT"

STAGE="short probe validation"
probe_file="$(find_single_mkv "$PROBE_OUTPUT")"
probe_duration="$(validate_mkv "$probe_file")"
probe_bytes="$(wc -c <"$probe_file" | tr -d '[:space:]')"

FULL_HOME="$WORK_DIR/full-home"
FULL_OUTPUT="$WORK_DIR/full-output"
write_config "$FULL_HOME" "$FULL_OUTPUT" "$FFMPEG_BIN"

STAGE="complete episode download"
HOME="$FULL_HOME" \
"$ORPHION_BIN" download \
    --type anime \
    --title-id "$title_id" \
    --title "$TITLE" \
    --episodes "$EPISODE" \
    --output "$FULL_OUTPUT"

STAGE="complete episode validation"
full_file="$(find_single_mkv "$FULL_OUTPUT")"
full_duration="$(validate_mkv "$full_file")"
full_bytes="$(wc -c <"$full_file" | tr -d '[:space:]')"

append_summary "## Media Validation"
append_summary
append_summary "| Check | Duration | Bytes |"
append_summary "|---|---:|---:|"
append_summary "| Short probe | ${probe_duration}s | ${probe_bytes} |"
append_summary "| Complete episode | ${full_duration}s | ${full_bytes} |"
