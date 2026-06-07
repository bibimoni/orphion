# Orphion

A command-line tool to search a catalog provider and download selected
episodes as MKV files via system FFmpeg.

## Prerequisites

- **FFmpeg** — used to remux streams into MKV files

Install FFmpeg:

| Platform | Command |
|----------|----------|
| macOS | `brew install ffmpeg` |
| Ubuntu/Debian | `sudo apt install ffmpeg` |
| Arch | `sudo pacman -S ffmpeg` |

```bash
ffmpeg -version
```

## Installation

### Install Script (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/bibimoni/orphion/main/install.sh | bash
```

### Build From Source

Requires Go 1.26+ (check `.go-version` for the exact version):

```bash
git clone https://github.com/bibimoni/orphion.git
cd orphion
go build -trimpath -ldflags="-s -w" -o dist/orphion ./cmd/orphion
sudo cp dist/orphion /usr/local/bin/
```

### Configuration

```bash
orphion config init
```

This creates `~/.config/orphion/config.yaml`:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: catalog
ffmpeg_path: ffmpeg
```

## Usage

### Interactive Mode

```bash
orphion
```

### CLI Commands

| Command | Description |
|---------|-------------|
| `orphion` | Interactive mode |
| `orphion search "Frieren"` | Search for titles |
| `orphion download --title "Frieren" --episodes "1-4"` | Download episodes |
| `orphion download --title-id "catalog:abc123" --episodes "1,3,7"` | Download by ID |
| `orphion config init` | Create default config |
| `orphion version` | Show version |

### Episode Expressions

```
1             Single episode
1-4           Range
1,3,7         List
1-3,7,10-12   Mixed
all           All episodes
```

### Output

```
~/Anime/
└── Frieren-Beyond-Journeys-End/
    ├── Episode 01.mkv
    └── Episode 02.mkv
```

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

Project Link: [https://github.com/bibimoni/orphion](https://github.com/bibimoni/orphion)