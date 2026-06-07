# Project Context

## Environment
- Language: Go 1.24.4
- Runtime: macOS
- Build: go build -trimpath -ldflags "-s -w" -o dist/orphion ./cmd/orphion
- Test: go test -race ./...
- Package Manager: Go modules (go mod)
- Lint: go vet

## Project Type
- Application (CLI)

## Infrastructure
- Container: None
- Orchestration: None
- CI/CD: None
- Cloud: None

## Structure (planned)
- Source: cmd/orphion/, internal/
- Tests: alongside source files (*_test.go)
- Docs: docs/
- Entry: cmd/orphion/main.go

## Conventions (from AGENTS.md and docs/)
- Naming: Go conventions (camelCase for unexported, PascalCase for exported)
- Error handling: typed internal errors, concise user-facing messages, %w wrapping
- Testing: TDD for features/bugfixes, never contact live provider in regular tests
- Path style: Unix-style `/`
- Configuration: YAML with strict field checking
- External deps: Cobra 1.9.1, pterm 0.12.81, yaml.v3 3.0.1
- System dep: FFmpeg (user-installed, validated at runtime)

## Current State
- Phase 1 implementation
- No Go code exists yet
- Starting from scratch following docs/implementation-plan.md