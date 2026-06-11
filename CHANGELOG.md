# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Search anime/drama from AllAnime and Bettermelon providers
- Download episodes as MKV via system FFmpeg
- Interactive mode with animated TUI (branded header, spinners, live progress)
- Non-interactive CLI flags for scripting (`download`, `search`, `subtitles`)
- Episode expression support (`1-4`, `1,3,7`, `1-3,7,10-12`, `all`)
- Quality selection (1080p, 720p, 480p)
- Concurrent downloads (1–4, with multi-line live progress)
- Subtitle search and download from SubDL, Kitsunekko, and Jimaku providers
- Episode-aware subtitle matching (auto-match by episode number)
- Fuzzy token matching for subtitle search (handles typos and partial matches)
- Auto-configuration on first run (creates `~/.config/orphion/config.yaml`)
- YAML configuration with strict validation and defaults
- Cross-platform binary releases (darwin/linux, amd64/arm64)
- One-line installer with checksum verification (`install.sh`)
- GitHub Actions CI/CD pipeline (lint, test, coverage, release)
- Pre-commit hook integration for golangci-lint
- Session-only config overrides (no disk writes for per-run settings)
- "Go back" navigation in interactive prompts
- Segment-level download progress for HLS providers (Bettermelon)
- Automatic season navigation for SubDL (skips specials, tries first season)
- Atomic file writes (`.part.mkv` → `.mkv` rename on success)

### Changed

- Improved subtitle matching with shared-title-word rejection (e.g., "Hero Bank" no longer matches "Sentenced to Be a Hero")
- Bettermelon provider now uses lazy stream preparation (no upfront segment download)
- Provider switching in interactive mode no longer implies content type
- FFmpeg args include error-resilient flags (`-err_detect`, `-fflags`, `-map`)

### Fixed

- FFmpeg muxing error (exit status 183, "Invalid data found when processing input")
- IME/terminal escape artifacts in interactive text input on macOS
- Whitespace trimming for CLI flags and search output
- GraphQL error messages now include actual error details
