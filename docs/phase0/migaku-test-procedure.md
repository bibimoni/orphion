# Migaku Compatibility Test Procedure

## Purpose

This procedure determines whether the installed Migaku Chrome extension can
mine Orphion-style streamed video. It tests six combinations of media source
and subtitle DOM arrangement.

Phase 1 remains blocked until the completed report records a passing result and
explicit user authorization.

## Prerequisites

- macOS with Google Chrome.
- An active Migaku extension and working card-creation setup.
- Docker with Compose.
- The Phase 0 harness running at `http://127.0.0.1:8090`.

Start the harness from the repository root:

```bash
phase0/scripts/run
```

Before testing, record:

```text
macOS version:
Chrome version:
Migaku extension version/channel:
Migaku language/course:
Relevant Migaku settings:
Test date:
```

Use the generated fixture, not copyrighted anime media. It contains a moving
test pattern, a 440 Hz tone, and three subtitle cues.

## Reset Between Cases

1. Close any existing harness tab.
2. Open `chrome://extensions`.
3. Confirm Migaku is enabled.
4. If a previous result appears affected by stale permissions, open Chrome
   site settings for `127.0.0.1` and reset permissions.
5. Open a fresh tab at `http://127.0.0.1:8090`.
6. Click **Reset fixture**.
7. Select the media source and subtitle arrangement for the case.
8. Confirm the status line shows the expected combination and three cues.

## Test Cases

Run every combination:

1. Native track + MP4.
2. Native track + HLS.
3. Selectable DOM + MP4.
4. Selectable DOM + HLS.
5. Combined + MP4.
6. Combined + HLS.

For each case:

1. Start playback and confirm moving video plus audible tone.
2. Open Migaku.
3. Record whether Migaku detects the video.
4. Record whether Migaku detects the current subtitle text.
5. Move to the next subtitle line through Migaku.
6. Move to the previous subtitle line through Migaku.
7. Replay the current subtitle line through Migaku.
8. Create a test card containing captured sentence text.
9. Confirm the card contains an audio clip from the tone.
10. Confirm the card contains a video screenshot.
11. Seek to another position and repeat subtitle detection.
12. Pause and replay the current line.
13. Set subtitle offset to `500` ms and confirm cue timing shifts later.
14. Enter and leave fullscreen, then repeat detection and capture.
15. Record pass/fail and concise evidence for every item.

Do not mark a case as passing when card creation succeeds without all required
text, audio, and screenshot fields.

## Uploaded SRT Check

For the strongest case:

1. Click **Upload SRT**.
2. Select `phase0/fixtures/sample.srt`.
3. Confirm the status reports three cues.
4. Repeat detection, cue navigation, replay, and card creation.
5. Record whether upload changes Migaku behavior.

## Automatic Subtitle Generation

Using the strongest case:

1. Open Migaku's subtitle-generation controls.
2. Record whether generation is offered for `127.0.0.1`.
3. If offered, run generation and record whether generated cues appear.
4. Search Migaku's visible UI and official documentation for an export action.
5. If export exists, record format and whether the exported file can be loaded
   with the harness SRT upload.
6. Record whether a documented API exposes generated cues.

Choose exactly one persistence result:

```text
persist_via_documented_api
persist_via_manual_export_upload
usable_in_migaku_but_not_persistable
generation_not_available
```

Do not inspect private extension storage, intercept extension traffic, or rely
on undocumented internal APIs.

## Acceptance Decision

Phase 0 passes only when at least one arrangement works with both MP4 and HLS
and satisfies every mandatory behavior:

- Video and subtitle detection.
- Previous/next line navigation.
- Current-line replay.
- Sentence text capture.
- Audio capture.
- Screenshot capture.
- Usable card creation.
- Operation after seek, pause, offset change, and fullscreen.

The approved player contract must identify:

- Required native `<track>` presence.
- Required selectable DOM cue presence.
- Required attributes or normal-document placement.
- Any required Migaku settings.
- Known limitations.

Complete `docs/phase0/migaku-compatibility-report.md`. Do not begin Phase 1
until the user sets `PHASE 1 AUTHORIZED: YES`.

