# Phase 1 Orphion Application Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Docker-run localhost anime player with PostgreSQL persistence, replaceable provider adapters, restricted HLS proxying, SRT uploads, watch progress, and the player contract approved by Phase 0.

**Architecture:** A modular Go/Gin monolith serves a small React/TypeScript SPA and JSON API. PostgreSQL stores profile, library, progress, and subtitle metadata; host bind mounts persist development PostgreSQL and uploaded SRT files. Provider-specific behavior is isolated behind a compile-time interface, while playback URLs are represented by expiring server-side sessions and opaque resource IDs.

**Tech Stack:** Go 1.24.4, Gin 1.10.1, pgx 5.7.5, goose 3.24.3, PostgreSQL 17.5, React 19.1.0, TypeScript 5.8.3, Vite 6.3.5, hls.js 1.6.5, Vitest 3.2.1, Playwright 1.52.0, Docker Compose 2.35.1; all images and dependencies pinned with no `:latest`

**Execution Gate:** Do not execute this plan until `docs/phase0/migaku-compatibility-report.md` says `PHASE 0 RESULT: PASS` and `PHASE 1 AUTHORIZED: YES`.

---

## File Map

```text
cmd/orphion/main.go                       # composition root
internal/
├── config/                              # typed YAML loading and validation
├── httpapi/                             # Gin routes, middleware, error envelope
├── profile/                             # default-profile service/repository
├── library/                             # anime and episode persistence
├── progress/                            # progress rules and persistence
├── subtitle/                            # upload, storage, SRT parsing, WebVTT
├── provider/                            # normalized contracts and registry
│   ├── fake/                            # deterministic contract-test adapter
│   └── allanime/                        # live source implementation
└── playback/                            # sessions, URL policy, HLS proxy
db/
├── migrations/                          # versioned SQL migrations
└── queries/                             # focused SQL files if used
web/
├── package.json
├── package-lock.json
├── src/
│   ├── api/                             # typed API client
│   ├── app/                             # routes and application shell
│   ├── features/search/
│   ├── features/anime/
│   ├── features/player/
│   ├── features/subtitles/
│   └── features/progress/
└── tests/
config/config.example.yaml
deploy/
├── Dockerfile.dev
├── Dockerfile
├── compose.dev.yaml
└── compose.yaml
scripts/
├── create_docker
├── run_docker
├── install
├── run
├── test
└── migrate
```

### Task 1: Establish Pinned Toolchains and Repository Skeleton

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `.go-version`
- Create: `.node-version`
- Create: `web/package.json`
- Create: `web/package-lock.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `.gitignore`
- Create: `Makefile`

- [ ] **Step 1: Initialize the Go module with the pinned toolchain**

```bash
go mod init github.com/distiled/orphion
go mod edit -go=1.24.0
go mod edit -toolchain=go1.24.4
```

Set `.go-version` to `1.24.4` and `.node-version` to `24.1.0`.

- [ ] **Step 2: Add exact frontend dependencies**

Use exact versions:

```json
{
  "name": "@orphion/web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0",
    "build": "tsc --noEmit && vite build",
    "test": "vitest run",
    "test:e2e": "playwright test"
  },
  "dependencies": {
    "hls.js": "1.6.5",
    "react": "19.1.0",
    "react-dom": "19.1.0",
    "react-router-dom": "7.6.2"
  },
  "devDependencies": {
    "@playwright/test": "1.52.0",
    "@testing-library/react": "16.3.0",
    "@types/react": "19.1.6",
    "@types/react-dom": "19.1.5",
    "@vitejs/plugin-react": "4.5.2",
    "jsdom": "26.1.0",
    "typescript": "5.8.3",
    "vite": "6.3.5",
    "vitest": "3.2.1"
  }
}
```

- [ ] **Step 3: Generate dependency lockfiles in pinned containers**

Run `go mod tidy` with `golang:1.24.4-bookworm` and `npm install
--package-lock-only` with `node:24.1.0-bookworm-slim`.

- [ ] **Step 4: Add a version-policy check**

The Makefile target `check-versions` fails when Dockerfiles or Compose files
contain `:latest`, an untagged `image:`, or an unversioned `FROM`.

- [ ] **Step 5: Verify empty builds**

Run:

```bash
go test ./...
npm --prefix web ci
npm --prefix web run build
```

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "chore: establish pinned project toolchains"
```

### Task 2: Build the Development Container Workflow

**Files:**
- Create: `deploy/Dockerfile.dev`
- Create: `deploy/compose.dev.yaml`
- Create: `scripts/create_docker`
- Create: `scripts/run_docker`
- Create: `scripts/install`
- Create: `scripts/run`
- Create: `scripts/test`
- Create: `scripts/migrate`
- Create: `.env.docker.example`

- [ ] **Step 1: Write shell tests for platform path resolution**

Test that macOS resolves:

```text
~/Library/Application Support/Orphion/dev/app
~/Library/Application Support/Orphion/dev/postgres
```

and Linux resolves:

```text
${XDG_DATA_HOME:-$HOME/.local/share}/orphion/dev/app
${XDG_DATA_HOME:-$HOME/.local/share}/orphion/dev/postgres
```

- [ ] **Step 2: Write a pinned development Dockerfile**

Start from `golang:1.24.4-bookworm`, install Node.js `24.1.0` through a
checksum-verified archive, and install a fixed migration tool version. Do not
download unversioned installer scripts.

- [ ] **Step 3: Define bind-mounted development services**

Use `postgres:17.5-bookworm`. Compose mounts:

```yaml
services:
  app:
    volumes:
      - ../:/workspace
      - ${ORPHION_DEV_APP_DATA}:/var/lib/orphion
  db:
    image: postgres:17.5-bookworm
    volumes:
      - ${ORPHION_DEV_POSTGRES_DATA}:/var/lib/postgresql/data
```

Publish the application only on `127.0.0.1`.

- [ ] **Step 4: Implement `create_docker`**

It must:

- Detect Darwin/Linux.
- Resolve absolute app and PostgreSQL data paths.
- Create both directories.
- Create `.env.docker` with shell-safe values.
- Build with `docker compose --env-file .env.docker`.

- [ ] **Step 5: Implement the remaining scripts**

`install`, `run`, `test`, and `migrate` must refuse to run outside the app
container by checking a marker such as `ORPHION_CONTAINER=1`.

- [ ] **Step 6: Verify persistence**

Create a marker table, recreate the database container, and confirm the marker
remains in the host bind-mounted PostgreSQL directory.

- [ ] **Step 7: Run the version-policy check and commit**

```bash
make check-versions
git add deploy scripts .env.docker.example
git commit -m "chore: add persistent container development workflow"
```

### Task 3: Implement Typed Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `config/config.example.yaml`

- [ ] **Step 1: Write failing tests**

Cover default values, full YAML loading, missing database fields, invalid
duration, non-loopback default binding warning, relative storage rejection,
unknown provider, and secret-file loading.

- [ ] **Step 2: Run the focused test**

```bash
go test ./internal/config -run TestLoad -v
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement typed YAML configuration**

Use a strict YAML decoder that rejects unknown fields. Define:

```go
type Config struct {
    Server    ServerConfig
    Database  DatabaseConfig
    Storage   StorageConfig
    Providers ProviderConfig
    Playback  PlaybackConfig
    Progress  ProgressConfig
    Logging   LoggingConfig
}
```

Only `ORPHION_CONFIG` selects the YAML path. Secrets may come from configured
files.

- [ ] **Step 4: Pass tests and commit**

```bash
go test ./internal/config -v
git add internal/config config/config.example.yaml
git commit -m "feat: load validated YAML configuration"
```

### Task 4: Add Database Migrations and Default Profile

**Files:**
- Create: `db/migrations/00001_initial.sql`
- Create: `internal/profile/model.go`
- Create: `internal/profile/repository.go`
- Create: `internal/profile/repository_test.go`
- Create: `internal/database/database.go`
- Create: `internal/database/migration_test.go`

- [ ] **Step 1: Write migration integration tests**

Assert all five tables, foreign keys, unique constraints, indexes, and one
idempotently seeded `default` profile.

- [ ] **Step 2: Run tests against the development PostgreSQL**

Expected: FAIL because no migration exists.

- [ ] **Step 3: Write the migration**

Create:

- `profiles`
- `anime_entries`
- `episode_entries`
- `watch_progress`
- `subtitle_assets`

Use UUID primary keys, `timestamptz`, integer milliseconds, and explicit
cascading behavior. Seed profile key `default` with `ON CONFLICT DO NOTHING`.

- [ ] **Step 4: Implement pgx connection and profile repository**

Use `pgxpool`. Expose `GetDefault(ctx)` and return a typed error when the seed
is missing.

- [ ] **Step 5: Verify migration up/down/up and commit**

```bash
scripts/migrate down
scripts/migrate up
go test ./internal/database ./internal/profile -v
git add db internal/database internal/profile
git commit -m "feat: add persistent default profile schema"
```

### Task 5: Define Provider Contracts and Fake Adapter

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/errors.go`
- Create: `internal/provider/registry.go`
- Create: `internal/provider/contract_test.go`
- Create: `internal/provider/fake/provider.go`
- Create: `internal/provider/fake/fixtures.go`

- [ ] **Step 1: Write the reusable provider contract suite**

Test:

- Stable provider key.
- Search normalization.
- Opaque anime/episode refs.
- Ordered episode results.
- Stream candidates with quality and provider request policy.
- Context cancellation.
- Typed unavailable/not-found errors.

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/provider/... -v
```

- [ ] **Step 3: Implement contracts and fake adapter**

Define normalized `Anime`, `Episode`, `Stream`, and `RequestPolicy` types.
Registry construction rejects duplicate keys and missing configured defaults.

- [ ] **Step 4: Pass tests and commit**

```bash
go test ./internal/provider/... -v
git add internal/provider
git commit -m "feat: define replaceable provider contract"
```

### Task 6: Implement the AllAnime-Derived Adapter

**Files:**
- Create: `internal/provider/allanime/client.go`
- Create: `internal/provider/allanime/graphql.go`
- Create: `internal/provider/allanime/decode.go`
- Create: `internal/provider/allanime/provider.go`
- Create: `internal/provider/allanime/provider_test.go`
- Create: `internal/provider/allanime/testdata/`
- Create: `docs/providers/allanime.md`

- [ ] **Step 1: Capture sanitized deterministic fixtures**

Store representative search, episode, and source-resolution responses without
user cookies or sensitive query values. Record retrieval date and the
corresponding ani-cli revision used as behavioral reference.

- [ ] **Step 2: Write failing fixture-backed tests**

Test GraphQL request shape, response mapping, source decoding, quality
normalization, required referrer/user-agent policy, malformed response, and
upstream timeout.

- [ ] **Step 3: Implement the adapter**

Keep endpoint, query hashes, decode keys, and allowed-host policy in the
AllAnime package. Do not expose upstream URLs through API DTOs or logs.

- [ ] **Step 4: Run contract and adapter tests**

```bash
go test ./internal/provider/... -v
```

- [ ] **Step 5: Add opt-in live smoke test**

Guard it with `ORPHION_LIVE_PROVIDER_TEST=1`. Regular test commands must not
contact the live source.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/allanime docs/providers/allanime.md
git commit -m "feat: add AllAnime-derived provider adapter"
```

### Task 7: Implement Playback Sessions and SSRF Policy

**Files:**
- Create: `internal/playback/session.go`
- Create: `internal/playback/session_test.go`
- Create: `internal/playback/urlpolicy.go`
- Create: `internal/playback/urlpolicy_test.go`

- [ ] **Step 1: Write failing session tests**

Cover opaque IDs, TTL expiry, concurrent lookup, cleanup, and provider policy
ownership.

- [ ] **Step 2: Write failing URL-policy tests**

Reject:

- Non-HTTP(S) schemes.
- Loopback and private IPv4.
- IPv6 loopback, unique-local, and link-local.
- Redirects to prohibited addresses.
- Hosts outside provider allowlists.
- DNS rebinding between validation and connection.

- [ ] **Step 3: Implement session store and controlled dialer**

Resolve and dial the validated address directly so a second DNS lookup cannot
change the destination after policy validation.

- [ ] **Step 4: Pass race-enabled tests**

```bash
go test -race ./internal/playback -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/playback
git commit -m "feat: secure playback sessions and upstream policy"
```

### Task 8: Implement Structural HLS Rewriting and Streaming Proxy

**Files:**
- Create: `internal/playback/hls.go`
- Create: `internal/playback/hls_test.go`
- Create: `internal/playback/proxy.go`
- Create: `internal/playback/proxy_test.go`

- [ ] **Step 1: Write failing manifest tests**

Cover master/media playlists, relative/absolute URIs, query strings,
`EXT-X-KEY`, `EXT-X-MAP`, nested playlists, malformed playlists, and size
limits.

- [ ] **Step 2: Write failing proxy integration tests**

Use `httptest.Server` upstream fixtures to assert required headers, redirect
validation, streamed segment bodies, content type, range requests, timeout,
and opaque rewritten URLs.

- [ ] **Step 3: Implement parser-backed rewriting**

Use a maintained HLS parser pinned in `go.mod`. Do not rewrite manifests with
regular expressions.

- [ ] **Step 4: Implement bounded streaming**

Buffer manifests only up to the configured maximum. Stream media bodies with
context cancellation and selected safe response headers.

- [ ] **Step 5: Pass tests and commit**

```bash
go test -race ./internal/playback -v
git add internal/playback
git commit -m "feat: proxy session-scoped HLS resources"
```

### Task 9: Implement Subtitle Storage and Conversion

**Files:**
- Create: `internal/subtitle/parser.go`
- Create: `internal/subtitle/parser_test.go`
- Create: `internal/subtitle/storage.go`
- Create: `internal/subtitle/storage_test.go`
- Create: `internal/subtitle/service.go`
- Create: `internal/subtitle/repository.go`

- [ ] **Step 1: Write failing parser tests**

Port the approved Phase 0 cue behavior into Go tests, including UTF-8,
multiline cues, malformed timing, empty files, and WebVTT output.

- [ ] **Step 2: Write failing storage tests**

Assert generated filenames, directory traversal prevention, configured size
limit, SHA-256 calculation, atomic write/rename, duplicate-content behavior,
and cleanup after repository failure.

- [ ] **Step 3: Implement parser and filesystem storage**

Preserve original SRT bytes. Generate WebVTT on request from normalized cues.
Never use the uploaded filename as a storage path.

- [ ] **Step 4: Implement metadata transaction**

Associate the subtitle with default profile and episode. Activating one
subtitle deactivates the previous active subtitle for that profile/episode.

- [ ] **Step 5: Pass tests and commit**

```bash
go test ./internal/subtitle -v
git add internal/subtitle
git commit -m "feat: persist and serve uploaded SRT subtitles"
```

### Task 10: Implement Library and Progress Services

**Files:**
- Create: `internal/library/model.go`
- Create: `internal/library/repository.go`
- Create: `internal/library/repository_test.go`
- Create: `internal/progress/service.go`
- Create: `internal/progress/service_test.go`
- Create: `internal/progress/repository.go`

- [ ] **Step 1: Write failing library upsert tests**

Test opaque provider IDs, metadata refresh, episode ordering, and profile
isolation.

- [ ] **Step 2: Write failing completion tests**

Define completion as either configured percentage reached or configured
remaining duration reached, while rejecting negative/non-finite values and
clamping position to duration.

- [ ] **Step 3: Implement repositories and service rules**

Use transactions when selecting an episode creates/updates anime and episode
entries together.

- [ ] **Step 4: Pass PostgreSQL integration tests and commit**

```bash
go test ./internal/library ./internal/progress -v
git add internal/library internal/progress
git commit -m "feat: save library entries and watch progress"
```

### Task 11: Build the Gin API and Error Contract

**Files:**
- Create: `internal/httpapi/router.go`
- Create: `internal/httpapi/errors.go`
- Create: `internal/httpapi/middleware.go`
- Create: `internal/httpapi/handlers_provider.go`
- Create: `internal/httpapi/handlers_playback.go`
- Create: `internal/httpapi/handlers_subtitle.go`
- Create: `internal/httpapi/handlers_progress.go`
- Create: `internal/httpapi/router_test.go`

- [ ] **Step 1: Write failing endpoint tests**

Cover:

```text
GET  /health/live
GET  /health/ready
GET  /api/providers
GET  /api/anime/search?q=
GET  /api/anime/:provider/:animeRef/episodes
POST /api/playback
GET  /api/playback/:session/resources/:resource
POST /api/episodes/:episodeID/subtitles
GET  /api/subtitles/:subtitleID.vtt
GET  /api/subtitles/:subtitleID/cues
GET  /api/episodes/:episodeID/progress
PUT  /api/episodes/:episodeID/progress
```

- [ ] **Step 2: Define a stable error envelope**

```json
{
  "error": {
    "code": "provider_unavailable",
    "message": "The anime source is temporarily unavailable.",
    "requestId": "..."
  }
}
```

- [ ] **Step 3: Implement request IDs, recovery, body limits, and typed mapping**

Never return upstream URLs or internal decoder errors to clients.

- [ ] **Step 4: Pass API tests and commit**

```bash
go test ./internal/httpapi -v
git add internal/httpapi
git commit -m "feat: expose Orphion HTTP API"
```

### Task 12: Build the Minimal React Workflow

**Files:**
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/app/App.tsx`
- Create: `web/src/app/styles.css`
- Create: `web/src/api/client.ts`
- Create: `web/src/features/search/SearchPage.tsx`
- Create: `web/src/features/anime/AnimePage.tsx`
- Create: `web/src/features/player/PlayerPage.tsx`
- Create: `web/src/features/player/VideoPlayer.tsx`
- Create: `web/src/features/subtitles/SubtitleControls.tsx`
- Create: `web/src/features/progress/useProgress.ts`
- Create: corresponding `*.test.tsx` files

- [ ] **Step 1: Write failing page and player tests**

Cover search, episode selection, resume, throttled progress, pause flush,
subtitle upload, offset, typed errors, expired-session retry, and the exact
Phase 0-approved player DOM contract.

- [ ] **Step 2: Implement the API client**

Use typed responses, `AbortController`, and one centralized conversion from
API error envelopes to UI states.

- [ ] **Step 3: Implement plain semantic pages**

Use native elements and plain CSS. Do not introduce a UI framework, gradients,
or animation dependencies.

- [ ] **Step 4: Implement the approved player contract**

Use a normal `<video>` and only the subtitle DOM arrangement approved by the
Phase 0 report. Use native HLS where supported and hls.js otherwise.

- [ ] **Step 5: Pass frontend tests and commit**

```bash
npm --prefix web test
npm --prefix web run build
git add web
git commit -m "feat: add minimal anime playback interface"
```

### Task 13: Compose the Application and Serve the SPA

**Files:**
- Create: `cmd/orphion/main.go`
- Create: `internal/httpapi/static.go`
- Create: `internal/httpapi/static_test.go`
- Create: `web/embed.go`

- [ ] **Step 1: Write failing composition and static-route tests**

Assert dependency wiring, graceful shutdown, SPA history fallback, API 404
behavior, and startup failure when migrations are missing.

- [ ] **Step 2: Implement the composition root**

Construct configuration, logging, pgx pool, repositories, provider registry,
playback store/client, Gin router, and signal-aware server.

- [ ] **Step 3: Embed built assets**

The Go binary serves the frontend build while preserving `/api` and `/health`
routes.

- [ ] **Step 4: Pass all deterministic tests and commit**

```bash
go test -race ./...
npm --prefix web test
git add cmd internal/httpapi web/embed.go
git commit -m "feat: compose Orphion application"
```

### Task 14: Build the Production Image and Compose Deployment

**Files:**
- Create: `deploy/Dockerfile`
- Create: `deploy/compose.yaml`
- Create: `deploy/postgres-password.example`
- Create: `.dockerignore`

- [ ] **Step 1: Write a pinned multi-stage image**

Use exact tags:

```dockerfile
FROM node:24.1.0-bookworm-slim AS frontend
FROM golang:1.24.4-bookworm AS backend
FROM debian:12.11-slim
```

Copy the CA certificate bundle from the pinned Go builder stage, create a
dedicated non-root `orphion` user, and import Go's `time/tzdata` package so the
runtime stage does not install mutable Debian packages during its build.

- [ ] **Step 2: Define production Compose**

Use `postgres:17.5-bookworm`, a named production database volume, configurable
app-data bind mount, secrets, health checks, and localhost-only publishing.

- [ ] **Step 3: Run security and version checks**

Confirm:

- No `latest`.
- Runtime user is non-root.
- PostgreSQL is not host-published.
- Only app data and explicit temporary paths are writable.
- Image contains no Go/Node toolchains.

- [ ] **Step 4: Smoke test**

```bash
docker compose -f deploy/compose.yaml up --build -d
curl --fail http://127.0.0.1:8080/health/ready
curl --fail http://127.0.0.1:8080/
docker compose -f deploy/compose.yaml down
```

- [ ] **Step 5: Commit**

```bash
git add deploy .dockerignore
git commit -m "chore: package pinned production deployment"
```

### Task 15: Add Browser and Migaku Regression Verification

**Files:**
- Create: `web/playwright.config.ts`
- Create: `web/e2e/watch.spec.ts`
- Create: `docs/testing.md`
- Create: `docs/migaku-regression.md`

- [ ] **Step 1: Add deterministic browser fixtures**

Use the fake provider and local HLS fixture. Test search through progress
resume and SRT activation in Chromium.

- [ ] **Step 2: Run Playwright in a pinned image**

Use `mcr.microsoft.com/playwright:v1.52.0-noble` and do not install an
unversioned browser at runtime.

- [ ] **Step 3: Repeat the approved Migaku manual contract**

Verify detection, cue controls, text/audio/screenshot capture, card creation,
seek, pause, offset, and fullscreen against the Phase 1 player.

- [ ] **Step 4: Record generated-subtitle behavior**

Follow the Phase 0 persistence decision. Do not add private Migaku integration.

- [ ] **Step 5: Commit**

```bash
git add web/e2e web/playwright.config.ts docs/testing.md docs/migaku-regression.md
git commit -m "test: verify browser and Migaku playback workflow"
```

### Task 16: Complete Operations Documentation

**Files:**
- Create: `README.md`
- Create: `docs/development.md`
- Create: `docs/configuration.md`
- Create: `docs/operations.md`
- Modify: `docs/infrastructure.md`

- [ ] **Step 1: Document macOS-first setup**

Include exact script order, resolved default paths, Chrome requirement, and
how to inspect both app and PostgreSQL bind-mounted development data.

- [ ] **Step 2: Document configuration**

Explain every YAML key, secret file, provider setting, timeout, storage limit,
and safe localhost default.

- [ ] **Step 3: Document backup and restore**

Provide exact `pg_dump`/`pg_restore` commands using `postgres:17.5-bookworm`
and archive/restore commands for the subtitle directory.

- [ ] **Step 4: Document provider breakage**

Explain how to disable the adapter, run fixtures, run the opt-in smoke test,
and replace the adapter without changing frontend contracts.

- [ ] **Step 5: Run documentation command checks and commit**

```bash
make check-versions
scripts/test
git add README.md docs
git commit -m "docs: add development and operations guides"
```

### Task 17: Final Verification

**Files:**
- Modify only files required to fix verification failures.

- [ ] **Step 1: Run formatting and static analysis**

```bash
gofmt -w cmd internal
go vet ./...
npm --prefix web run build
make check-versions
```

- [ ] **Step 2: Run deterministic tests**

```bash
go test -race ./...
npm --prefix web test
docker run --rm \
  --network host \
  -v "$PWD:/work" \
  -w /work/web \
  mcr.microsoft.com/playwright:v1.52.0-noble \
  npm run test:e2e
```

- [ ] **Step 3: Verify development persistence**

Recreate both containers and confirm uploaded SRT and PostgreSQL progress rows
remain present in their host bind mounts.

- [ ] **Step 4: Run production smoke test**

Build from an empty Docker cache, migrate, start, verify health/UI, play the
fixture, and stop cleanly.

- [ ] **Step 5: Run opt-in live provider smoke test**

Record the date and result separately from deterministic tests.

- [ ] **Step 6: Perform manual Migaku regression**

Complete `docs/migaku-regression.md` using the approved Phase 0 criteria.

- [ ] **Step 7: Commit verification fixes**

```bash
git add .
git commit -m "test: complete Phase 1 verification"
```
