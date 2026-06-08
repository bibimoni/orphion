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

---

# Mission: Implement HiAnime Provider ✅

## M3: Core HiAnime Provider Package ✅
### T3.1: Create provider package structure | agent:Worker ✅
- [x] S3.1.1: Create `internal/provider/hianime/` package with config.go, client.go, provider.go | verified | evidence: go vet + 17 tests pass
- [x] S3.1.2: Add HiAnime constants to `internal/common/constants.go` | verified | evidence: HiAnimeBaseURL, HiAnimeRapidCloudURL, HiAnimeKeyURL, AniListAPIURL defined
- [x] S3.1.3: Implement Config + DefaultConfig + validation | verified | evidence: 4 config tests pass
- [x] S3.1.4: Implement Client (Search, Episodes, Streams, MegaCloud extractor) | verified | evidence: 13 client tests pass incl. encryption round-trip
- [x] S3.1.5: Implement Provider struct satisfying provider.Provider interface | verified | evidence: var _ provider.Provider = (*Provider)(nil) compiles

### T3.2: Unit tests for HiAnime provider | agent:Worker | depends:T3.1 ✅
- [x] S3.2.1: Config tests (default values, validation, injected HTTP client) | verified | evidence: 4 tests pass
- [x] S3.2.2: Client tests (Search, Episodes, Streams, encryption, error handling) | verified | evidence: 13 tests pass
- [x] S3.2.3: Live provider test (ORPHION_LIVE_PROVIDER_TEST=1) | verified | evidence: live_test.go created, skipped without env var, compiles clean

## M4: Integration | depends:M3 ✅
### T4.1: Wire HiAnime into application | agent:Worker ✅
- [x] S4.1.1: Register hianime provider in `cmd/orphion/main.go` | verified | evidence: go build passes, provider in switch + newProviders map
- [x] S4.1.2: Update normalizeProviderName if needed | verified | no alias needed, hianime used directly

### T4.2: Full verification | agent:Reviewer | depends:T4.1 ✅
- [x] S4.2.1: `go vet ./internal/provider/hianime/` passes | evidence: clean output
- [x] S4.2.2: `go test -race ./internal/provider/hianime/` passes (17 tests) | evidence: all PASS
- [x] S4.2.3: `go build ./cmd/orphion` succeeds | evidence: clean build
- [x] S4.2.4: `go test -race ./...` passes all suites | evidence: all 16 packages PASS, SYNC-2 resolved (bettermelon tests updated for proxied URLs)
