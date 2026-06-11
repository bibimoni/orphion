# Contributing to Orphion

Thank you for your interest in contributing to Orphion! This guide covers everything you need to get started.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/bibimoni/orphion.git
cd orphion

# Install dependencies
go mod download

# Verify everything works
go test -race ./...
go vet ./...
golangci-lint run

# Build and install locally
bash scripts/dev-setup.sh
```

## Development Environment

### Prerequisites

- **Go 1.26+** (check `.go-version` for the exact version)
- **FFmpeg** — required for integration tests
- **golangci-lint** — for linting (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- **pre-commit** (optional) — for git hook automation

### Set up pre-commit hooks

```bash
# Install pre-commit (macOS)
brew install pre-commit

# Install the git hooks
pre-commit install
```

The pre-commit hook runs `golangci-lint` on changed files before each commit.

### Development scripts

```bash
# Build + install to $GOPATH/bin
bash scripts/dev-setup.sh

# Build + install + run tests
bash scripts/dev-setup.sh --test

# Clean previous install first
bash scripts/dev-setup.sh --clean
```

## Running Tests

```bash
# Run all tests with race detection
go test -race ./...

# Run tests for a specific package
go test -race ./internal/app/...

# Run a specific test
go test -race -run TestBestMatch ./internal/subtitle/

# Run tests with coverage
go test -race -cover ./...

# Run tests with verbose output
go test -race -v ./internal/ffmpeg/
```

### Live integration tests

Some tests hit real network endpoints. These are gated behind the `-tags=live` build tag and are **not** run by default:

```bash
# Run live tests (requires network access)
go test -race -tags=live ./internal/provider/allanime/
go test -race -tags=live ./internal/provider/bettermelon/
```

## Linting

```bash
# Run the linter
golangci-lint run

# Run with verbose output
golangci-lint run -v

# Run on a specific package
golangci-lint run ./internal/app/
```

The project uses a `.golangci.yml` configuration file. See it for the enabled linters and settings.

## Code Style

Follow standard Go conventions:

- **Formatting**: `gofmt` (or `goimports`) — no configuration needed
- **Naming**: `camelCase` for unexported, `PascalCase` for exported
- **Error handling**: Wrap errors with `fmt.Errorf("context: %w", err)`
- **Comments**: Document all exported types and functions
- **Package organization**: One concern per package, interface at the top

### Error handling pattern

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("search %s/%s: %w", kind, query, err)
}
```

### Constants pattern

All hardcoded values should live in `internal/common/constants.go`:

```go
const (
    // DefaultQuality is the preferred stream quality.
    DefaultQuality = "1080p"
)
```

## Project Structure

```
cmd/
  orphion/          # Main entry point
internal/
  app/              # Core application service
  cli/              # CLI commands and interactive UI
  common/           # Shared constants
  config/           # YAML configuration
  download/         # Download scheduler
  episode/          # Episode expression parsing
  ffmpeg/           # FFmpeg process wrapper
  paths/            # Output path layout
  provider/         # Content provider interface + implementations
    allanime/       # AllAnime provider
    bettermelon/    # Bettermelon provider
  quality/          # Stream quality selection
  subtitle/         # Subtitle provider interface + implementations
    jimaku/         # Jimaku subtitle provider
    kitsunekko/     # Kitsunekko subtitle provider
    subdl/          # SubDL subtitle provider
```

## Adding a New Content Provider

Content providers implement the `provider.Provider` interface:

```go
type Provider interface {
    Search(ctx context.Context, query, kind string) ([]Anime, error)
    Episodes(ctx context.Context, animeID string) ([]Episode, error)
    Streams(ctx context.Context, episodeID string) ([]Stream, error)
}
```

### Steps

1. **Create a new package** under `internal/provider/yourprovider/`
2. **Implement the interface** with a `Provider` struct
3. **Add a `Config` struct** with `DefaultConfig()` function
4. **Add a `Client` struct** for HTTP interaction
5. **Write tests** using `httptest.Server` for mocking
6. **Register the provider** in `cmd/orphion/main.go`
7. **Add provider constants** to `internal/common/constants.go`
8. **Add a live test** gated behind `//go:build live`

### Template

```
internal/provider/yourprovider/
  config.go      # Config struct + DefaultConfig()
  client.go      # Client with Search, Episodes, Streams
  provider.go    # Provider wrapping Client
  client_test.go # Unit tests with httptest
  live_test.go   # Integration tests (//go:build live)
```

See `internal/provider/allanime/` or `internal/provider/bettermelon/` for complete examples.

## Adding a New Subtitle Provider

Subtitle providers implement the `subtitle.Provider` interface:

```go
type Provider interface {
    Search(ctx context.Context, query string) ([]Result, error)
    Page(ctx context.Context, sdID, slug, seasonSlug string) (*PageResult, error)
    DownloadURL(sub Subtitle) string
}
```

### Steps

1. **Create a new package** under `internal/subtitle/yourprovider/`
2. **Implement the interface** with a `Provider` struct
3. **Add a `Config` struct** with `DefaultConfig()` function
4. **Add a `Client` struct** for HTTP/HTML scraping
5. **Write tests** using `httptest.Server` for mocking
6. **Register the provider** in `cmd/orphion/main.go`
7. **Add provider constants** to `internal/common/constants.go`

See `internal/subtitle/subdl/` or `internal/subtitle/jimaku/` for complete examples.

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new provider support
fix: resolve download timeout on slow connections
docs: update troubleshooting guide
test: add coverage for episode expression parser
refactor: extract quality selection logic
ci: update GitHub Actions workflow
chore: update dependencies
```

### Scope (optional)

```
feat(provider): add dramacool provider
fix(subtitle): match episodes by metadata
docs(readme): add features section
```

## Pull Request Process

1. **Fork** the repository
2. **Create a branch** from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```
3. **Make your changes** with tests
4. **Run the full check suite**:
   ```bash
   go test -race ./...
   go vet ./...
   golangci-lint run
   ```
5. **Commit** with a descriptive message following Conventional Commits
6. **Push** to your fork
7. **Open a Pull Request** against the `main` branch

### PR checklist

- [ ] All tests pass (`go test -race ./...`)
- [ ] No lint issues (`golangci-lint run`)
- [ ] No vet issues (`go vet ./...`)
- [ ] New code has tests
- [ ] Exported types and functions are documented
- [ ] Hardcoded values are in `internal/common/constants.go`
- [ ] Commit messages follow Conventional Commits

## Questions?

- [Open an issue](https://github.com/bibimoni/orphion/issues) for bug reports or feature requests
- Start a [Discussion](https://github.com/bibimoni/orphion/discussions) for questions
