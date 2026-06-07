# Mission: Implement Orphion Go CLI (Phase 1)

## M1: Project Initialization
### T1: Initialize Go CLI Project | agent:Worker
- [x] S1.1: Initialize Go module and pinned dependencies
- [x] S1.2: Create cmd/orphion/main.go
- [x] S1.3: Implement root command with version subcommand
- [x] S1.4: Write `go test ./internal/cli -run TestVersion -v` (red then green)

### T2: Implement Strict Configuration | agent:Worker
- [ ] S2.1: Create internal/config/config.go
- [ ] S2.2: Create internal/config/config_test.go (TDD)
- [ ] S2.3: Create internal/cli/config.go (config init command)

### T3: Provider Contracts and Registry | agent:Worker
- [ ] S3.1: Define provider types (Anime, Episode, Stream)
- [ ] S3.2: Define Provider interface
- [ ] S3.3: Implement registry with duplicate/unknown key detection

## M2: Core Components
### T4: Episode Expressions | agent:Worker
- [ ] S4.1: Implement expression parser (single, range, comma, "all")
- [ ] S4.2: Implement episode resolution
- [ ] S4.3: Error handling for invalid ranges

### T5: Quality Selection | agent:Worker
- [ ] S5.1: Implement quality parser (1080, 1080p, 1920x1080)
- [ ] S5.2: Implement selection with exact/lower_fallback/lowest_fallback/provider_fallback

### T6: Output Paths | agent:Worker
- [ ] S6.1: Implement title sanitization
- [ ] S6.2: Implement episode filename generation with padding
- [ ] S6.3: Implement path traversal prevention
- [ ] S6.4: Implement partial/final file operations

### T7: FFmpeg Config | agent:Worker
- [ ] S7.1: Create fake-ffmpeg testdata
- [ ] S7.2: Implement FFmpeg runner (executable lookup, arg construction)

## M3: Provider Implementation
### T8: Catalog Provider | agent:Worker
- [ ] S8.1: Research and record sanitized fixtures
- [ ] S8.2: Implement catalog search, episodes, streams
- [ ] S8.3: Live smoke test gated by ORPHION_LIVE_PROVIDER_TEST=1

## M4: Integration
### T9: App Service | agent:Worker
- [ ] S9.1: Implement orchestration service bridging CLI and provider

### T10: CLI Commands | agent:Worker
- [ ] S10.1: Search command
- [ ] S10.2: Download command (non-interactive)
- [ ] S10.3: Version and help commands

### T11: Interactive Mode | agent:Worker
- [ ] S11.1: Implement pterm interactive prompts
- [ ] S11.2: Implement default root command interactive flow

### T12: Binary Assembly | agent:Worker
- [ ] S12.1: Wire up main.go with signal handling and composition
- [ ] S12.2: Exit code mapping

## M5: Final Verification | agent:Reviewer
### T13: Verification | agent:Reviewer
- [ ] S13.1: Format and vet (gofmt, go vet)
- [ ] S13.2: Run all tests with race detector (go test -race ./...)
- [ ] S13.3: Build binary
- [ ] S13.4: Integration tests with fake-FFmpeg