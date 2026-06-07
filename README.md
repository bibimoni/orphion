# Orphion

A command-line tool to search a catalog provider and download selected
episodes as MKV files via system FFmpeg.

## Features

- **Zero-config first run** — works immediately, auto-creates config with sensible defaults
- **Interactive mode** — guided search, selection, and download with animated UI
- **Download progress** — live speed, size, and animated spinner during downloads
- **Episode expressions** — download single episodes, ranges, lists, or all at once
- **Quality selection** — prefers 1080p, falls back gracefully
- **Concurrent downloads** — up to 4 episodes in parallel

## Prerequisites

- **FFmpeg** — used to remux streams into MKV files

| Platform | Command |
|----------|----------|
| macOS | `brew install ffmpeg` |
| Ubuntu/Debian | `sudo apt install ffmpeg` |
| Arch | `sudo pacman -S ffmpeg` |

## Installation

### Install Script (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/bibimoni/orphion/main/install.sh | bash
```

### Build From Source

Requires Go 1.24+ (check `.go-version` for the exact version):

```bash
git clone https://github.com/bibimoni/orphion.git
cd orphion
go build -trimpath -ldflags="-s -w" -o dist/orphion ./cmd/orphion
sudo cp dist/orphion /usr/local/bin/
```

## Usage

### Interactive Mode

Just run `orphion` with no arguments for a guided experience:

```bash
orphion
```

### CLI Commands

| Command | Description |
|---------|-------------|
| `orphion` | Interactive mode |
| `orphion search "Frieren"` | Search for titles |
| `orphion download --title "Frieren" --episodes "1-4"` | Download episodes |
| `orphion download --title-id "abc123" --episodes "1,3,7"` | Download by ID |
| `orphion version` | Show version |

### Episode Expressions

```
1             Single episode
1-4           Range
1,3,7         List
1-3,7,10-12   Mixed
all           All episodes
```

### Output Structure

```
~/Anime/
└── Frieren/
    ├── Episode 01.mkv
    └── Episode 02.mkv
```

## Configuration

On first run, Orphion auto-creates `~/.config/orphion/config.yaml` with defaults:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: catalog
ffmpeg_path: ffmpeg
```

Edit the file to customize. You can also recreate it:

```bash
orphion config init    # errors if config already exists
```

### CLI Flags (override config)

| Flag | Description |
|------|-------------|
| `--output` | Output directory |
| `--quality` | Preferred quality (e.g. 720p) |
| `--concurrency` | Download concurrency (1-4) |
| `--force` | Overwrite existing files |

## Contributing

Contributions are welcome. Before submitting a PR, review the project
documentation in [`docs/`](docs/).

### Dev Setup

```bash
git clone https://github.com/bibimoni/orphion.git
cd orphion
go mod download
go test -race ./...
```

## License

MIT — see [`LICENSE`](LICENSE).
