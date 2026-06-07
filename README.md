# Orphion

Orphion is a Go CLI that searches an AllAnime-derived source and downloads selected episodes as MKV files through system FFmpeg.

## Prerequisites

- Go 1.24+
- FFmpeg

## Install

```bash
git clone https://github.com/distiled/orphion.git
cd orphion
go build -trimpath -ldflags="-s -w" -o dist/orphion ./cmd/orphion
```

## Usage

### Interactive mode

```bash
orphion
```

### Non-interactive commands

```bash
orphion version
orphion config init
orphion search "Frieren"
orphion download --title-id "id" --episodes "1-4" --type anime
```

## Development

```bash
go test -race ./...
go mod tidy
```

## License

MIT