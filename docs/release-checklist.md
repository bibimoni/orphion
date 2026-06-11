# Release Checklist

Use this checklist before publishing a new Orphion release.

## Pre-Release Verification

- [ ] All tests pass: `go test -race ./...`
- [ ] Go vet passes clean: `go vet ./...`
- [ ] Lint passes: `golangci-lint run --timeout=5m`
- [ ] Installer tests pass: `bash scripts/test-install.sh`
- [ ] Release config tests pass: `bash scripts/test-release-config.sh`
- [ ] Dev setup works: `ORPHION_INSTALL_DIR=/tmp/orphion-test bash scripts/dev-setup.sh --test`
- [ ] Build succeeds for all targets: `GOOS=darwin GOARCH=arm64 go build ./cmd/orphion`

## Documentation

- [ ] README.md is up to date
- [ ] CHANGELOG.md has an entry for this release
- [ ] docs/troubleshooting.md is current
- [ ] docs/providers.md lists all supported providers
- [ ] CLI help text is accurate (read-only: `orphion --help`)

## Configuration

- [ ] Version in `internal/cli.Version` is set via ldflags (no hardcoded version)
- [ ] Go version in `.go-version` matches `go.mod` toolchain directive
- [ ] `.goreleaser.yaml` targets darwin/linux, amd64/arm64
- [ ] `.goreleaser.yaml` ldflags include `-X github.com/bibimoni/orphion/internal/cli.Version={{.Tag}}`

## CI / Automation

- [ ] GitHub Actions CI workflow (`.github/workflows/ci.yml`) is green on main
  - Lint job uses golangci-lint
  - Test job runs with race detector and coverage
  - Live download test runs for PRs (allanime provider)
  - Bettermelon smoke test runs for PRs
- [ ] GitHub Actions release workflow (`.github/workflows/release.yml`) is valid
  - Triggered on tag push (`v*`)
  - Runs tests before releasing
  - Runs installer and release config tests
  - Uses goreleaser to publish

## Install Script

- [ ] `install.sh` handles darwin/linux detection
- [ ] `install.sh` handles amd64/arm64 detection
- [ ] Checksum verification works (tested by `scripts/test-install.sh`)
- [ ] FFmpeg availability check is included
- [ ] Test mode (`ORPHION_INSTALLER_TEST=1`) works

## Release Steps

1. [ ] Update CHANGELOG.md with the new version entry
2. [ ] Commit all changes: `git commit -m "chore: prepare for vX.Y.Z"`
3. [ ] Create and push the tag: `git tag vX.Y.Z && git push origin vX.Y.Z`
4. [ ] Verify CI triggers on the tag
5. [ ] Verify goreleaser publishes the GitHub release
6. [ ] Verify the release contains binaries for all 4 targets:
   - `orphion_X.Y.Z_darwin_amd64.tar.gz`
   - `orphion_X.Y.Z_darwin_arm64.tar.gz`
   - `orphion_X.Y.Z_linux_amd64.tar.gz`
   - `orphion_X.Y.Z_linux_arm64.tar.gz`
7. [ ] Verify `checksums.txt` is attached to the release
8. [ ] Test install from release: `curl -fsSL https://raw.githubusercontent.com/bibimoni/orphion/main/install.sh | bash`
9. [ ] Announce the release

## Post-Release

- [ ] Verify `orphion version` shows the correct tag
- [ ] Verify install script resolves the new version
- [ ] Update main branch default config if needed
