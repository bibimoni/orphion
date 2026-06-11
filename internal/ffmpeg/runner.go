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

// Progress holds a snapshot of download/remux progress.
type Progress struct {
	Bytes      int64  // downloaded bytes
	TotalBytes int64  // total size when known
	Speed      string // download speed (e.g. "1.5x" or "2.5 MiB/s")
	TimeMs     int64  // out_time_ms from FFmpeg
	Bitrate    string // bitrate from FFmpeg (e.g. "3400.0kbits/s")

	// Segment download progress (set by providers like bettermelon
	// that download HLS segments before running ffmpeg).
	SegmentsDone  int    // number of segments downloaded
	SegmentsTotal int    // total number of segments
	Phase         string // current phase: "segments" or "" (ffmpeg)
}

// ProgressFunc receives progress updates during an FFmpeg download.
type ProgressFunc func(Progress)

// Runner runs FFmpeg processes.
type Runner struct {
	config Config
}

// NewRunner creates a new FFmpeg runner.
// It verifies that the FFmpeg binary is findable on the given path.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.FFmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg path is required")
	}
	// Resolve the binary path. LookPath searches PATH for bare names
	// and returns the absolute path. For explicit paths, it verifies
	// the file exists and is executable.
	p, err := exec.LookPath(cfg.FFmpegPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found at %q: install ffmpeg and ensure it is on your PATH", cfg.FFmpegPath)
	}
	return &Runner{config: Config{FFmpegPath: p}}, nil
}

// Args builds the FFmpeg argument list for a download.
func (r *Runner) Args(url, output, referer, userAgent string) []string {
	args := []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "warning",
		"-err_detect", "ignore_err",
		"-fflags", "+genpts",
	}
	if !strings.HasPrefix(url, "file://") {
		headers := fmt.Sprintf("Referer: %s\r\nUser-Agent: %s\r\n", referer, userAgent)
		args = append(args, "-headers", headers)
	}
	args = appendHLSFlags(args, url)
	args = append(args,
		"-i", url,
		"-map", "0:v?",
		"-map", "0:a?",
		"-c", "copy",
		"-max_interleave_delta", "0",
		"-avoid_negative_ts", "make_zero",
		output,
	)
	return args
}

// ProgressArgs builds the FFmpeg argument list for a download with progress
// reporting. It adds -progress pipe:2 and -stats_period to enable structured
// progress output on stderr.
func (r *Runner) ProgressArgs(url, output, referer, userAgent string) []string {
	args := []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "warning",
		"-err_detect", "ignore_err",
		"-fflags", "+genpts",
		"-progress", "pipe:2",
		"-stats_period", "1",
	}
	if !strings.HasPrefix(url, "file://") {
		headers := fmt.Sprintf("Referer: %s\r\nUser-Agent: %s\r\n", referer, userAgent)
		args = append(args, "-headers", headers)
	}
	args = appendHLSFlags(args, url)
	args = append(args,
		"-i", url,
		"-map", "0:v?",
		"-map", "0:a?",
		"-c", "copy",
		"-max_interleave_delta", "0",
		"-avoid_negative_ts", "make_zero",
		output,
	)
	return args
}

// appendHLSFlags adds flags needed for HLS streams with obfuscated or
// non-standard segment extensions (e.g. .jpg, .html used by CDNs to
// disguise video segments). For local m3u8 files referencing remote
// segments, protocol_whitelist is added to allow file+https access.
func appendHLSFlags(args []string, url string) []string {
	if !isHLS(url) {
		return args
	}
	// Default allowed_segment_extensions in ffmpeg omits many fake extensions
	// used by anime CDNs (e.g. .jpg, .js, .css disguising MPEG-TS segments).
	// Append the full set of known fake extensions so ffmpeg will fetch them.
	args = append(args,
		"-allowed_extensions", "ALL",
		"-allowed_segment_extensions", "3gp,aac,avi,ac3,eac3,flac,mkv,m3u8,m4a,m4s,m4v,mpg,mov,mp2,mp3,mp4,mpeg,mpegts,ogg,ogv,oga,ts,vob,vtt,wav,webvtt,cmfv,cmfa,ec3,fmp4,html,jpg,jpeg,js,css,txt,png,webp,gif,svg,ico,json,xml",
		"-extension_picky", "0",
	)
	if strings.HasPrefix(url, "file://") {
		args = append(args, "-protocol_whitelist", "file,https,http,crypto,data,tcp,tls")
	}
	return args
}

// isHLS returns true if the URL looks like an HLS manifest.
func isHLS(url string) bool {
	lower := strings.ToLower(url)
	return (strings.Contains(lower, ".m3u8") || strings.Contains(lower, "/proxy?url=")) && !strings.Contains(lower, ".ts")
}

// Execute runs the FFmpeg binary and captures stderr for error reporting.
func (r *Runner) Execute(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, r.config.FFmpegPath, args...)
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg: create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg: start: %w", err)
	}

	// Capture stderr output (limited to 4KB to avoid memory issues).
	stderrData, _ := io.ReadAll(io.LimitReader(stderr, 4096))

	waitErr := cmd.Wait()
	if waitErr != nil {
		// Include captured stderr in the error for diagnostics.
		if len(stderrData) > 0 {
			msg := strings.TrimSpace(string(stderrData))
			if msg != "" {
				return fmt.Errorf("ffmpeg: %w (%s)", waitErr, truncate(msg, 200))
			}
		}
		return waitErr
	}
	return nil
}

// truncate shortens a string to at most n characters, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// ExecuteWithProgress runs the FFmpeg binary and calls fn with progress updates
// parsed from stderr. On failure, the last stderr lines are included in the error.
func (r *Runner) ExecuteWithProgress(ctx context.Context, args []string, fn ProgressFunc) error {
	cmd := exec.CommandContext(ctx, r.config.FFmpegPath, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg: create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg: start: %w", err)
	}

	done := make(chan struct{})
	var lastLines []string
	go func() {
		defer close(done)
		lastLines = parseProgressOutputWithDiags(stderr, fn)
	}()

	waitErr := cmd.Wait()
	<-done
	if waitErr != nil && len(lastLines) > 0 {
		joined := strings.TrimSpace(strings.Join(lastLines, "; "))
		if joined != "" {
			return fmt.Errorf("ffmpeg: %w (%s)", waitErr, truncate(joined, 200))
		}
	}
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
//
//nolint:unused
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

// parseProgressOutputWithDiags is like parseProgressOutput but also returns
// the last few non-progress stderr lines for diagnostic error reporting.
func parseProgressOutputWithDiags(r io.Reader, fn ProgressFunc) []string {
	scanner := bufio.NewScanner(r)
	var p Progress
	const diagMaxLines = 5
	var diags []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		key, value, ok := parseKV(line)
		if !ok {
			// Non key=value line — keep for diagnostics (e.g. error messages).
			if len(diags) >= diagMaxLines {
				diags = diags[1:]
			}
			diags = append(diags, line)
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
			if value == "end" {
				// FFmpeg reports progress=end on completion or error.
				// Keep this as a diagnostic hint.
				if len(diags) >= diagMaxLines {
					diags = diags[1:]
				}
				diags = append(diags, "progress=end")
			}
			if value == "continue" && fn != nil {
				fn(p)
			}
		}
	}
	return diags
}

func parseKV(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	return line[:idx], line[idx+1:], true
}
