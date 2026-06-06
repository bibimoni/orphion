# AGENTS.md

## Project

Orphion is a Go CLI that searches an AllAnime-derived source and downloads
selected episodes as MKV files through system FFmpeg.

Read before changing code:

1. `docs/architecture.md`
2. `docs/usage.md`
3. `docs/implementation-plan.md`

## Scope

Phase 1 is a command-line downloader only.

Do not add:

- A web server or frontend.
- Docker.
- A database.
- Playback.
- Migaku integration.
- Subtitle downloading.
- Watch-progress tracking.
- Accounts or profiles.
- Download resume.
- A dynamic provider-plugin runtime.
- An ani-cli wrapper.

ani-cli may be researched as a behavioral reference. Orphion must not execute,
parse output from, vendor, or depend on ani-cli.

## Runtime

- One Go binary.
- System FFmpeg is the only external runtime dependency.
- Use `exec.CommandContext` with explicit arguments; never invoke FFmpeg
  through a shell.
- macOS is the initial supported platform.
- Keep process and filesystem code portable where practical.

## Configuration

- Configuration path is `~/.config/orphion/config.yaml`.
- YAML decoding must reject unknown fields.
- Flags override YAML.
- Missing configuration uses defaults.
- Concurrency defaults to 1 and must remain between 1 and 4.
- Preferred quality defaults to `1080p`.
- Never overwrite an existing configuration from `config init`.

## Architecture

- Keep interactive and non-interactive commands thin.
- Both modes must use the same application service.
- Keep provider-specific behavior inside `internal/provider/allanime`.
- Treat provider anime and episode IDs as opaque.
- Keep episode parsing, quality selection, path creation, scheduling, and
  FFmpeg execution in separate focused packages.
- Avoid global mutable state.
- Pass contexts through provider, scheduler, and process boundaries.

## Downloads

- Output layout is `<output>/<Anime Title>/Episode NN.mkv`.
- Download to `.part.mkv`.
- Rename to `.mkv` only after FFmpeg succeeds.
- Remove partial files after failure or cancellation.
- Do not overwrite existing final files by default.
- Stop scheduling new work after cancellation.
- One episode failure must not cancel unrelated jobs.
- Return a non-zero aggregate exit status when any requested download fails.

## Provider

- Implement the AllAnime-derived source directly in Go.
- Use an injected `http.Client`.
- Keep GraphQL, decoding, headers, host rules, and error translation private to
  the adapter.
- Regular tests use sanitized fixtures and never contact the live provider.
- Live tests require `ORPHION_LIVE_PROVIDER_TEST=1`.
- Never log full signed URLs, cookies, or sensitive request headers.
- Do not implement DRM, authentication, or paywall bypass.

## Testing

- Use test-driven development for features and bug fixes.
- Run `go test -race ./...` before completion.
- Test cancellation and process termination with a fake FFmpeg executable.
- Test scheduler concurrency with controlled runners.
- Test path containment and traversal resistance.
- Test all CLI exit codes.
- Keep live-provider instability out of deterministic tests.

## Versioning

- Pin the Go language/toolchain versions in `go.mod` and `.go-version`.
- Pin direct Go dependencies to reviewed versions.
- Do not update unrelated dependencies during feature work.
- FFmpeg is user-installed; validate its presence and report its version in
  verbose diagnostics.

## Code Quality

- Prefer the standard library unless a dependency removes meaningful
  complexity.
- Keep errors typed internally and concise for users.
- Add context with `%w` wrapping.
- Do not call `os.Exit` outside `cmd/orphion/main.go`.
- Do not build command strings with provider-controlled input.
- Keep files and packages focused on one responsibility.
- Update architecture and usage documentation when behavior changes.
