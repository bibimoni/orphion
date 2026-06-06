# AGENTS.md

## Project

Orphion is a localhost anime streaming web application designed for custom SRT
subtitles, saved watch progress, and direct Migaku sentence mining.

Read these documents before making changes:

1. `docs/superpowers/specs/2026-06-07-orphion-phase-0-phase-1-design.md`
2. `docs/infrastructure.md`
3. `docs/superpowers/plans/2026-06-07-phase-0-migaku-compatibility.md`
4. `docs/superpowers/plans/2026-06-07-phase-1-orphion-application.md`

## Phase Gate

Phase 0 is a disposable Migaku compatibility test. Do not implement Phase 1
application code until `docs/phase0/migaku-compatibility-report.md` contains:

```text
PHASE 0 RESULT: PASS
PHASE 1 AUTHORIZED: YES
```

Phase 0 must not introduce PostgreSQL, the live anime provider, production
proxy code, or reusable application architecture.

## Development Environment

- Develop, test, debug, install dependencies, and run migrations inside the
  development container.
- Use `scripts/create_docker`, `scripts/run_docker`, `scripts/install`,
  `scripts/run`, `scripts/test`, and `scripts/migrate`.
- Bind the application only to `127.0.0.1` unless a reviewed requirement
  explicitly changes that behavior.
- Development application data and PostgreSQL data must both use configurable
  host bind mounts.
- Never replace the development PostgreSQL bind mount with an anonymous volume.
- Do not delete, recreate, or migrate user data destructively without explicit
  approval and a documented backup path.

Default development data roots:

```text
macOS:
  ~/Library/Application Support/Orphion/dev/app
  ~/Library/Application Support/Orphion/dev/postgres

Linux:
  ${XDG_DATA_HOME:-$HOME/.local/share}/orphion/dev/app
  ${XDG_DATA_HOME:-$HOME/.local/share}/orphion/dev/postgres
```

## Version Policy

- Never use `:latest`.
- Never omit a Docker image tag.
- Pin every Docker `FROM` and Compose `image` to an explicit version.
- Pin Go with the `go` and `toolchain` directives plus `.go-version`.
- Pin Node.js with `.node-version` and matching Docker images.
- Use exact frontend dependency versions and commit `package-lock.json`.
- Commit `go.mod` and `go.sum`.
- Do not update unrelated dependencies while implementing a feature or bugfix.
- Run the repository version-policy check before completing container changes.

## Architecture Rules

- Keep Orphion a modular Go/Gin monolith with a small React/TypeScript client.
- Serve the production frontend from the Go application.
- Keep provider-specific requests, decoding, headers, host policy, and errors
  inside the provider package.
- Use a compile-time provider registry in Phase 1. Do not add a dynamic plugin
  runtime.
- Treat provider anime and episode IDs as opaque strings.
- Do not expose upstream media URLs to the browser.
- Proxy media only through expiring playback sessions and opaque resource IDs.
- Parse HLS manifests structurally. Do not rewrite playlists with regular
  expressions.
- Apply SSRF protections to initial URLs, DNS results, and every redirect.
- Do not implement DRM bypass or proprietary authorization circumvention.

## Migaku Contract

- The Phase 0-approved player DOM arrangement is a compatibility contract.
- Use a normal HTML5 `<video>` element.
- Do not move required media/subtitle state into an iframe or closed shadow
  root.
- Preserve the approved native track and/or selectable DOM subtitle structure.
- Migaku-generated subtitles may be persisted only through a documented API or
  a user-exported SRT/WebVTT file.
- Do not read Migaku private extension storage, intercept undocumented traffic,
  or depend on private extension internals.

## Persistence

- PostgreSQL stores profile, library, progress, and subtitle metadata.
- Uploaded SRT files are stored under generated names in mounted application
  storage; never use an uploaded filename as a filesystem path.
- Preserve original SRT bytes and generate WebVTT/JSON cues from normalized
  parsed data.
- Every user-owned table must include `profile_id`, even while only the seeded
  default profile exists.
- Database migrations are explicit. Production startup verifies schema state
  but does not silently migrate.

## Testing

- Use test-driven development for features and bug fixes.
- Keep deterministic tests independent of the live anime provider.
- Run live-provider smoke tests only through the explicit opt-in flag.
- Test provider implementations with the shared provider contract suite.
- Test proxy behavior with local fixture servers, including redirects, keys,
  ranges, cancellation, timeouts, and prohibited destinations.
- Run Go tests with the race detector for concurrent playback/session code.
- Test the React player against the exact Phase 0-approved DOM contract.
- Repeat the manual Migaku regression before declaring Phase 1 complete.
- Verify that application and PostgreSQL data survive development-container
  recreation.

## Code Quality

- Follow existing package boundaries and keep files focused.
- Prefer standard-library and existing project facilities over new
  dependencies.
- Add a dependency only when it removes substantial implementation risk or
  complexity.
- Return stable public error codes without leaking upstream URLs, secrets, or
  decoder internals.
- Keep UI styling plain, semantic, keyboard accessible, and free of unnecessary
  component frameworks or animation libraries.
- Update relevant documentation when configuration, scripts, architecture, or
  operational behavior changes.

## Scope

Phase 1 does not include:

- Authentication or multiple selectable profiles.
- Windows support.
- Episode downloads or offline playback.
- A second live provider.
- ASS/SSA subtitle uploads.
- A provider marketplace.

Do not add excluded features without an approved design update.

