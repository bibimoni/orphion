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

<h1 align="center">🎬 Orphion</h1>

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

#### Quick Install (recommended)

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
provider: catalog
ffmpeg_path: ffmpeg
```

You can also create it explicitly:

```bash
orphion config init
```

When using orphion, use can use flags to override configuration values:

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
selection, and quality preferences — with animated spinners, colored
results, and live download progress.

### CLI Commands

| Command | Description |
|---------|-------------|
| `orphion` | Interactive mode — guided prompts with animated UI |
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

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTRIBUTING -->
## Contributing

Contributions are welcome. Before submitting a PR, please review:

1. [`docs/architecture.md`](docs/architecture.md) — package boundaries
2. [`docs/usage.md`](docs/usage.md) — user-facing documentation

### Dev Setup

```bash
git clone https://github.com/bibimoni/orphion.git
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
