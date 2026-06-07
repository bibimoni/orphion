# Orphion CLI Architecture

## Purpose

Orphion is a Go command-line tool for finding anime and drama episodes from a
single unofficial catalog provider and downloading them as MKV files into a
user-selected folder.

Orphion does not run a server, browser interface, database, Docker environment,
or media player. It does not execute or wrap ani-cli. ani-cli is used only as a
behavioral reference for the unofficial anime-focused parts of the catalog,
while `gonwatch` is the broader reference for anime, drama, and other content
groupings.

## Scope

Phase 1 includes:

- Interactive terminal operation.
- Non-interactive commands and flags using the same application services.
- Anime and drama search.
- Episode listing and selection.
- Episode expressions such as `1-4,7`.
- Preferred-quality selection with downward fallback.
- Configurable download concurrency from 1 through 4, defaulting to 1.
- System FFmpeg invocation to download/remux streams into MKV.
- Deterministic output folders and filenames.
- Partial-file cleanup after failure or cancellation.
- YAML configuration at `~/.config/orphion/config.yaml`.
- One independently implemented catalog provider that supports anime and drama.
- macOS support with portable process and filesystem boundaries.

Phase 1 excludes:

- Video playback.
- Migaku or browser-extension integration.
- Subtitle discovery or downloading.
- Watch-progress tracking.
- Accounts or profiles.
- A database.
- Docker.
- A web server or frontend.
- Wrapping or shelling out to ani-cli.
- A dynamic provider-plugin system.
- Download resume.

## Application Shape

Orphion builds as one Go binary:

```text
CLI input
   |
   v
command layer
   |
   +--> configuration
   +--> interactive prompts
   +--> non-interactive validation
   |
   v
application services
   |
   +--> provider interface
   |      |
   |      +--> catalog adapter
   |
   +--> episode selection
   +--> quality selection
   +--> path generation
   +--> download scheduler
          |
          +--> FFmpeg process runner
```

Interactive and flag-based operation must call the same services. Prompt code
must not contain provider or download behavior.

## Package Boundaries

### `cmd/orphion`

Composition root. It loads configuration, constructs the provider and process
runner, handles operating-system signals, executes the root command, and maps
application errors to exit codes.

### `internal/cli`

Defines commands, flags, validation, interactive prompts, progress output, and
human-readable error reporting.

### `internal/config`

Loads strict YAML configuration, applies defaults, expands `~`, validates
values, and combines configuration with command-line overrides.

### `internal/provider`

Defines normalized provider contracts and domain types.

Conceptual interface:

```go
type Provider interface {
    Search(ctx context.Context, query, kind string) ([]Title, error)
    Episodes(ctx context.Context, titleID string) ([]Episode, error)
    Streams(ctx context.Context, episodeID string) ([]Stream, error)
}
```

Provider identifiers are opaque strings. Consumers must not parse or build
provider-specific identifiers.

`kind` selects the catalog group, such as `anime` or `drama`.

### `internal/provider/catalog`

Owns all unofficial-source details:

- Requests and response mapping.
- Query variables and response structures.
- Source decoding.
- Required upstream headers.
- Stream-quality normalization.
- Provider-specific error translation.
- Allowed upstream hosts.

It may reference observed ani-cli behavior and gonwatch behavior, but must not
copy code with incompatible licensing or invoke either project.

### `internal/episode`

Parses and resolves episode expressions such as:

```text
1
1-4
1,3,7
1-3,7,10-12
all
```

It returns episodes in provider order without duplicates and reports missing
requested episode numbers.

### `internal/quality`

Selects stream candidates. Given a preferred quality of `1080p`:

1. Select exact `1080p` when available.
2. Otherwise select the nearest lower numeric quality.
3. When no lower quality exists, select the lowest available numeric quality.
4. If streams lack parseable quality labels, select the provider-preferred
   candidate and report that fallback under verbose output.

### `internal/download`

Creates jobs, enforces the concurrency limit, tracks results, and stops
scheduling new work after context cancellation.

Independent episode failures do not cancel unrelated queued episodes unless
the root context is cancelled. The final command exits non-zero when any job
fails.

### `internal/ffmpeg`

Finds and validates the configured FFmpeg executable, builds argument lists,
starts child processes, forwards provider-required HTTP headers, relays
progress, handles cancellation, and returns structured process errors.

FFmpeg is the only external runtime dependency.

### `internal/paths`

Sanitizes provider titles for local filesystems, creates anime directories,
formats episode filenames, prevents path traversal, and performs final atomic
renames.

## Download Flow

1. Load built-in defaults.
2. Read `~/.config/orphion/config.yaml` when present.
3. Apply command flags.
4. Validate output directory, concurrency, quality, provider, and FFmpeg path.
5. Search for an anime or resolve an opaque anime ID.
6. List episodes and resolve the requested episode expression.
7. Resolve streams for each selected episode.
8. Select the preferred stream.
9. Create:

   ```text
   <output>/<Sanitized Title>/Episode 01.part.mkv
   ```

10. Start FFmpeg with the stream URL and provider-required headers.
11. After successful FFmpeg exit, sync/close the output and rename:

   ```text
   Episode 01.part.mkv -> Episode 01.mkv
   ```

12. On failure or cancellation, remove the partial file.
13. Print a per-episode and aggregate summary.

Existing final files are not overwritten by default. The command reports them
as skipped. A future reviewed feature may add explicit overwrite behavior.

## FFmpeg Contract

The process runner passes arguments as an argument slice, never through a
shell. Provider data cannot inject command syntax.

Representative command:

```text
ffmpeg
-nostdin
-hide_banner
-loglevel warning
-headers "Referer: ...\r\nUser-Agent: ...\r\n"
-i <stream-url>
-map 0
-c copy
<output>.part.mkv
```

Arguments may be refined for source compatibility. Orphion never logs full
signed URLs unless explicit diagnostic output redacts sensitive query values.

On cancellation, Orphion signals FFmpeg, waits for a bounded grace period, and
forcibly terminates it only if needed.

## Configuration

Path:

```text
~/.config/orphion/config.yaml
```

Shape:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: catalog
default_type: anime
ffmpeg_path: ffmpeg
```

Rules:

- Missing configuration uses built-in defaults.
- Unknown YAML keys are errors.
- Flags override YAML.
- Concurrency must be between 1 and 4.
- `output_dir` is expanded and converted to an absolute path.
- `ffmpeg_path` may be a command on `PATH` or an explicit file path.
- `orphion config init` creates parent directories and refuses to overwrite an
  existing configuration.

## Output Layout

Default:

```text
<output>/
└── <Title>/
    ├── Episode 01.mkv
    ├── Episode 02.mkv
    └── Episode 12.5.mkv
```

Folder titles remove path separators, control characters, reserved filesystem
characters, trailing dots/spaces, and traversal components. Empty sanitized
titles fall back to a stable provider identifier.

Episode labels preserve meaningful decimals where supplied. Padding width is
at least two digits and expands for series with 100 or more episodes.

## Errors and Exit Codes

Error categories:

- Configuration invalid.
- FFmpeg missing or unusable.
- Provider unavailable.
- Anime not found or ambiguous.
- Episode expression invalid.
- Requested episode unavailable.
- No stream candidate.
- Output path unavailable.
- Download failed.
- Interrupted.

Default output is concise. `--verbose` adds request stage, selected quality,
FFmpeg diagnostics, and sanitized upstream host information.

Exit code contract:

```text
0  all requested work completed or was intentionally skipped
1  one or more download jobs failed
2  usage or configuration error
3  provider/search/selection error before downloads started
130 interrupted by the user
```

## Security and Legal Boundaries

- Orphion downloads only streams returned by the configured provider.
- It does not bypass DRM, authentication, paywalls, or proprietary protection.
- It does not accept an arbitrary URL-download flag in Phase 1.
- It does not execute provider-controlled strings through a shell.
- Logs avoid signed URLs, cookies, and sensitive headers.
- Users are responsible for complying with applicable law and source terms.

## Verification

Automated tests cover:

- YAML loading, strict fields, defaults, and flag precedence.
- Episode-expression parsing and resolution.
- Quality fallback.
- Filename and path sanitization.
- Catalog mapping and decoding using sanitized recorded fixtures.
- FFmpeg argument construction.
- Partial cleanup and atomic rename.
- Cancellation and child-process termination.
- Scheduler concurrency and failure aggregation.
- CLI exit-code behavior.

Regular tests never contact the live provider. An explicit opt-in smoke test
verifies current catalog compatibility.
