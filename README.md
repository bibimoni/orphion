<!--
*** Orphion — CLI for searching and downloading anime/drama episodes as MKV files
*** Based on Best-README-Template by othneildrew
*** See: https://github.com/othneildrew/Best-README-Template
-->

<!-- PROJECT SHIELDS -->
<!--
*** Reference-style links at the bottom of the document
-->
[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![License][license-shield]][license-url]
[![Go Version][go-shield]][go-url]

<br />
<div align="center">

<h1 align="center">Orphion</h1>

  <p align="center">
    A command-line tool to search an anime source and download<br/>
    selected episodes as MKV files via system FFmpeg.
    <br />
    <br />
    <a href="#usage"><strong>Explore the docs »</strong></a>
    <br />
    <br />
  </p>
</div>

<!-- ABOUT THE PROJECT -->
## About The Project

Orphion is a Go CLI that searches a catalog provider for anime
and drama content, then downloads selected episodes as MKV
files. It supports interactive prompts and non-interactive
command-line flags backed by the same application services.

**Phase 1 scope:** Command-line downloader only. No web server,
database, Docker, playback, subtitles, or account management.

<!-- GETTING STARTED -->
## Getting Started

### Prerequisites

- **Go 1.24+** (check `.go-version` for the exact version)
- **FFmpeg** — used to download and remux streams into MKV files

Install FFmpeg:

| Platform | Command |
|----------|---------|
| macOS | `brew install ffmpeg` |
| Ubuntu/Debian | `sudo apt install ffmpeg` |
| Fedora | `sudo dnf install ffmpeg` |
| Arch | `sudo pacman -S ffmpeg` |
| Windows | Download from [ffmpeg.org](https://ffmpeg.org/download.html) |

Verify the installation:

```bash
ffmpeg -version
```

### Installation

#### Quick Install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/distiled/orphion/main/install.sh | bash
```

Or clone and build manually:

```bash
git clone https://github.com/distiled/orphion.git
cd orphion
go build -trimpath -ldflags="-s -w" -o dist/orphion ./cmd/orphion
sudo cp dist/orphion /usr/local/bin/
```

#### Configuration

Initialize default configuration:

```bash
orphion config init
```

This creates `~/.config/orphion/config.yaml`:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: catalog
default_type: anime
ffmpeg_path: ffmpeg
```

Flags override configuration values:

```bash
orphion download --output /Volumes/Media --quality 720p --concurrency 2 --episodes "1-4"
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- USAGE -->
## Usage

### Interactive Mode

```bash
orphion
```

Orphion prompts for search text, content type (anime/drama), episode
selection, and quality preferences.

### CLI Commands

| Command | Description |
|---------|-------------|
| `orphion` | Interactive mode — guided prompts |
| `orphion search "Frieren" --type anime` | Search for titles |
| `orphion download --title "Frieren" --episodes "1-4"` | Download by search |
| `orphion download --title-id "catalog:abc123" --episodes "1,3,7"` | Download by ID |
| `orphion config init` | Create default config |
| `orphion version` | Show version |
| `orphion help` | Show all commands |

### Episode Expressions

```
1             Single episode
1-4           Range
1,3,7         List
1-3,7,10-12   Mixed
all           All episodes
```

### Output Layout

```
~/Anime/
└── Frieren-Beyond-Journeys-End/
    ├── Episode 01.mkv
    └── Episode 02.mkv
```

Downloads use `.part.mkv` during transfer and rename to `.mkv` only on success.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | One or more downloads failed |
| 2 | Usage/configuration error |
| 3 | Provider/search/selection error |
| 130 | Interrupted |

_For detailed documentation, see [docs/usage.md](docs/usage.md)._

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- ROADMAP -->
## Roadmap

- [x] Interactive and non-interactive CLI
- [x] Anime and drama search
- [x] Episode selection with quality fallback
- [x] Configurable download concurrency
- [x] Partial-file cleanup on failure
- [x] YAML configuration
- [ ] Subtitle support
- [ ] Download resume

See the [implementation plan](docs/implementation-plan.md) for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- ARCHITECTURE -->
## Architecture

```
CLI input
   |
   v
command layer
   |
   +--> configuration
   +--> interactive prompts
   +--> non-interactive validation
   |
   v
application services
   |
   +--> provider interface
   |      |
   |      +--> catalog adapter
   |
   +--> episode selection
   +--> quality selection
   +--> path generation
   +--> download scheduler
          |
          +--> FFmpeg process runner
```

All commands use shared application services. Provider-specific
details are isolated inside `internal/provider/catalog`.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTRIBUTING -->
## Contributing

Contributions are welcome. Before submitting a PR, please review:

1. [`AGENTS.md`](AGENTS.md) — conventions and project rules
2. [`docs/architecture.md`](docs/architecture.md) — package boundaries
3. [`docs/usage.md`](docs/usage.md) — user-facing documentation

### Dev Setup

```bash
git clone https://github.com/distiled/orphion.git
cd orphion
go mod download
go test -race ./...

# Pre-commit hooks
brew install pre-commit
pre-commit install
```

### Testing

```bash
go test -race ./...
go vet ./...
golangci-lint run
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->
## License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTACT -->
## Contact

Project Link: [https://github.com/distiled/orphion](https://github.com/distiled/orphion)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
[contributors-shield]: https://img.shields.io/github/contributors/distiled/orphion.svg?style=flat-square
[contributors-url]: https://github.com/distiled/orphion/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/distiled/orphion.svg?style=flat-square
[forks-url]: https://github.com/distiled/orphion/network/members
[stars-shield]: https://img.shields.io/github/stars/distiled/orphion.svg?style=flat-square
[stars-url]: https://github.com/distiled/orphion/stargazers
[issues-shield]: https://img.shields.io/github/issues/distiled/orphion.svg?style=flat-square
[issues-url]: https://github.com/distiled/orphion/issues
[license-shield]: https://img.shields.io/github/license/distiled/orphion.svg?style=flat-square
[license-url]: https://github.com/distiled/orphion/blob/main/LICENSE
[go-shield]: https://img.shields.io/github/go-mod/go-version/distiled/orphion?style=flat-square
[go-url]: https://github.com/distiled/orphion