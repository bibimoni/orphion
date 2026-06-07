# Mission: Implement Orphion Go CLI (Phase 1)

## M1: Project Initialization | status: completed
### T1: Initialize Go CLI Project | agent:Worker | status: completed
- [x] S1.1: Initialize Go module and pinned dependencies
- [x] S1.2: Create cmd/orphion/main.go
- [x] S1.3: Implement root command with version subcommand
- [x] S1.4: Write `go test ./internal/cli -run TestVersion -v` (red then green)

### T2: Implement Strict Configuration | agent:Worker | status: completed
- [x] S2.1: Create internal/config/config.go
- [x] S2.2: Create internal/config/config_test.go (TDD)
- [x] S2.3: Create internal/cli/config.go (config init command)

### T3: Provider Contracts and Registry | agent:Worker | status: completed
- [x] S3.1: Define provider types (Anime, Episode, Stream)
- [x] S3.2: Define Provider interface
- [x] S3.3: Implement registry with duplicate/unknown key detection

## M2: Core Components | status: completed
### T4: Episode Expressions | agent:Worker | status: completed
- [x] S4.1: Implement expression parser (single, range, comma, "all")
- [x] S4.2: Implement episode resolution
- [x] S4.3: Error handling for invalid ranges

### T5: Quality Selection | agent:Worker | status: completed
- [x] S5.1: Implement quality parser (1080, 1080p, 1920x1080)
- [x] S5.2: Implement selection with exact/lower_fallback/lowest_fallback/provider_fallback

### T6: Output Paths | agent:Worker | status: completed
- [x] S6.1: Implement title sanitization
- [x] S6.2: Implement episode filename generation with padding
- [x] S6.3: Implement path traversal prevention
- [x] S6.4: Implement partial/final file operations

### T7: FFmpeg Config | agent:Worker | status: completed
- [x] S7.1: Create fake-ffmpeg testdata
- [x] S7.2: Implement FFmpeg runner (executable lookup, arg construction)

## M3: Provider Implementation | status: completed
### T8: Catalog Provider | agent:Worker | status: completed
- [x] S8.1: Research and record sanitized fixtures (in-memory test server)
- [x] S8.2: Implement catalog search, episodes, streams
- [x] S8.3: Live smoke test gated by ORPHION_LIVE_PROVIDER_TEST=1

## M4: Integration | status: completed
### T9: App Service | agent:Worker | status: completed
- [x] S9.1: Implement orchestration service bridging CLI and provider

### T10: CLI Commands | agent:Worker | status: completed
- [x] S10.1: Search command
- [x] S10.2: Download command (non-interactive)
- [x] S10.3: Version and help commands

### T11: Interactive Mode | agent:Worker | status: completed
- [x] S11.1: Implement pterm interactive prompts
- [x] S11.2: Implement default root command interactive flow

### T12: Binary Assembly | agent:Worker | status: completed
- [x] S12.1: Wire up main.go with signal handling and composition
- [x] S12.2: Exit code mapping

## M5: Final Verification | agent:Reviewer | status: completed
### T13: Verification | agent:Reviewer | status: completed
- [x] S13.1: Format and vet (gofmt, go vet)
- [x] S13.2: Run all tests with race detector (go test -race ./...)
- [x] S13.3: Build binary
- [x] S13.4: Integration tests with fake-FFmpeg

## Summary
- **38 unit/integration tests across 10 packages, all passing with race detector**
- Binary builds: ✓
- GO vet passes: ✓
- gofmt clean: ✓