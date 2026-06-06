# Migaku Compatibility Report

**Status:** Complete - failed compatibility gate
**Test date:** 2026-06-07

## Environment

| Item | Value |
|---|---|
| macOS | 26.4.1 (build 25E253) |
| Google Chrome | 148.0.7778.216 |
| Migaku extension | Installed and active |
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

Evidence: [`evidence/native-track-mp4-failure.png`](evidence/native-track-mp4-failure.png)

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | FAIL | Migaku overlay opened, but no media-player/subtitle-browser tools appeared. |
| Subtitle detected | FAIL | Native captions rendered in Chrome, but Migaku did not attach subtitle timeline controls. |
| Next line | FAIL | Subtitle browsing controls unavailable. |
| Previous line | FAIL | Subtitle browsing controls unavailable. |
| Replay current line | FAIL | Media replay control unavailable. |
| Sentence text captured | FAIL | Required mining workflow unavailable. |
| Audio captured | FAIL | Required mining workflow unavailable. |
| Screenshot captured | FAIL | Required mining workflow unavailable. |
| Usable card created | FAIL | Required mining workflow unavailable. |
| Works after seek | FAIL | Base detection failed. |
| Works after pause | FAIL | Base detection failed. |
| Works with 500 ms offset | FAIL | Base detection failed. |
| Works after fullscreen | FAIL | Base detection failed. |

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
| Video detected | FAIL | No media-player/subtitle-browser tools appeared. |
| Subtitle detected | FAIL | Visible Japanese text briefly received pitch-accent coloring from generic reading-support mode; no timed subtitle recognition occurred. |
| Next line | FAIL | Subtitle browsing controls unavailable. |
| Previous line | FAIL | Subtitle browsing controls unavailable. |
| Replay current line | FAIL | Media replay control unavailable. |
| Sentence text captured | FAIL | Required mining workflow unavailable. |
| Audio captured | FAIL | Required mining workflow unavailable. |
| Screenshot captured | FAIL | Required mining workflow unavailable. |
| Usable card created | FAIL | Required mining workflow unavailable. |
| Works after seek | FAIL | Base detection failed. |
| Works after pause | FAIL | Base detection failed. |
| Works with 500 ms offset | FAIL | Base detection failed. |
| Works after fullscreen | FAIL | Base detection failed. |

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
| Video detected | FAIL | Native track and selectable DOM together did not produce Migaku media tools. |
| Subtitle detected | FAIL | No timed subtitle recognition or subtitle browser appeared. |
| Next line | FAIL | Subtitle browsing controls unavailable. |
| Previous line | FAIL | Subtitle browsing controls unavailable. |
| Replay current line | FAIL | Media replay control unavailable. |
| Sentence text captured | FAIL | Required mining workflow unavailable. |
| Audio captured | FAIL | Required mining workflow unavailable. |
| Screenshot captured | FAIL | Required mining workflow unavailable. |
| Usable card created | FAIL | Required mining workflow unavailable. |
| Works after seek | FAIL | Base detection failed. |
| Works after pause | FAIL | Base detection failed. |
| Works with 500 ms offset | FAIL | Base detection failed. |
| Works after fullscreen | FAIL | Base detection failed. |

### Combined + HLS

| Behavior | Result | Evidence/notes |
|---|---|---|
| Video detected | FAIL | Switching transport from MP4 to HLS did not activate Migaku media tools. |
| Subtitle detected | FAIL | Native track and selectable DOM cues remained unavailable to Migaku's subtitle browser. |
| Next line | FAIL | Subtitle browsing controls unavailable. |
| Previous line | FAIL | Subtitle browsing controls unavailable. |
| Replay current line | FAIL | Media replay control unavailable. |
| Sentence text captured | FAIL | Required mining workflow unavailable. |
| Audio captured | FAIL | Required mining workflow unavailable. |
| Screenshot captured | FAIL | Required mining workflow unavailable. |
| Usable card created | FAIL | Required mining workflow unavailable. |
| Works after seek | FAIL | Base detection failed. |
| Works after pause | FAIL | Base detection failed. |
| Works with 500 ms offset | FAIL | Base detection failed. |
| Works after fullscreen | FAIL | Base detection failed. |

## Uploaded SRT

| Check | Result | Evidence/notes |
|---|---|---|
| User SRT loads | PASS | User selected `01.srt`; harness parsed and reported 356 cues. |
| Cues displayed | PASS | Chrome rendered the selected Japanese cue over the video. |
| Migaku behavior unchanged | PASS | Migaku media/subtitle-browser tools remained unavailable. |

## Migaku Automatic Subtitle Generation

| Check | Result | Evidence/notes |
|---|---|---|
| Generation offered for localhost | FAIL | `Shift+G` did not activate subtitle generation. |
| Generated cues usable | NOT AVAILABLE | Generation was unavailable. |
| Documented export available | NOT AVAILABLE | No generated subtitle workflow was available. |
| Export format | NOT AVAILABLE | |
| Export imports into harness | NOT AVAILABLE | |
| Documented API available | NOT FOUND | No documented third-party generated-subtitle API was identified. |

**Persistence result:** `generation_not_available`

Allowed final values:

```text
persist_via_documented_api
persist_via_manual_export_upload
usable_in_migaku_but_not_persistable
generation_not_available
```

## Approved Player Contract

**Arrangement:** None

**Required DOM rules:**

- No tested combination activated Migaku's supported-player integration.

**Required Migaku settings:**

- The generic reading-support mode may briefly apply pitch-accent coloring to
  visible Japanese DOM text. This does not provide subtitle browsing, timed
  navigation, audio capture, screenshot capture, or media card creation.

**Known limitations:**

- Migaku appears to require a site-specific or otherwise privileged player
  integration that is not exposed to this arbitrary localhost page.
- MP4 versus HLS transport did not change the result.
- Native WebVTT, selectable DOM cues, and both together did not change the
  result.

## Gate Decision

```text
PHASE 0 RESULT: FAIL
APPROVED PLAYER CONTRACT: NONE
PHASE 1 AUTHORIZED: NO
USER APPROVAL DATE:
```

## Conclusion

The Phase 0 acceptance criteria were not met. Orphion cannot claim direct
Migaku support for an arbitrary localhost HTML5 player using the tested public
browser mechanisms. Phase 1 remains blocked until the design is changed or
Migaku provides a supported integration path.
