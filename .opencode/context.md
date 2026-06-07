# Project Context

## Environment
- Language: Go 1.26
- Build: `go build ./cmd/orphion`
- Test: `go test -race ./...`
- Lint: `go vet ./...`, golangci-lint
- Package Manager: Go modules

## Project Type
- CLI Application (anime search and download tool)

## Structure
- Source: internal/
- Commands: cmd/orphion/main.go
- Tests: alongside source files (*_test.go)

## Key Packages
- `internal/app` - Service layer (orchestration)
- `internal/cli` - Cobra CLI commands + pterm interactive UI
- `internal/config` - YAML configuration
- `internal/provider` - Anime provider interface + allanime impl
- `internal/download` - Concurrent download scheduler
- `internal/ffmpeg` - FFmpeg wrapper for stream download
- `internal/paths` - Output path generation
- `internal/episode` - Episode expression parser
- `internal/quality` - Stream quality selection
- `internal/subtitle` - Subtitle provider interface + ZIP extraction
- `internal/subtitle/subdl` - SubDL provider (scrapes Next.js SSR pages)

## Conventions
- Naming: Go standard (camelCase unexported, PascalCase exported)
- Error handling: fmt.Errorf with %w wrapping
- Testing: table-driven tests, testdata/ directory
- Imports: stdlib, then external, then internal
- UI: pterm for interactive prompts, cobra for CLI

## SubDL Website Structure (Subtitle Source)
- Search: `GET /search/{query}` - Next.js SSR, `__NEXT_DATA__` JSON
  - `props.pageProps.list` = [{type, sd_id, name, slug, year, subtitles_count}]
- Subtitle page: `GET /subtitle/{sd_id}/{slug}/{season-slug}`
  - `props.pageProps.movieInfo` = {type, sd_id, name, seasons: [{number, name}]}
  - `props.pageProps.groupedSubtitles` = {lang: [{id, language, quality, link, bucketLink, author, season, episode, title, downloads, releases}]}
- Download: `https://dl.subdl.com/subtitle/{link}` (ZIP file containing .srt/.ass)
- Season slugs: "first-season", "second-season", etc.
- Language filter: append `/english`, `/arabic`, etc. to subtitle URL

## Notes
- Provider interface pattern: Provider interface + ProviderImpl struct + NewProvider() + client
- Interactive flow: step-based loop with backOption
