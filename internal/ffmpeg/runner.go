package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds FFmpeg runtime configuration.
type Config struct {
	FFmpegPath string
}

// Runner runs FFmpeg processes.
type Runner struct {
	config Config
}

// NewRunner creates a new FFmpeg runner.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.FFmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg path is required")
	}
	return &Runner{config: cfg}, nil
}

// Args builds the FFmpeg argument list for a download.
func (r *Runner) Args(url, output, referer, userAgent string) []string {
	headers := fmt.Sprintf("Referer: %s\r\nUser-Agent: %s\r\n", referer, userAgent)
	return []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "warning",
		"-headers", headers,
		"-i", url,
		"-map", "0",
		"-c", "copy",
		output,
	}
}

// EnsureDir creates parent directories for the output file.
func (r *Runner) EnsureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0o755)
}

// RenamePart atomically renames a .part.mkv to .mkv.
func (r *Runner) RenamePart(part, final string) error {
	return os.Rename(part, final)
}

// CleanupPartial removes partial files.
func (r *Runner) CleanupPartial(path string) {
	os.Remove(path)
}