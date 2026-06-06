# Orphion Phase 0 and Phase 1 Design

**Status:** Approved design, pending written-spec review  
**Date:** 2026-06-07

## 1. Purpose

Orphion is a localhost web application for watching streamed anime with custom
subtitles, saved watch progress, and direct Migaku sentence-mining support.

Phase 0 is a disposable compatibility test. It must prove that the current
Migaku Chrome extension can work with Orphion's proposed browser-player
structure before Phase 1 application development begins.

Phase 1 is the first usable application. It targets Chrome on macOS first,
runs through Docker, stores data in PostgreSQL, and uses one live
AllAnime-derived provider adapter.

## 2. Scope

### 2.1 Phase 0

Phase 0 contains only:

- A disposable same-origin browser-player test harness.
- Deterministic test video and HLS fixtures.
- SRT upload and parsing needed for the test.
- Native WebVTT and selectable DOM subtitle experiments.
- A manual Migaku compatibility procedure.
- A written compatibility report.

Phase 0 does not contain:

- PostgreSQL.
- Watch-progress persistence.
- The AllAnime-derived provider.
- Production proxy security.
- The final React application.
- Production Docker packaging.

Phase 1 must not begin until the user reviews and approves the Phase 0 report.

### 2.2 Phase 1

Phase 1 includes:

- One seeded, empty default profile.
- Anime search through one AllAnime-derived live adapter.
- Episode listing and streaming playback.
- A restricted backend HLS proxy.
- SRT upload and persistent subtitle storage.
- Saved and resumed watch progress.
- A minimal Chrome-first React interface.
- Docker-based development and production workflows.
- PostgreSQL persistence.

Phase 1 excludes:

- User accounts and authentication.
- Multiple selectable profiles.
- Windows support.
- Local episode downloads.
- Offline playback.
- Multiple live anime providers.
- DRM bypass.
- A downloadable provider-plugin marketplace.
- ASS/SSA and custom subtitle formats other than SRT.

## 3. Migaku Compatibility Gate

### 3.1 Test matrix

The Phase 0 harness must test these subtitle arrangements separately:

1. A native HTML5 `<track>` containing generated WebVTT.
2. Timed, selectable subtitle text in the normal document DOM.
3. Native `<track>` and selectable DOM subtitles together.

The player must use a normal HTML5 `<video>` element. It must not place the
video in an iframe or hide required subtitle state inside a closed shadow root.

### 3.2 Mandatory acceptance criteria

Using the user's installed Migaku extension in Google Chrome, Migaku must:

1. Detect the video and subtitle cues.
2. Move to the previous and next subtitle lines.
3. Replay the current subtitle line.
4. Capture subtitle text, a video screenshot, and the corresponding audio.
5. Create a usable card through the user's configured Migaku workflow.
6. Continue working after seek, pause, subtitle offset adjustment, and
   fullscreen entry/exit.

The report records:

- macOS version.
- Chrome version.
- Migaku extension version/channel.
- Relevant Migaku settings.
- Test media and subtitle fixture.
- Exact DOM/media arrangement.
- Result and evidence for every criterion.
- Known limitations and required workarounds.

Any failed mandatory criterion blocks Phase 1 pending a design reassessment.
Compatibility with asbplayer or another mining tool is not a substitute unless
the user explicitly changes the requirement.

### 3.3 Migaku-generated subtitles

Phase 0 must also test Migaku's automatic subtitle-generation feature.

Migaku's public documentation confirms subtitle generation in some supported
workflows, especially YouTube, but does not document a general third-party API
or export contract for generated subtitles.

Orphion may persist generated subtitles only when one of these supported
paths exists:

- Migaku exports SRT or WebVTT and the user uploads that file to Orphion.
- Migaku exposes a documented API or documented browser integration that
  provides the generated cues.

Orphion will not read private extension storage, reverse engineer Migaku's
internal protocol, or intercept undocumented extension traffic. If generated
subtitles remain internal to Migaku, users may mine with them, but Orphion
will not store them.

## 4. Architecture

### 4.1 Application shape

Phase 1 uses a modular Go monolith:

- Gin serves the JSON API, proxy endpoints, health endpoints, and built React
  assets.
- React and TypeScript implement the browser UI.
- PostgreSQL stores profile-owned application state.
- A mounted host directory stores uploaded SRT files.
- The application binds to `127.0.0.1` by default.

The production artifact is one Orphion application image plus PostgreSQL.
The frontend is not deployed as a separate production service.

### 4.2 Backend modules

`provider`

- Defines normalized provider operations.
- Registers providers explicitly at application startup.
- Contains one AllAnime-derived live adapter and one deterministic fake
  adapter for contract tests.

`playback`

- Resolves provider stream candidates.
- Creates short-lived playback sessions.
- Maps upstream resources to opaque resource IDs.
- Proxies and rewrites HLS manifests, segments, and keys.

`subtitle`

- Validates SRT uploads.
- Stores original SRT files under generated names.
- Parses normalized subtitle cues.
- Produces WebVTT for native tracks and JSON cues for selectable DOM display.

`library`

- Stores profile-owned references to provider anime and episodes.
- Treats provider IDs as opaque strings.
- Treats display metadata as a cache rather than canonical provider truth.

`progress`

- Loads and upserts playback position, duration, completion state, and
  last-watched time.
- Applies a completion threshold near the end of an episode.

`profile`

- Creates exactly one empty default profile through an idempotent seed.
- Keeps `profile_id` on every user-owned record for future multi-profile work.

### 4.3 Provider contract

The conceptual provider interface is:

```go
type Provider interface {
    Search(ctx context.Context, query string) ([]Anime, error)
    Episodes(ctx context.Context, animeRef string) ([]Episode, error)
    Streams(ctx context.Context, episodeRef string) ([]Stream, error)
}
```

Provider references remain opaque. AllAnime-specific GraphQL requests,
source decoding, referrer headers, host policy, and error translation remain
inside the AllAnime package.

Phase 1 intentionally uses a compile-time registry rather than a dynamic
plugin runtime. A second live provider is not required. The fake adapter runs
the same behavioral contract tests and proves that the application depends on
the provider interface rather than AllAnime internals.

### 4.4 Playback proxy

The browser never submits an arbitrary upstream URL.

After stream resolution, the backend creates an expiring playback session and
assigns opaque IDs to allowed upstream resources:

```text
/api/playback/{sessionID}/resources/{resourceID}
```

The proxy:

- Parses HLS playlists structurally.
- Resolves relative URIs against the current upstream manifest.
- Rewrites nested manifests, media segments, initialization segments, and
  encryption-key URIs.
- Supplies provider-required request headers.
- Validates initial URLs and redirects.
- Rejects unsupported schemes, loopback/private/link-local destinations,
  unapproved hosts, oversized manifests, excessive redirects, and expired
  sessions.
- Uses connection, response-header, and total request timeouts.

Playback sessions are in memory and disposable. Restarting Orphion invalidates
active sessions and the user requests a new one.

The proxy does not decrypt DRM or bypass proprietary authorization systems.

## 5. Data Model

### `profiles`

- Internal UUID.
- Stable profile key such as `default`.
- Display name.
- Creation and update timestamps.

An idempotent seed creates one empty default profile.

### `anime_entries`

- Internal UUID.
- `profile_id`.
- Provider key.
- Opaque provider anime ID.
- Cached title and optional image metadata.
- Creation and update timestamps.

A profile/provider/provider-anime-ID tuple is unique.

### `episode_entries`

- Internal UUID.
- `anime_entry_id`.
- Opaque provider episode ID.
- Normalized display label and sortable episode number when available.
- Cached title metadata.

### `watch_progress`

- `profile_id`.
- `episode_entry_id`.
- Position and duration in milliseconds.
- Completion flag.
- Last-watched timestamp.

One row exists per profile and episode.

### `subtitle_assets`

- Internal UUID.
- `profile_id`.
- `episode_entry_id`.
- Original filename.
- Generated storage filename.
- SHA-256 digest.
- Byte size.
- Optional BCP 47 language tag.
- Active flag.
- Creation timestamp.

The database stores metadata only. Original SRT content resides in mounted
application storage.

## 6. User Experience

The visual style is intentionally plain and functional, similar to a
2010-2015 utility website:

- Semantic HTML.
- Plain CSS.
- No gradients or animation-heavy presentation.
- No component-design framework.
- Keyboard-accessible controls and visible focus states.

Primary screens:

1. Home: search and recently watched episodes.
2. Search results: title, source, and direct selection.
3. Anime: episode list and saved progress.
4. Player: video, episode navigation, quality selection, SRT upload/selection,
   subtitle offset, and typed error state.

A fresh profile has no subtitles. Uploading an SRT stores it persistently and
activates it for the selected episode. The selectable subtitle panel is shown
only when the Phase 0 result says it is needed for Migaku compatibility or
when retained as a useful accessibility/mining aid.

The client sends throttled progress updates every 10 seconds and immediate
updates on pause, completion, episode change, and page exit where browser
lifecycle behavior permits.

## 7. Configuration and Storage

`config/config.example.yaml` documents all options. The ignored
`config/config.yaml` is the primary runtime configuration and is mounted into
the container.

Configuration covers:

- Bind address and port.
- PostgreSQL connection.
- Application storage directory.
- SRT upload limits.
- Enabled/default provider.
- Provider and proxy timeouts.
- Playback-session TTL.
- Progress-save interval and completion threshold.
- Logging level.

Environment variables select the configuration path and provide secrets where
necessary. They are not the primary configuration interface.

Default host data locations:

- macOS: `~/Library/Application Support/Orphion`
- Linux: `${XDG_DATA_HOME:-~/.local/share}/orphion`
- Container: `/var/lib/orphion`
- Subtitles: `/var/lib/orphion/subtitles`

In development, PostgreSQL uses a configurable host bind mount so database
state survives container replacement and remains visible in host-managed
development storage. Production uses a separate named volume by default, with
a configured host bind mount remaining possible.

## 8. Development and Packaging

Development uses Docker Compose with:

- An application development container.
- PostgreSQL.
- Source-code mounts.
- Go and Vite watch processes inside the container.
- Configurable host bind mounts for uploaded application data and PostgreSQL
  data.

Required scripts:

- `scripts/create_docker`
- `scripts/run_docker`
- `scripts/install`
- `scripts/run`
- `scripts/test`
- `scripts/migrate`

`create_docker` resolves the platform-appropriate host data path, creates the
required application and PostgreSQL directories, and writes ignored Docker
volume variables. `run_docker` starts or enters the development environment.
`install`, `run`, `test`, and `migrate` execute inside the container.

The production image uses a multi-stage build:

1. Build React assets.
2. Compile a Go binary.
3. Copy the binary, migrations, and static assets into a minimal runtime
   image.

Production Compose publishes the app only on `127.0.0.1`.

All Docker base images and Compose service images use explicit fixed version
tags. The project must not use `:latest` or an omitted image tag. Go
modules and frontend packages are locked through committed dependency files.
Version upgrades are deliberate changes reviewed and tested separately from
feature work.

Database migrations run explicitly. Application startup checks schema
compatibility and fails with an actionable error rather than silently changing
production tables.

## 9. Error Handling

Public API errors use stable typed codes such as:

- `provider_unavailable`
- `anime_not_found`
- `episode_unavailable`
- `stream_unsupported`
- `upstream_timeout`
- `playback_session_expired`
- `subtitle_invalid`
- `subtitle_too_large`
- `storage_unavailable`

Provider errors do not expose upstream URLs, decoding details, or secrets.
Provider failures do not delete or corrupt saved progress and subtitle
metadata.

Uploads are parsed before activation, size-limited, stored under generated
names, and prevented from controlling filesystem paths.

## 10. Verification Strategy

### Automated

- Unit tests for SRT parsing, cue normalization, configuration, progress
  completion, provider mapping, URL policy, and manifest rewriting.
- Provider contract tests against the fake adapter and fixture-backed live
  adapter behavior.
- PostgreSQL integration tests for migrations and repositories.
- HTTP integration tests for uploads, playback sessions, and proxy routes
  using deterministic local upstream servers.
- Frontend tests for player state, resume behavior, progress throttling,
  subtitle upload, and typed errors.
- Browser tests in Chrome-compatible automation using deterministic fixtures.

### Manual

- Phase 0 Migaku compatibility test.
- Phase 1 Migaku regression test using the approved Phase 0 contract.
- One live-provider smoke test.

Live upstream tests remain separate from the regular automated suite so
provider instability cannot make deterministic development tests unreliable.

## 11. Reference Projects

- [Migaku documentation](https://migaku.com/faq/getting-started): official
  supported-site and generated-subtitle claims.
- [asbplayer](https://github.com/asbplayer/asbplayer): maintained reference
  for browser subtitle mining and separation of shared player logic.
- [Animebook](https://github.com/animebook/animebook.github.io): simple HTML5
  player with navigable/selectable subtitle cues; now superseded by asbplayer.
- [ani-cli](https://github.com/pystardust/ani-cli): current AllAnime-derived
  GraphQL, decoding, provider, and referrer behavior.
- [Consumet](https://github.com/consumet/consumet.ts): normalized provider
  interfaces and the maintenance cost of scraper collections.
- [Suwayomi Server](https://github.com/Suwayomi/Suwayomi-Server): local server,
  bundled web UI, source isolation, configuration, and Docker patterns.
- [Manatan](https://github.com/KolbyML/Manatan): localhost web UI and
  immersion-focused media workflow.

These projects are references, not dependencies. Orphion will not copy
license-incompatible source code.
