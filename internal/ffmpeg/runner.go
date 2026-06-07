// Package ffmpeg wraps FFmpeg process execution with progress tracking.
package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds FFmpeg runtime configuration.
type Config struct {
	FFmpegPath string
}

// Progress holds a snapshot of FFmpeg download progress.
type Progress struct {
	Bytes      int64  // total_size from FFmpeg, or downloaded bytes for torrents
	TotalBytes int64  // total torrent size when known
	Speed      string // speed from FFmpeg (e.g. "1.5x") or torrent byte speed
	Peers      int    // active torrent peers when known
	Seeders    int    // connected torrent seeders when known
	TimeMs     int64  // out_time_ms from FFmpeg
	Bitrate    string // bitrate from FFmpeg (e.g. "3400.0kbits/s")
}

// ProgressFunc receives progress updates during an FFmpeg download.
type ProgressFunc func(Progress)

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
		"-map", "0:v",
		"-map", "0:a",
		"-c", "copy",
		output,
	}
}

// ProgressArgs builds the FFmpeg argument list for a download with progress
// reporting. It adds -progress pipe:2 and -stats_period to enable structured
// progress output on stderr.
func (r *Runner) ProgressArgs(url, output, referer, userAgent string) []string {
	headers := fmt.Sprintf("Referer: %s\r\nUser-Agent: %s\r\n", referer, userAgent)
	return []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "warning",
		"-progress", "pipe:2",
		"-stats_period", "1",
		"-headers", headers,
		"-i", url,
		"-map", "0:v",
		"-map", "0:a",
		"-c", "copy",
		output,
	}
}

// Execute runs the FFmpeg binary.
func (r *Runner) Execute(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, r.config.FFmpegPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecuteWithProgress runs the FFmpeg binary and calls fn with progress updates
// parsed from stderr.
func (r *Runner) ExecuteWithProgress(ctx context.Context, args []string, fn ProgressFunc) error {
	cmd := exec.CommandContext(ctx, r.config.FFmpegPath, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		parseProgressOutput(stderr, fn)
	}()

	waitErr := cmd.Wait()
	<-done
	return waitErr
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
	_ = os.Remove(path)
}

// parseProgressOutput reads FFmpeg's -progress key=value output.
func parseProgressOutput(r io.Reader, fn ProgressFunc) {
	scanner := bufio.NewScanner(r)
	var p Progress

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		key, value, ok := parseKV(line)
		if !ok {
			continue
		}

		switch key {
		case "out_time_ms":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				p.TimeMs = v
			}
		case "total_size":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				p.Bytes = v
			}
		case "speed":
			p.Speed = value
		case "bitrate":
			p.Bitrate = value
		case "progress":
			if value == "continue" && fn != nil {
				fn(p)
			}
		}
	}
}

func parseKV(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	return line[:idx], line[idx+1:], true
}
