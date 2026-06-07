# Orphion Go CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that searches a single unofficial catalog provider for anime and drama content, then downloads selected episodes as MKV files through system FFmpeg.

**Architecture:** One Go binary provides interactive and non-interactive commands over shared application services. A provider interface isolates the unofficial catalog source, while a bounded scheduler runs FFmpeg processes and atomically promotes successful partial downloads.

**Tech Stack:** Go 1.24.4; Cobra 1.9.1; pterm 0.12.81 for terminal prompts/output; yaml.v3 3.0.1; standard `net/http`, `os/exec`, filesystem, context, and signal packages; system FFmpeg

---

## File Map

```text
cmd/orphion/main.go
internal/
├── app/
│   ├── service.go
│   └── service_test.go
├── cli/
│   ├── root.go
│   ├── search.go
│   ├── download.go
│   ├── config.go
│   ├── interactive.go
│   └── cli_test.go
├── config/
│   ├── config.go
│   └── config_test.go
├── provider/
│   ├── provider.go
│   ├── registry.go
│   ├── contract_test.go
│   └── catalog/
│       ├── client.go
│       ├── decode.go
│       ├── provider.go
│       ├── provider_test.go
│       └── testdata/
├── episode/
│   ├── expression.go
│   └── expression_test.go
├── quality/
│   ├── select.go
│   └── select_test.go
├── paths/
│   ├── paths.go
│   └── paths_test.go
├── ffmpeg/
│   ├── runner.go
│   └── runner_test.go
└── download/
    ├── scheduler.go
    └── scheduler_test.go
testdata/
└── fake-ffmpeg
go.mod
go.sum
.go-version
README.md
```

### Task 1: Initialize the Go CLI Project

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `.go-version`
- Create: `cmd/orphion/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/cli_test.go`
- Create: `README.md`

- [ ] **Step 1: Write a failing root-command test**

Test that `orphion version` writes a version string and exits successfully
without loading a provider or FFmpeg.

- [ ] **Step 2: Run the test and verify failure**

```bash
go test ./internal/cli -run TestVersion -v
```

Expected: FAIL because the CLI package does not exist.

- [ ] **Step 3: Initialize the module and pinned dependencies**

```bash
go mod init github.com/distiled/orphion
go mod edit -go=1.24.0
go mod edit -toolchain=go1.24.4
go get github.com/spf13/cobra@v1.9.1
go get github.com/pterm/pterm@v0.12.81
go get gopkg.in/yaml.v3@v3.0.1
```

Set `.go-version` to `1.24.4`.

- [ ] **Step 4: Implement the minimal root and version command**

The root command defaults to interactive mode only when no subcommand is
given. Keep `version` independent of runtime dependencies.

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/cli -v
go test ./...
git add go.mod go.sum .go-version cmd internal/cli README.md
git commit -m "chore: initialize Orphion Go CLI"
```

### Task 2: Implement Strict Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `internal/cli/config.go`

- [ ] **Step 1: Write failing configuration tests**

Cover:

- Missing file uses defaults.
- `~` expands using the current home directory.
- Unknown keys fail.
- Concurrency 0 and 5 fail.
- Concurrency 1 and 4 pass.
- Flags override YAML values.
- `config init` creates parent directories.
- `config init` refuses to overwrite an existing file.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/config -v
```

- [ ] **Step 3: Implement**

Expose:

```go
type Config struct {
    OutputDir        string `yaml:"output_dir"`
    PreferredQuality string `yaml:"preferred_quality"`
    Concurrency      int    `yaml:"concurrency"`
    Provider         string `yaml:"provider"`
    FFmpegPath       string `yaml:"ffmpeg_path"`
}
```

Use `yaml.Decoder.KnownFields(true)`. Default configuration path is
`~/.config/orphion/config.yaml`.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/config -v
git add internal/config internal/cli/config.go
git commit -m "feat: add strict CLI configuration"
```

### Task 3: Define Provider Contracts and Registry

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/registry.go`
- Create: `internal/provider/contract_test.go`
- Create: `internal/provider/fake_test.go`

- [ ] **Step 1: Write a reusable provider contract suite**

Test search normalization, opaque IDs, stable episode order, stream metadata,
context cancellation, and typed unavailable/not-found errors.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/provider -v
```

- [ ] **Step 3: Implement normalized types**

```go
type Anime struct {
    ID    string
    Title string
}

type Episode struct {
    ID      string
    Number  string
    SortKey float64
}

type Stream struct {
    URL     string
    Quality string
    Headers http.Header
}
```

Registry construction rejects duplicate keys and unknown configured providers.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/provider -v
git add internal/provider
git commit -m "feat: define anime provider contracts"
```

### Task 4: Parse Episode Expressions

**Files:**
- Create: `internal/episode/expression.go`
- Create: `internal/episode/expression_test.go`

- [ ] **Step 1: Write failing table tests**

Cover single values, ranges, comma lists, mixed expressions, whitespace,
`all`, duplicates, decimals, reversed ranges, empty tokens, unknown episodes,
and provider ordering.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/episode -v
```

- [ ] **Step 3: Implement parsing and resolution**

Parsing produces a request independent of provider episode objects.
Resolution matches normalized episode labels and preserves provider order.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/episode -v
git add internal/episode
git commit -m "feat: parse episode selections"
```

### Task 5: Implement Quality Selection

**Files:**
- Create: `internal/quality/select.go`
- Create: `internal/quality/select_test.go`

- [ ] **Step 1: Write failing table tests**

Cover exact quality, nearest lower quality, no lower quality, unordered
candidates, duplicate labels, labels such as `1080`, `1080p`, and `1920x1080`,
and unparseable labels.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/quality -v
```

- [ ] **Step 3: Implement deterministic selection**

Return the selected stream and a reason enum:

```go
exact
lower_fallback
lowest_fallback
provider_fallback
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/quality -v
git add internal/quality
git commit -m "feat: select preferred stream quality"
```

### Task 6: Implement Safe Output Paths

**Files:**
- Create: `internal/paths/paths.go`
- Create: `internal/paths/paths_test.go`

- [ ] **Step 1: Write failing tests**

Cover path separators, `..`, control characters, reserved characters, Unicode,
empty titles, trailing dots/spaces, decimal episodes, padding, existing files,
and output containment.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/paths -v
```

- [ ] **Step 3: Implement**

Expose final and partial paths. Verify with `filepath.Rel` that both remain
inside the configured output directory.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/paths -v
git add internal/paths
git commit -m "feat: generate safe anime output paths"
```

### Task 7: Implement FFmpeg Process Runner

**Files:**
- Create: `internal/ffmpeg/runner.go`
- Create: `internal/ffmpeg/runner_test.go`
- Create: `testdata/fake-ffmpeg`

- [ ] **Step 1: Write failing tests using a fake executable**

Test:

- Executable lookup.
- Argument construction without a shell.
- CRLF-separated HTTP headers.
- MKV stream-copy arguments.
- Stdout/stderr forwarding.
- Non-zero exit mapping.
- Context cancellation.
- Graceful then forced termination.
- Partial-file removal.
- Atomic final rename.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/ffmpeg -v
```

- [ ] **Step 3: Implement the runner**

Use `exec.CommandContext` only with explicit arguments. Create the partial file
in the final directory. Rename only after a zero FFmpeg exit.

- [ ] **Step 4: Verify with race detector and commit**

```bash
go test -race ./internal/ffmpeg -v
git add internal/ffmpeg testdata/fake-ffmpeg
git commit -m "feat: download streams through FFmpeg"
```

### Task 8: Implement the Download Scheduler

**Files:**
- Create: `internal/download/scheduler.go`
- Create: `internal/download/scheduler_test.go`

- [ ] **Step 1: Write failing concurrency tests**

Use a controlled fake runner to assert:

- Default concurrency 1.
- Limits 2 through 4.
- No more than the configured number run simultaneously.
- One failure does not cancel unrelated jobs.
- Root cancellation stops scheduling.
- Results preserve episode order.
- Aggregate status identifies skipped, completed, failed, and cancelled jobs.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/download -v
```

- [ ] **Step 3: Implement a bounded worker scheduler**

Do not create unbounded goroutines. Workers read jobs from a channel and send
one result per job.

- [ ] **Step 4: Verify with race detector and commit**

```bash
go test -race ./internal/download -v
git add internal/download
git commit -m "feat: schedule bounded episode downloads"
```

### Task 9: Implement the Catalog Provider

**Files:**
- Create: `internal/provider/catalog/client.go`
- Create: `internal/provider/catalog/decode.go`
- Create: `internal/provider/catalog/provider.go`
- Create: `internal/provider/catalog/provider_test.go`
- Create: `internal/provider/catalog/testdata/search.json`
- Create: `internal/provider/catalog/testdata/episodes.json`
- Create: `internal/provider/catalog/testdata/streams.json`
- Create: `docs/provider-catalog.md`

- [ ] **Step 1: Record sanitized fixtures**

Research current ani-cli behavior and independently capture representative
catalog search, episode, and stream responses. Record retrieval date and
reference revision. Remove cookies, signed URLs, and personal data. Use
gonwatch behavior as the secondary reference for drama and other non-anime
content groupings.

- [ ] **Step 2: Write failing fixture-backed tests**

Test GraphQL requests, response mapping, opaque IDs, source decoding, stream
quality labels, required headers, malformed responses, cancellation, and
timeouts.

- [ ] **Step 3: Implement the adapter**

Use an injected `http.Client`. Keep all endpoints, decoding, headers, and host
rules inside this package. Do not import or execute ani-cli or gonwatch.

- [ ] **Step 4: Run contract tests**

```bash
go test ./internal/provider/... -v
```

- [ ] **Step 5: Add an opt-in live smoke test**

Require:

```text
ORPHION_LIVE_PROVIDER_TEST=1
```

Regular tests must not contact the live catalog.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/catalog docs/provider-catalog.md
git commit -m "feat: add catalog provider"
```

### Task 10: Build the Shared Application Service

**Files:**
- Create: `internal/app/service.go`
- Create: `internal/app/service_test.go`

- [ ] **Step 1: Write failing orchestration tests**

Test:

- Search forwarding.
- Ambiguous title detection.
- Catalog ID resolution.
- Episode selection.
- Stream resolution.
- Quality selection.
- Existing-file skip.
- Job construction.
- Aggregate failure mapping.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/app -v
```

- [ ] **Step 3: Implement the service**

The service contains no terminal prompts and writes no output directly.
Interactive and non-interactive commands call the same methods.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/app -v
git add internal/app
git commit -m "feat: orchestrate content downloads"
```

### Task 11: Implement Search and Non-Interactive Download Commands

**Files:**
- Create: `internal/cli/search.go`
- Create: `internal/cli/download.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/cli_test.go`

- [ ] **Step 1: Write failing command tests**

Cover:

- Search output.
- Unambiguous title download.
- Ambiguous title prints IDs and exits 3.
- ID-based download.
- Missing episode flag exits 2.
- Configuration override flags.
- Partial failure exits 1.
- Cancellation exits 130.
- Sensitive URLs absent from default output.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/cli -v
```

- [ ] **Step 3: Implement commands**

Commands accept injected application service, input, output, and error streams.
Do not call `os.Exit` below `cmd/orphion/main.go`.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/cli -v
git add internal/cli
git commit -m "feat: add search and download commands"
```

### Task 12: Implement Interactive Mode

**Files:**
- Create: `internal/cli/interactive.go`
- Create: `internal/cli/interactive_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write failing interaction tests**

Abstract prompts behind an interface. Test search query, result selection,
episode expression, output confirmation, quality/concurrency confirmation,
cancelled prompts, and use of the shared application service.

- [ ] **Step 2: Verify red**

```bash
go test ./internal/cli -run Interactive -v
```

- [ ] **Step 3: Implement pterm prompts**

Running `orphion` without arguments enters interactive mode. Never prompt when
a non-interactive command has enough input.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/cli -v
git add internal/cli
git commit -m "feat: add interactive download workflow"
```

### Task 13: Compose the Binary and Signal Handling

**Files:**
- Modify: `cmd/orphion/main.go`
- Create: `cmd/orphion/main_test.go`

- [ ] **Step 1: Write failing composition tests**

Test dependency construction, missing FFmpeg diagnostics, signal-derived
context cancellation, and exit-code mapping.

- [ ] **Step 2: Verify red**

```bash
go test ./cmd/orphion -v
```

- [ ] **Step 3: Implement composition**

Use `signal.NotifyContext` for interrupt/termination signals. Build the
configuration, registry, catalog provider, FFmpeg runner, scheduler,
application service, and root command.

- [ ] **Step 4: Verify and commit**

```bash
go test ./cmd/orphion -v
git add cmd/orphion
git commit -m "feat: compose Orphion CLI binary"
```

### Task 14: Complete Documentation and Release Build

**Files:**
- Modify: `README.md`
- Modify: `docs/usage.md`
- Create: `LICENSE`
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Document installation and FFmpeg prerequisite**

Include source build, macOS installation, configuration, interactive use,
non-interactive examples, exit codes, legal notice, and troubleshooting.

- [ ] **Step 2: Configure fixed release targets**

Build macOS arm64/amd64 first. Keep Linux/Windows targets disabled until tested.

- [ ] **Step 3: Build and smoke test**

```bash
go build -trimpath -ldflags "-s -w" -o dist/orphion ./cmd/orphion
./dist/orphion version
./dist/orphion help
```

- [ ] **Step 4: Commit**

```bash
git add README.md docs/usage.md LICENSE .goreleaser.yaml
git commit -m "docs: add CLI installation and release guidance"
```

### Task 15: Final Verification

**Files:**
- Modify only files needed to fix verification failures.

- [ ] **Step 1: Format and vet**

```bash
gofmt -w cmd internal
go vet ./...
```

- [ ] **Step 2: Run deterministic tests**

```bash
go test -race ./...
```

- [ ] **Step 3: Build**

```bash
go build -trimpath -o dist/orphion ./cmd/orphion
```

- [ ] **Step 4: Run fake-FFmpeg end-to-end test**

Exercise search fixtures through final MKV rename without contacting the live
provider.

- [ ] **Step 5: Run opt-in live provider smoke test**

Record the date, source behavior, and result separately from deterministic
tests.

- [ ] **Step 6: Verify interruption**

Start a fake long-running download, send `SIGINT`, verify exit 130, no new jobs,
terminated children, removed partial files, and preserved completed files.

- [ ] **Step 7: Commit fixes**

```bash
git add .
git commit -m "test: complete Orphion CLI verification"
```
