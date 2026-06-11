<!--
*** Orphion — CLI for searching and downloading anime/drama episodes as MKV files
*** Based on Best-README-Template by othneildrew
*** See: https://github.com/othneildrew/Best-README-Template
-->

<!-- PROJECT SHIELDS -->
[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![License][license-shield]][license-url]
[![Go Version][go-shield]][go-url]

<br />
<div align="center">

<img src="orphion.svg" alt="Orphion Logo" width="120" />

<h1 align="center">Orphion</h1>

  <p align="center">
    Search, select, and download anime and drama episodes from your terminal.<br/>
    Interactive TUI or fully scriptable CLI — powered by system FFmpeg.
    <br />
    <br />
    <a href="#features"><strong>Features</strong></a>
    ·
    <a href="#getting-started"><strong>Install</strong></a>
    ·
    <a href="#usage"><strong>Usage</strong></a>
    ·
    <a href="docs/troubleshooting.md"><strong>Troubleshooting</strong></a>
    ·
    <a href="CONTRIBUTING.md"><strong>Contribute</strong></a>
    <br />
    <br />
  </p>
</div>

<!-- ABOUT THE PROJECT -->
## About The Project

Orphion is a Go CLI that searches catalog providers for anime and drama
content, then downloads selected episodes as MKV files. It supports
interactive prompts with an animated TUI and non-interactive command-line
flags for scripting — all backed by the same core services.

## Features

- **Interactive TUI** — animated search, episode selection, and live download progress
- **Scriptable CLI** — `search`, `download`, and `subtitles` commands with flags
- **Multiple providers** — AllAnime and Bettermelon content sources
- **Episode expressions** — `1-4,7` or `all` for flexible episode selection
- **Quality selection** — 1080p, 720p, 480p with automatic fallback
- **Concurrent downloads** — 1–4 parallel episode downloads with live progress
- **Parallel segment fetching** — configurable 1–4 workers for HLS segment download
- **Subtitles** — search and download from SubDL, Kitsunekko, and Jimaku
- **Episode-aware subtitles** — auto-match subtitles to selected episodes
- **Auto-configuration** — sensible defaults on first run, no setup required
- **Cross-platform** — macOS, Linux, and Windows, amd64 and arm64 binaries

<!-- GETTING STARTED -->
## Getting Started

### Prerequisites

- **Go 1.26+** (check `.go-version` for the exact version)
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

#### Download from Releases (recommended)

Pre-built binaries for **macOS**, **Linux**, and **Windows** are available on the [Releases page](https://github.com/bibimoni/orphion/releases/latest).

1. Go to the [latest release](https://github.com/bibimoni/orphion/releases/latest)
2. Download the archive for your platform:
   - **macOS**: `orphion_<version>_darwin_amd64.tar.gz` (Intel) or `orphion_<version>_darwin_arm64.tar.gz` (Apple Silicon)
   - **Linux**: `orphion_<version>_linux_amd64.tar.gz` or `orphion_<version>_linux_arm64.tar.gz`
   - **Windows**: `orphion_<version>_windows_amd64.zip` or `orphion_<version>_windows_arm64.zip`
3. Extract the binary and place it on your `PATH`

#### Quick Install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/bibimoni/orphion/main/install.sh | bash
```

Or build manually:

```bash
bash scripts/dev-setup.sh          # build + install
bash scripts/dev-setup.sh --test    # also run tests
bash scripts/dev-setup.sh --clean   # remove previous install first
```

Or clone and build from source:

```bash
git clone https://github.com/bibimoni/orphion.git
cd orphion
go build -trimpath -ldflags="-s -w" -o dist/orphion ./cmd/orphion
sudo cp dist/orphion /usr/local/bin/
```

#### Configuration

No configuration is required. On first run, Orphion auto-creates
`~/.config/orphion/config.yaml` with sensible defaults:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: allanime
ffmpeg_path: ffmpeg
subtitle_lang: english
segment_workers: 4
```

> **Windows note**: The config file is located at `%APPDATA%\orphion\config.yaml`. On macOS and Linux, it is at `~/.config/orphion/config.yaml`.

You can also create it explicitly:

```bash
orphion config init
```

Override configuration values per-session with flags:

```bash
orphion download --title "Frieren" --episodes "1-4" --quality 720p --concurrency 2 --output ~/Downloads
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- USAGE -->
## Usage

### Interactive Mode

```bash
orphion
```

Orphion prompts for search text, provider selection, episode
selection, and quality preferences — with animated spinners, colored
results, and live download progress.

### CLI Commands

| Command | Description |
|---------|-------------|
| `orphion` | Interactive mode — guided prompts with animated UI |
| `orphion search "Frieren" --type anime` | Search for titles |
| `orphion download --title "Frieren" --episodes "1-4"` | Download by search |
| `orphion download --title-id "allanime:abc123" --episodes "1,3,7"` | Download by ID |
| `orphion subtitles "Frieren" --lang english` | Search and download subtitles |
| `orphion config init` | Create default config |
| `orphion version` | Show version |
| `orphion help` | Show all commands |

### Download Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--title` | Search query to resolve | — |
| `--title-id` | Content ID (skip search) | — |
| `--episodes` | Episode expression | — |
| `--type` | Content type: anime, drama, or both | both |
| `--quality` | Preferred quality (1080p, 720p, 480p) | 1080p |
| `--output` | Output directory | ~/Anime |
| `--concurrency` | Parallel downloads (1–4) | 1 |
| `--force` | Overwrite existing files | false |

**Config-only options** (set in `config.yaml`, not available as CLI flags):

| Key | Description | Default |
|-----|-------------|---------|
| `segment_workers` | Parallel segment download workers for HLS providers (1–4) | 4 |

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

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- SUBTITLES -->
## Subtitles

Orphion can search and download subtitles from multiple providers:

```bash
# Interactive subtitle search
orphion subtitles "Frieren"

# Specify language
orphion subtitles "Naruto" --lang english

# Subtitles are also offered during interactive download
orphion
```

Supported providers: **SubDL**, **Kitsunekko**, **Jimaku**. See [Providers](docs/providers.md) for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- COMMON ISSUES -->
## Common Issues

| Issue | Fix |
|-------|-----|
| `ffmpeg not found` | [Install FFmpeg](#prerequisites) |
| `No results found` | Try different search terms or [switch providers](docs/providers.md) |
| `no streams for episode` | Provider may not have that content — try another provider |
| `.part.mkv` files | Safe to delete — these are incomplete downloads |
| Config errors | `rm ~/.config/orphion/config.yaml && orphion config init` |

See the full [Troubleshooting Guide](docs/troubleshooting.md) for more solutions.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTRIBUTING -->
## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Quick dev setup

```bash
git clone https://github.com/bibimoni/orphion.git
cd orphion
go mod download
go test -race ./...

# Pre-commit hooks
brew install pre-commit
pre-commit install
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->
## License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for details.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTACT -->
## Contact

Project Link: [https://github.com/bibimoni/orphion](https://github.com/bibimoni/orphion)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
[contributors-shield]: https://img.shields.io/github/contributors/bibimoni/orphion.svg?style=flat-square
[contributors-url]: https://github.com/bibimoni/orphion/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/bibimoni/orphion.svg?style=flat-square
[forks-url]: https://github.com/bibimoni/orphion/network/members
[stars-shield]: https://img.shields.io/github/stars/bibimoni/orphion.svg?style=flat-square
[stars-url]: https://github.com/bibimoni/orphion/stargazers
[issues-shield]: https://img.shields.io/github/issues/bibimoni/orphion.svg?style=flat-square
[issues-url]: https://github.com/bibimoni/orphion/issues
[license-shield]: https://img.shields.io/github/license/bibimoni/orphion.svg?style=flat-square
[license-url]: https://github.com/bibimoni/orphion/blob/main/LICENSE
[go-shield]: https://img.shields.io/github/go-mod/go-version/bibimoni/orphion?style=flat-square
[go-url]: https://github.com/bibimoni/orphion
