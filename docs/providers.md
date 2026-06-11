# Providers

Orphion uses **content providers** to search for and download episodes, and **subtitle providers** to search for and download subtitle files.

## Content Providers

### AllAnime

The default content provider. Searches anime and drama titles via the AllAnime GraphQL API.

- **Content**: Anime and drama
- **Quality options**: 1080p, 720p, 480p (depends on available streams)
- **Stream delivery**: Direct M3U8/HLS via FFmpeg

#### How it works

1. Searches the AllAnime GraphQL API for titles matching your query
2. Retrieves episode listings for the selected title
3. Resolves available streams and quality variants
4. FFmpeg downloads the selected stream directly

#### Switching to AllAnime

In interactive mode, select "allanime" from the provider list.

In config:

```yaml
provider: allanime
```

#### Limitations

- API responses can be slow under load
- Some titles may not have streams available
- Episode numbering may differ from other sources
- Stream availability depends on the upstream source

### Bettermelon

A CDN-based content provider that downloads HLS segments in parallel before running FFmpeg.

- **Content**: Anime (via upstream providers like hianime)
- **Quality options**: 1080p, 720p, 480p (depends on the upstream provider)
- **Stream delivery**: Segments downloaded in parallel, then remuxed by FFmpeg

#### How it works

1. Searches the Bettermelon API for titles
2. Retrieves episode listings with available provider sources
3. Lists quality variants from the master playlist (no upfront download)
4. On selection, downloads HLS segments in parallel with retry logic
5. FFmpeg remuxes the downloaded segments into a single MKV

#### Switching to Bettermelon

In interactive mode, select "bettermelon" from the provider list.

In config:

```yaml
provider: bettermelon
```

#### Limitations

- Depends on upstream providers (default: hianime) which can go down
- Segment downloads may fail if the CDN is having issues
- The "resolving stream" phase can be slow if the upstream is unresponsive
- Fallback to alternative upstream providers is automatic on failure

### Switching Providers

You can switch providers in several ways:

**Interactive mode** — select a provider when prompted during search.

**Configuration file** — set the default provider:

```yaml
# ~/.config/orphion/config.yaml
provider: bettermelon
```

**Per-session** — not currently available as a CLI flag, but you can select the provider in interactive mode.

## Subtitle Providers

Orphion searches all configured subtitle providers simultaneously and presents merged results. You can pick which provider's subtitles to download.

### SubDL

A subtitle database with broad coverage of TV shows and movies.

- **Languages**: Many languages (English, Spanish, French, etc.)
- **Content types**: TV series and movies
- **Quality tags**: WebDL, BluRay, etc.
- **Format**: Downloads ZIP archives containing SRT/ASS files
- **Season support**: Yes — automatic season navigation (skips specials)

#### How it works

1. Searches SubDL for titles matching your query
2. Ranks results by similarity using fuzzy token matching
3. Auto-matches if there's a single clear result, otherwise shows selection
4. Lists available subtitles filtered by language
5. Downloads and extracts subtitle files from ZIP archives

### Kitsunekko

A community-driven anime subtitle site with directories organized by title.

- **Languages**: Primarily English and Japanese
- **Content types**: Anime
- **Quality tags**: Limited — mostly community-uploaded
- **Format**: Direct SRT/ASS file downloads or ZIP archives
- **Season support**: Limited — organized by folder names

#### How it works

1. Fetches the full home page directory listing (cached for the session)
2. Filters entries client-side using fuzzy token matching
3. Ranks results by similarity to avoid showing thousands of unrelated entries
4. Lists subtitle files for the selected entry
5. Downloads and extracts subtitle files

#### Limitations

- No search API — the full directory is fetched and filtered locally
- First search can be slow if the site is under load
- Directory structure varies — some entries are well-organized, others are not

### Jimaku

A Japanese-focused subtitle site hosting anime subtitle files.

- **Languages**: Primarily Japanese (some English)
- **Content types**: Anime
- **Quality tags**: Detected from filename (BluRay, WebRip, etc.)
- **Format**: Direct file downloads
- **Season support**: No — organized by entry only

#### How it works

1. Fetches the home page entry listing (cached for the session)
2. Filters entries by token overlap with the query
3. Lists subtitle files for the selected entry
4. Downloads files directly

#### Limitations

- Primarily hosts Japanese subtitles — for English subtitles, prefer SubDL or Kitsunekko
- Language detection is based on filename heuristics
- Entry list is cached for the session — restart Orphion to see new entries

### Subtitle Language

By default, Orphion searches for English subtitles. Change the language with:

```bash
# Per-command
orphion subtitles "Naruto" --lang spanish

# In config
# ~/.config/orphion/config.yaml
subtitle_lang: spanish
```

### Episode-Aware Subtitle Matching

When downloading episodes, Orphion automatically matches subtitles to episodes:

1. Subtitles with explicit episode numbers are matched directly
2. Subtitles without episode numbers are parsed from the filename (e.g., `S01E03`, `EP3`)
3. When multiple subtitles exist for the same episode, the one with the most downloads is preferred
4. Missing subtitle episodes are reported so you know which ones need manual search

## Provider Architecture

### Content provider interface

```go
type Provider interface {
    Search(ctx context.Context, query, kind string) ([]Anime, error)
    Episodes(ctx context.Context, animeID string) ([]Episode, error)
    Streams(ctx context.Context, episodeID string) ([]Stream, error)
}
```

Providers that need to prepare streams before FFmpeg can consume them also implement:

```go
type StreamPreparer interface {
    PrepareStream(ctx context.Context, stream Stream, progress SegmentProgressFunc) (Stream, error)
}
```

### Subtitle provider interface

```go
type Provider interface {
    Search(ctx context.Context, query string) ([]Result, error)
    Page(ctx context.Context, sdID, slug, seasonSlug string) (*PageResult, error)
    DownloadURL(sub Subtitle) string
}
```

For details on adding new providers, see [CONTRIBUTING.md](../CONTRIBUTING.md).
