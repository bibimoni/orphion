# Mission: Implement Bettermelon Provider ✅

## M1: Core Bettermelon Provider Package ✅
### T1.1: Create provider package structure | agent:Worker ✅
- [x] S1.1.1: Create `internal/provider/bettermelon/` package with config.go, client.go, provider.go
- [x] S1.1.2: Add Bettermelon constants to `internal/common/constants.go`
- [x] S1.1.3: Implement Config + DefaultConfig + validation
- [x] S1.1.4: Implement Client (Search, Episodes, Streams)
- [x] S1.1.5: Implement Provider struct satisfying provider.Provider interface

### T1.2: Unit tests for Bettermelon provider | agent:Worker | depends:T1.1 ✅
- [x] S1.2.1: Config tests (default values, validation)
- [x] S1.2.2: Client tests (Search, Episodes, Streams with mock HTTP)
- [x] S1.2.3: Live provider test (ORPHION_LIVE_PROVIDER_TEST=1)

## M2: Integration | depends:M1 ✅
### T2.1: Wire Bettermelon into application | agent:Worker ✅
- [x] S2.1.1: Register bettermelon provider in `cmd/orphion/main.go`
- [x] S2.1.2: Update normalizeProviderName for bettermelon

### T2.2: Full verification | agent:Reviewer | depends:T2.1 ✅
- [x] S2.2.1: `go vet ./...` passes
- [x] S2.2.2: `go test -race ./...` passes (all 16 suites)
- [x] S2.2.3: `go build ./cmd/orphion` succeeds
