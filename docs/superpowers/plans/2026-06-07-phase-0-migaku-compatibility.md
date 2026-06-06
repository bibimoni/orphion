# Phase 0 Migaku Compatibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a disposable local test harness and produce evidence showing whether the installed Migaku Chrome extension can mine streamed video in the DOM arrangements proposed for Orphion.

**Architecture:** A small static TypeScript application serves deterministic same-origin video/HLS fixtures and three subtitle-rendering variants. It has no database, provider integration, production backend, or reusable Phase 1 application code. The final deliverable is a manual compatibility report that the user must approve before Phase 1 starts.

**Tech Stack:** Node.js 24.1.0, npm with committed lockfile, TypeScript 5.8.3, Vite 6.3.5, hls.js 1.6.5, Vitest 3.2.1, Docker Compose 2.35.1, nginx 1.28.0-alpine

---

## File Map

```text
phase0/
├── package.json                         # pinned harness dependencies/scripts
├── package-lock.json                    # exact npm dependency graph
├── tsconfig.json                        # strict TypeScript configuration
├── vite.config.ts                       # local fixture server configuration
├── index.html                           # harness entry point
├── src/
│   ├── main.ts                          # variant selection and player wiring
│   ├── player.ts                        # native video/HLS lifecycle
│   ├── subtitles.ts                     # SRT parsing and VTT/cue generation
│   ├── variants.ts                      # track, DOM, and combined renderers
│   └── style.css                        # minimal test-only presentation
├── tests/
│   ├── subtitles.test.ts                # deterministic parser tests
│   └── variants.test.ts                 # DOM contract tests
├── fixtures/
│   ├── sample.srt                       # known subtitle timings/text
│   ├── sample.mp4                       # short browser-compatible test media
│   └── hls/                             # deterministic same-origin HLS fixture
├── Dockerfile                           # pinned test-server image
└── compose.yaml                         # localhost-only harness service
docs/
└── phase0/
    ├── migaku-test-procedure.md          # exact manual test steps
    └── migaku-compatibility-report.md    # completed evidence and decision
```

### Task 1: Scaffold the Disposable Harness

**Files:**
- Create: `phase0/package.json`
- Create: `phase0/package-lock.json`
- Create: `phase0/tsconfig.json`
- Create: `phase0/vite.config.ts`
- Create: `phase0/index.html`
- Create: `phase0/src/main.ts`
- Create: `phase0/src/style.css`
- Create: `phase0/.gitignore`

- [ ] **Step 1: Create pinned package metadata**

Use exact dependency versions, without caret or tilde ranges:

```json
{
  "name": "@orphion/phase0-migaku-harness",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0",
    "build": "tsc --noEmit && vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "hls.js": "1.6.5"
  },
  "devDependencies": {
    "@types/node": "22.15.29",
    "jsdom": "26.1.0",
    "typescript": "5.8.3",
    "vite": "6.3.5",
    "vitest": "3.2.1"
  }
}
```

- [ ] **Step 2: Generate and commit the exact npm lockfile**

Run inside the pinned Node container:

```bash
docker run --rm \
  -v "$PWD/phase0:/work" \
  -w /work \
  node:24.1.0-bookworm-slim \
  npm install --package-lock-only
```

Expected: `phase0/package-lock.json` exists and records the exact dependency graph.

- [ ] **Step 3: Add strict TypeScript and Vite configuration**

Configure ES2022, DOM libraries, strict mode, no emit, and Vitest `jsdom`.
Configure Vite to serve `fixtures/` under a same-origin path and fail if the
configured port is occupied.

- [ ] **Step 4: Add a plain harness shell**

`index.html` must contain:

```html
<main id="app"></main>
<script type="module" src="/src/main.ts"></script>
```

`main.ts` initially renders:

```text
Orphion Migaku Compatibility Harness
Variant: not selected
```

- [ ] **Step 5: Verify build**

Run:

```bash
docker run --rm \
  -v "$PWD/phase0:/work" \
  -w /work \
  node:24.1.0-bookworm-slim \
  sh -c "npm ci && npm run build"
```

Expected: TypeScript and Vite build pass.

- [ ] **Step 6: Commit**

```bash
git add phase0
git commit -m "chore: scaffold Migaku compatibility harness"
```

### Task 2: Implement and Test SRT Parsing

**Files:**
- Create: `phase0/src/subtitles.ts`
- Create: `phase0/tests/subtitles.test.ts`
- Create: `phase0/fixtures/sample.srt`

- [ ] **Step 1: Write failing parser tests**

Cover:

- CRLF and LF input.
- Multiline cue text.
- Comma millisecond separators.
- Optional numeric cue indexes.
- UTF-8 Japanese text.
- Rejection of end times before start times.

Use this public type:

```ts
export interface SubtitleCue {
  index: number;
  startMs: number;
  endMs: number;
  text: string;
}

export function parseSrt(input: string): SubtitleCue[];
export function cuesToWebVtt(cues: SubtitleCue[]): string;
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
docker run --rm \
  -v "$PWD/phase0:/work" \
  -w /work \
  node:24.1.0-bookworm-slim \
  sh -c "npm ci && npm test -- tests/subtitles.test.ts"
```

Expected: FAIL because `subtitles.ts` does not exist.

- [ ] **Step 3: Implement the minimal parser**

Parse blocks structurally, normalize line endings, validate timestamps, retain
cue text without language-specific processing, and emit valid `WEBVTT` with
period millisecond separators.

- [ ] **Step 4: Run parser tests**

Expected: all subtitle tests pass.

- [ ] **Step 5: Commit**

```bash
git add phase0/src/subtitles.ts phase0/tests/subtitles.test.ts phase0/fixtures/sample.srt
git commit -m "feat: parse SRT cues in compatibility harness"
```

### Task 3: Add Deterministic Media Fixtures

**Files:**
- Create: `phase0/fixtures/sample.mp4`
- Create: `phase0/fixtures/hls/master.m3u8`
- Create: `phase0/fixtures/hls/media.m3u8`
- Create: `phase0/fixtures/hls/segment-000.ts`
- Create: `phase0/fixtures/hls/segment-001.ts`
- Create: `phase0/fixtures/README.md`

- [ ] **Step 1: Generate a short browser-compatible source fixture**

Use a pinned FFmpeg image:

```bash
docker run --rm \
  -v "$PWD/phase0/fixtures:/out" \
  jrottenberg/ffmpeg:7.1-alpine \
  -f lavfi -i testsrc2=size=1280x720:rate=30 \
  -f lavfi -i sine=frequency=440:sample_rate=48000 \
  -t 12 \
  -c:v libx264 -pix_fmt yuv420p \
  -c:a aac -b:a 128k \
  -movflags +faststart \
  /out/sample.mp4
```

Expected: a 12-second MP4 containing visible motion and audible tone.

- [ ] **Step 2: Generate HLS from the source fixture**

Run:

```bash
docker run --rm \
  -v "$PWD/phase0/fixtures:/work" \
  -w /work \
  jrottenberg/ffmpeg:7.1-alpine \
  -i sample.mp4 \
  -c copy \
  -hls_time 6 \
  -hls_list_size 0 \
  -hls_segment_filename "hls/segment-%03d.ts" \
  hls/media.m3u8
```

Create `master.m3u8` pointing to `media.m3u8`.

- [ ] **Step 3: Document fixture checksums and generation commands**

`fixtures/README.md` records SHA-256 checksums and the exact pinned image tags.

- [ ] **Step 4: Verify the HLS playlist**

Run:

```bash
docker run --rm \
  -v "$PWD/phase0/fixtures:/work" \
  -w /work \
  jrottenberg/ffmpeg:7.1-alpine \
  -v error -i hls/master.m3u8 -f null -
```

Expected: exit 0 with no decode errors.

- [ ] **Step 5: Commit**

```bash
git add phase0/fixtures
git commit -m "test: add deterministic media fixtures"
```

### Task 4: Implement the Three Player Variants

**Files:**
- Create: `phase0/src/player.ts`
- Create: `phase0/src/variants.ts`
- Create: `phase0/tests/variants.test.ts`
- Modify: `phase0/src/main.ts`
- Modify: `phase0/src/style.css`

- [ ] **Step 1: Write failing DOM-contract tests**

Assert:

- Every variant contains a normal `<video>` in the main document.
- No variant uses an iframe.
- `track` variant contains a `<track kind="subtitles">`.
- `dom` variant contains an `aria-live="polite"` selectable cue element.
- `combined` variant contains both.
- Cue text changes when simulated playback time crosses cue boundaries.
- Offset adjustment shifts cue selection without mutating source cues.

- [ ] **Step 2: Run tests to verify failure**

Expected: FAIL because player and variant implementations do not exist.

- [ ] **Step 3: Implement player lifecycle**

Expose:

```ts
export interface HarnessPlayer {
  video: HTMLVideoElement;
  destroy(): void;
  setOffsetMs(offsetMs: number): void;
}

export function createPlayer(
  sourceUrl: string,
  cues: SubtitleCue[],
  variant: "track" | "dom" | "combined"
): HarnessPlayer;
```

Use native HLS when available and hls.js otherwise. Revoke generated VTT object
URLs and destroy hls.js instances during cleanup.

- [ ] **Step 4: Implement visible variant switching**

The harness offers buttons for:

- Native WebVTT track.
- Selectable DOM cues.
- Combined track and DOM cues.

It also exposes:

- MP4/HLS source selection.
- SRT file input.
- Offset input in milliseconds.
- Current cue index and text.
- A reset button.

- [ ] **Step 5: Run unit and build verification**

Run:

```bash
npm test
npm run build
```

Expected: all tests and build pass inside the pinned container.

- [ ] **Step 6: Commit**

```bash
git add phase0/src phase0/tests
git commit -m "feat: add Migaku player variants"
```

### Task 5: Containerize the Harness Without Floating Versions

**Files:**
- Create: `phase0/Dockerfile`
- Create: `phase0/nginx.conf`
- Create: `phase0/compose.yaml`
- Create: `phase0/scripts/run`

- [ ] **Step 1: Write a pinned multi-stage Dockerfile**

Use:

```dockerfile
FROM node:24.1.0-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.28.0-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
COPY fixtures /usr/share/nginx/html/fixtures
```

- [ ] **Step 2: Configure correct HLS MIME types and localhost publishing**

Compose must publish only:

```yaml
ports:
  - "127.0.0.1:8090:80"
```

No Compose image or Docker `FROM` may use `latest`.

- [ ] **Step 3: Add a run script**

`phase0/scripts/run` executes:

```sh
docker compose -f phase0/compose.yaml up --build --remove-orphans
```

- [ ] **Step 4: Verify**

Run:

```bash
phase0/scripts/run
curl --fail http://127.0.0.1:8090/
curl --fail http://127.0.0.1:8090/fixtures/hls/master.m3u8
```

Expected: both requests return HTTP 200.

- [ ] **Step 5: Commit**

```bash
git add phase0/Dockerfile phase0/nginx.conf phase0/compose.yaml phase0/scripts/run
git commit -m "chore: containerize compatibility harness"
```

### Task 6: Write the Manual Migaku Procedure

**Files:**
- Create: `docs/phase0/migaku-test-procedure.md`
- Create: `docs/phase0/migaku-compatibility-report.md`

- [ ] **Step 1: Document prerequisites**

Record:

- The tested macOS, Chrome, and Migaku versions.
- Migaku account/extension readiness.
- The harness URL.
- How to reset browser site permissions and harness state.

- [ ] **Step 2: Add one test table per variant and media source**

The report matrix covers six combinations:

```text
track + MP4
track + HLS
DOM + MP4
DOM + HLS
combined + MP4
combined + HLS
```

Each combination records pass/fail and evidence for detection, previous/next
cue, replay, text capture, audio capture, screenshot capture, card creation,
seek, pause, offset, and fullscreen.

- [ ] **Step 3: Add generated-subtitle tests**

Document:

- Whether Migaku offers generation on the harness.
- Whether generated cues are visible in the player.
- Whether Migaku exposes a documented export action.
- Exported file format, if any.
- Whether a documented API is available.

The decision must be one of:

```text
persist_via_documented_api
persist_via_manual_export_upload
usable_in_migaku_but_not_persistable
generation_not_available
```

- [ ] **Step 4: Add a binary gate decision**

The report ends with:

```text
PHASE 0 RESULT: PASS | FAIL
APPROVED PLAYER CONTRACT: <variant and required DOM rules>
PHASE 1 AUTHORIZED: YES | NO
USER APPROVAL DATE: <date or blank>
```

- [ ] **Step 5: Commit**

```bash
git add docs/phase0
git commit -m "docs: add Migaku compatibility procedure"
```

### Task 7: Execute the Manual Test and Stop at the Gate

**Files:**
- Modify: `docs/phase0/migaku-compatibility-report.md`

- [ ] **Step 1: Run all automated verification**

Run:

```bash
docker compose -f phase0/compose.yaml build --pull=false
docker run --rm \
  -v "$PWD/phase0:/work" \
  -w /work \
  node:24.1.0-bookworm-slim \
  sh -c "npm ci && npm test && npm run build"
```

Expected: all tests pass using already pinned images.

- [ ] **Step 2: Perform the procedure with the user**

The user operates Migaku in their installed Chrome environment. Record every
result without interpreting a partial pass as full compatibility.

- [ ] **Step 3: Complete the report**

Include exact versions, evidence references, successful variant, limitations,
and generated-subtitle persistence decision.

- [ ] **Step 4: Request explicit approval**

Do not start Phase 1. Ask the user to set:

```text
PHASE 1 AUTHORIZED: YES
USER APPROVAL DATE: YYYY-MM-DD
```

- [ ] **Step 5: Commit the completed report**

```bash
git add docs/phase0/migaku-compatibility-report.md
git commit -m "docs: record Migaku compatibility result"
```
