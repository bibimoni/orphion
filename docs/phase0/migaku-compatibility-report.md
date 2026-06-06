# Migaku Compatibility Report

**Status:** Ready for manual test
**Test date:** 2026-06-07

## Environment

| Item | Value |
|---|---|
| macOS | 26.4.1 (build 25E253) |
| Google Chrome | 148.0.7778.216 |
| Migaku extension | Not recorded |
| Migaku version/channel | Not recorded |
| Migaku language/course | Not recorded |
| Relevant Migaku settings | Not recorded |
| Harness commit | `69f5725eb31033d37a8878ec1c7b37f5797757f6` |
| Harness URL | `http://127.0.0.1:8090` |

## Automated Harness Verification

| Check | Result | Evidence |
|---|---|---|
| TypeScript tests | PASS | 12 tests across parser, player variants, and controls |
| Production build | PASS | Vite production build |
| MP4 fixture decode | PASS | FFmpeg 7.1 verification |
| HLS fixture decode | PASS | FFmpeg 7.1 verification |
| Container build | PASS | `orphion-phase0:0.1.0` |
| HTML endpoint | PASS | HTTP 200 |
| HLS endpoint | PASS | HTTP 200, `application/vnd.apple.mpegurl` |
| MP4 endpoint | PASS | HTTP 200, `video/mp4` |
| SRT endpoint | PASS | HTTP 200, `application/x-subrip` |

## Manual Test Matrix

Use `PASS`, `FAIL`, or `NOT TESTED`.

### Native Track + MP4

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | NOT TESTED | |
| Subtitle detected | NOT TESTED | |
| Next line | NOT TESTED | |
| Previous line | NOT TESTED | |
| Replay current line | NOT TESTED | |
| Sentence text captured | NOT TESTED | |
| Audio captured | NOT TESTED | |
| Screenshot captured | NOT TESTED | |
| Usable card created | NOT TESTED | |
| Works after seek | NOT TESTED | |
| Works after pause | NOT TESTED | |
| Works with 500 ms offset | NOT TESTED | |
| Works after fullscreen | NOT TESTED | |

### Native Track + HLS

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | NOT TESTED | |
| Subtitle detected | NOT TESTED | |
| Next line | NOT TESTED | |
| Previous line | NOT TESTED | |
| Replay current line | NOT TESTED | |
| Sentence text captured | NOT TESTED | |
| Audio captured | NOT TESTED | |
| Screenshot captured | NOT TESTED | |
| Usable card created | NOT TESTED | |
| Works after seek | NOT TESTED | |
| Works after pause | NOT TESTED | |
| Works with 500 ms offset | NOT TESTED | |
| Works after fullscreen | NOT TESTED | |

### Selectable DOM + MP4

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | NOT TESTED | |
| Subtitle detected | NOT TESTED | |
| Next line | NOT TESTED | |
| Previous line | NOT TESTED | |
| Replay current line | NOT TESTED | |
| Sentence text captured | NOT TESTED | |
| Audio captured | NOT TESTED | |
| Screenshot captured | NOT TESTED | |
| Usable card created | NOT TESTED | |
| Works after seek | NOT TESTED | |
| Works after pause | NOT TESTED | |
| Works with 500 ms offset | NOT TESTED | |
| Works after fullscreen | NOT TESTED | |

### Selectable DOM + HLS

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | NOT TESTED | |
| Subtitle detected | NOT TESTED | |
| Next line | NOT TESTED | |
| Previous line | NOT TESTED | |
| Replay current line | NOT TESTED | |
| Sentence text captured | NOT TESTED | |
| Audio captured | NOT TESTED | |
| Screenshot captured | NOT TESTED | |
| Usable card created | NOT TESTED | |
| Works after seek | NOT TESTED | |
| Works after pause | NOT TESTED | |
| Works with 500 ms offset | NOT TESTED | |
| Works after fullscreen | NOT TESTED | |

### Combined + MP4

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | NOT TESTED | |
| Subtitle detected | NOT TESTED | |
| Next line | NOT TESTED | |
| Previous line | NOT TESTED | |
| Replay current line | NOT TESTED | |
| Sentence text captured | NOT TESTED | |
| Audio captured | NOT TESTED | |
| Screenshot captured | NOT TESTED | |
| Usable card created | NOT TESTED | |
| Works after seek | NOT TESTED | |
| Works after pause | NOT TESTED | |
| Works with 500 ms offset | NOT TESTED | |
| Works after fullscreen | NOT TESTED | |

### Combined + HLS

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | NOT TESTED | |
| Subtitle detected | NOT TESTED | |
| Next line | NOT TESTED | |
| Previous line | NOT TESTED | |
| Replay current line | NOT TESTED | |
| Sentence text captured | NOT TESTED | |
| Audio captured | NOT TESTED | |
| Screenshot captured | NOT TESTED | |
| Usable card created | NOT TESTED | |
| Works after seek | NOT TESTED | |
| Works after pause | NOT TESTED | |
| Works with 500 ms offset | NOT TESTED | |
| Works after fullscreen | NOT TESTED | |

## Uploaded SRT

| Check | Result | Evidence/notes |
|---|---|---|
| `sample.srt` loads | NOT TESTED | |
| Three cues displayed | NOT TESTED | |
| Migaku behavior unchanged | NOT TESTED | |

## Migaku Automatic Subtitle Generation

| Check | Result | Evidence/notes |
|---|---|---|
| Generation offered for localhost | NOT TESTED | |
| Generated cues usable | NOT TESTED | |
| Documented export available | NOT TESTED | |
| Export format | NOT TESTED | |
| Export imports into harness | NOT TESTED | |
| Documented API available | NOT TESTED | |

**Persistence result:** `NOT TESTED`

Allowed final values:

```text
persist_via_documented_api
persist_via_manual_export_upload
usable_in_migaku_but_not_persistable
generation_not_available
```

## Approved Player Contract

**Arrangement:** Not selected

**Required DOM rules:**

- Not recorded.

**Required Migaku settings:**

- Not recorded.

**Known limitations:**

- Not recorded.

## Gate Decision

```text
PHASE 0 RESULT: NOT TESTED
APPROVED PLAYER CONTRACT: NOT SELECTED
PHASE 1 AUTHORIZED: NO
USER APPROVAL DATE:
```
