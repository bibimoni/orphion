# Orphion CLI Usage

## Prerequisites

- macOS for the initial supported release.
- FFmpeg installed and available on `PATH`.

Example installation:

```bash
brew install ffmpeg
```

Confirm:

```bash
ffmpeg -version
```

## Interactive Mode

Run without arguments:

```bash
orphion
```

The CLI asks for:

1. Search text.
2. A content type: anime or drama.
3. A result from the returned list.
4. Episodes or ranges.
5. Output directory confirmation.
6. Preferred quality and concurrency confirmation.

It then downloads selected episodes and prints a summary.

## Search

```bash
orphion search --type anime "Frieren"
```

Example result:

```text
ID                 TITLE
catalog:abc123      Frieren: Beyond Journey's End
catalog:def456      Frieren: Beyond Journey's End (Dub)
```

Provider IDs are opaque. Copy them exactly when using non-interactive
commands.

Drama works the same way:

```bash
orphion search --type drama "My Demon"
```

## Download by Search Text

```bash
orphion download \
  --type anime \
  --title "Frieren" \
  --episodes "1-4"
```

This succeeds without a prompt only when the search has one unambiguous
result. Otherwise, Orphion prints matching IDs and exits with code 3.

## Download by ID

```bash
orphion download \
  --type anime \
  --title-id "catalog:abc123" \
  --episodes "1,3,7" \
  --output "$HOME/Anime"
```

## Episode Expressions

Supported:

```text
1
1-4
1,3,7
1-3,7,10-12
all
```

Whitespace is ignored. Duplicate selections are removed.

Invalid:

```text
4-1
1,,3
episode-1
```

## Quality

Set a preferred height:

```bash
orphion download \
  --type anime \
  --title-id "catalog:abc123" \
  --episodes "1-4" \
  --quality 1080p
```

If `1080p` is unavailable, Orphion chooses the nearest lower quality. When no
lower quality exists, it chooses the lowest available stream.

## Concurrency

The default is one episode at a time:

```bash
orphion download \
  --type anime \
  --title-id "catalog:abc123" \
  --episodes "1-8" \
  --concurrency 1
```

The allowed range is 1 through 4:

```bash
orphion download \
  --type anime \
  --title-id "catalog:abc123" \
  --episodes "1-8" \
  --concurrency 4
```

Higher concurrency uses more bandwidth and may increase provider failures.

## Output

Default layout:

```text
~/Anime/
└── Frieren Beyond Journeys End/
    ├── Episode 01.mkv
    ├── Episode 02.mkv
    └── Episode 03.mkv
```

Downloads first use:

```text
Episode 01.part.mkv
```

The partial file is renamed only after FFmpeg succeeds. Failed or interrupted
partial files are removed.

Existing final files are skipped.

## Configuration

Default path:

```text
~/.config/orphion/config.yaml
```

Create it:

```bash
orphion config init
```

Default content:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: catalog
ffmpeg_path: ffmpeg
```

The command refuses to overwrite an existing file.

Flags override configuration:

```bash
orphion download \
  --type anime \
  --title-id "catalog:abc123" \
  --episodes all \
  --output /Volumes/Media/Anime \
  --quality 720p \
  --concurrency 2
```

## Verbose Diagnostics

```bash
orphion --verbose download \
  --type anime \
  --title-id "catalog:abc123" \
  --episodes 1
```

Verbose output includes:

- Provider operation stages.
- Available and selected qualities.
- Sanitized upstream hostname.
- FFmpeg diagnostic output.
- Partial-file cleanup details.

It must not print full signed URLs or sensitive headers.

## Cancellation

Press `Ctrl+C` to:

- Stop scheduling new episodes.
- Terminate active FFmpeg processes.
- Remove partial files.
- Exit with status 130.

Completed MKV files remain intact.

## Exit Codes

| Code | Meaning |
|---:|---|
| 0 | All requested episodes completed or were skipped |
| 1 | One or more episode downloads failed |
| 2 | Invalid flags or configuration |
| 3 | Provider, search, or selection failed before downloading |
| 130 | Interrupted by the user |

## Commands

```text
orphion
orphion search <query>
orphion download [flags]
orphion config init
orphion version
orphion help
```

## Not Supported in Phase 1

- Playback.
- Subtitles.
- Download resume.
- Arbitrary URL downloading.
- DRM-protected media.
- Multiple live providers.
- Docker-based operation.
