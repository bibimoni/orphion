package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testFFmpegPath returns a valid ffmpeg path for testing.
// It creates a minimal executable script so that LookPath succeeds.
func testFFmpegPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(dir, name)
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// newTestRunner creates a Runner with a fake ffmpeg binary for testing.
func newTestRunner(t *testing.T) *Runner {
	t.Helper()
	r, err := NewRunner(Config{FFmpegPath: testFFmpegPath(t)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRunnerArgs(t *testing.T) {
	r, err := NewRunner(Config{FFmpegPath: testFFmpegPath(t)})
	if err != nil {
		t.Fatal(err)
	}
	args := r.Args("https://example.com/stream.m3u8", "/tmp/test.mkv", "https://example.com", "orphion/1.0")
	if len(args) == 0 {
		t.Fatal("empty args")
	}
	// Check critical flags.
	hasCopy := false
	hasStdin := false
	hasMapV := false
	hasMapA := false
	hasErrDetect := false
	hasFFlags := false
	for i, a := range args {
		if a == "-c" {
			hasCopy = true
		}
		if a == "-nostdin" {
			hasStdin = true
		}
		if a == "-map" && i+1 < len(args) && args[i+1] == "0:v?" {
			hasMapV = true
		}
		if a == "-map" && i+1 < len(args) && args[i+1] == "0:a?" {
			hasMapA = true
		}
		if a == "-err_detect" && i+1 < len(args) && args[i+1] == "ignore_err" {
			hasErrDetect = true
		}
		if a == "-fflags" && i+1 < len(args) && args[i+1] == "+genpts" {
			hasFFlags = true
		}
	}
	if !hasCopy || !hasStdin {
		t.Error("missing critical args")
	}
	if !hasMapV {
		t.Error("missing -map 0:v? (optional video map)")
	}
	if !hasMapA {
		t.Error("missing -map 0:a? (optional audio map)")
	}
	if !hasErrDetect {
		t.Error("missing -err_detect ignore_err (error resilience)")
	}
	if !hasFFlags {
		t.Error("missing -fflags +genpts (PTS generation)")
	}
}

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRunner(Config{FFmpegPath: testFFmpegPath(t)})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "test", "sub", "file.mkv")
	if err := r.EnsureDir(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "test", "sub")); err != nil {
		t.Fatal("directory not created:", err)
	}
}

func TestRenamePart(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRunner(Config{FFmpegPath: testFFmpegPath(t)})
	if err != nil {
		t.Fatal(err)
	}

	part := filepath.Join(dir, "ep.part.mkv")
	final := filepath.Join(dir, "ep.mkv")
	if err := os.WriteFile(part, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := r.RenamePart(part, final); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(final); err != nil {
		t.Fatal("final file not found")
	}
}

func TestCleanupPartial(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRunner(Config{FFmpegPath: testFFmpegPath(t)})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "ep.part.mkv")
	if err := os.WriteFile(p, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.CleanupPartial(p)
	if _, err := os.Stat(p); err == nil {
		t.Fatal("partial file still exists after cleanup")
	}
}

func TestNewRunnerRejectsMissingFFmpeg(t *testing.T) {
	_, err := NewRunner(Config{FFmpegPath: "/nonexistent/path/ffmpeg"})
	if err == nil {
		t.Fatal("expected error for missing ffmpeg binary")
	}
	if _, ok := err.(*exec.Error); !ok {
		// The error should wrap an exec.Error from LookPath.
		t.Logf("error = %v (type %T)", err, err)
	}
}

func TestNewRunnerRejectsEmptyPath(t *testing.T) {
	_, err := NewRunner(Config{FFmpegPath: ""})
	if err == nil {
		t.Fatal("expected error for empty ffmpeg path")
	}
}

func TestRunFakeFFmpeg(t *testing.T) {
	if os.Getenv("ORPHION_LIVE_FFMPEG_TEST") != "1" {
		t.Skip("skipping fake FFmpeg integration test; set ORPHION_LIVE_FFMPEG_TEST=1")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "test.mkv")

	// Use real FFmpeg to validate fake-ffmpeg
	cmd := exec.CommandContext(context.Background(), "../../testdata/fake-ffmpeg", "--mode=success", "--", out)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal("output file not created")
	}
}

func TestNewRunnerWithValidPath(t *testing.T) {
	r, err := NewRunner(Config{FFmpegPath: testFFmpegPath(t)})
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("runner is nil")
	}
}

func TestArgsContainsHeadersForNonFileURL(t *testing.T) {
	r := newTestRunner(t)
	args := r.Args("https://example.com/stream.m3u8", "/tmp/test.mkv", "https://referer.com", "test-agent")

	hasHeaders := false
	for i, a := range args {
		if a == "-headers" && i+1 < len(args) {
			hasHeaders = true
			if !strings.Contains(args[i+1], "Referer: https://referer.com") {
				t.Error("headers missing Referer")
			}
			if !strings.Contains(args[i+1], "User-Agent: test-agent") {
				t.Error("headers missing User-Agent")
			}
		}
	}
	if !hasHeaders {
		t.Error("missing -headers flag for non-file URL")
	}
}

func TestArgsOmitsHeadersForFileURL(t *testing.T) {
	r := newTestRunner(t)
	args := r.Args("file:///tmp/local.m3u8", "/tmp/test.mkv", "https://referer.com", "test-agent")

	for i, a := range args {
		if a == "-headers" {
			t.Errorf("found -headers flag for file:// URL at index %d", i)
		}
	}
}

func TestArgsContainsHLSFlagsForM3U8(t *testing.T) {
	r := newTestRunner(t)
	args := r.Args("https://example.com/stream.m3u8", "/tmp/test.mkv", "", "")

	hasAllowedExtensions := false
	hasExtensionPicky := false
	for i, a := range args {
		if a == "-allowed_extensions" && i+1 < len(args) && args[i+1] == "ALL" {
			hasAllowedExtensions = true
		}
		if a == "-extension_picky" && i+1 < len(args) && args[i+1] == "0" {
			hasExtensionPicky = true
		}
	}
	if !hasAllowedExtensions {
		t.Error("missing -allowed_extensions ALL for m3u8 URL")
	}
	if !hasExtensionPicky {
		t.Error("missing -extension_picky 0 for m3u8 URL")
	}
}

func TestArgsOmitsHLSFlagsForNonM3U8(t *testing.T) {
	r := newTestRunner(t)
	args := r.Args("https://example.com/video.mp4", "/tmp/test.mkv", "", "")

	for _, a := range args {
		if a == "-allowed_extensions" {
			t.Error("found -allowed_extensions for non-m3u8 URL")
		}
		if a == "-extension_picky" {
			t.Error("found -extension_picky for non-m3u8 URL")
		}
	}
}

func TestArgsContainsProtocolWhitelistForFileURL(t *testing.T) {
	r := newTestRunner(t)
	args := r.Args("file:///tmp/local.m3u8", "/tmp/test.mkv", "", "")

	hasProtocolWhitelist := false
	for i, a := range args {
		if a == "-protocol_whitelist" {
			hasProtocolWhitelist = true
			if !strings.Contains(args[i+1], "file") || !strings.Contains(args[i+1], "https") {
				t.Errorf("protocol_whitelist = %q, want file+https", args[i+1])
			}
		}
	}
	if !hasProtocolWhitelist {
		t.Error("missing -protocol_whitelist for file:// m3u8 URL")
	}
}

func TestProgressArgsContainsProgressFlags(t *testing.T) {
	r := newTestRunner(t)
	args := r.ProgressArgs("https://example.com/stream.m3u8", "/tmp/test.mkv", "", "")

	hasProgress := false
	hasStatsPeriod := false
	for i, a := range args {
		if a == "-progress" && i+1 < len(args) && args[i+1] == "pipe:2" {
			hasProgress = true
		}
		if a == "-stats_period" && i+1 < len(args) && args[i+1] == "1" {
			hasStatsPeriod = true
		}
	}
	if !hasProgress {
		t.Error("missing -progress pipe:2 in ProgressArgs")
	}
	if !hasStatsPeriod {
		t.Error("missing -stats_period 1 in ProgressArgs")
	}
}

func TestProgressArgsContainsSameCoreFlagsAsArgs(t *testing.T) {
	r := newTestRunner(t)
	args := r.Args("https://example.com/stream.m3u8", "/tmp/test.mkv", "", "")
	progArgs := r.ProgressArgs("https://example.com/stream.m3u8", "/tmp/test.mkv", "", "")

	// Both should contain -c copy, -i, -err_detect, -fflags
	coreFlags := []string{"-c", "-i", "-err_detect", "-fflags"}
	for _, flag := range coreFlags {
		foundInArgs := false
		foundInProgArgs := false
		for _, a := range args {
			if a == flag {
				foundInArgs = true
			}
		}
		for _, a := range progArgs {
			if a == flag {
				foundInProgArgs = true
			}
		}
		if !foundInArgs {
			t.Errorf("Args() missing %s", flag)
		}
		if !foundInProgArgs {
			t.Errorf("ProgressArgs() missing %s", flag)
		}
	}
}

func TestIsHLS(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://cdn.example.com/master.m3u8", true},
		{"https://cdn.example.com/stream.M3U8", true},
		{"https://proxy.example.com/proxy?url=https://cdn/example.m3u8", true},
		{"https://example.com/video.mp4", false},
		{"https://example.com/segment.ts", false},
		{"https://proxy.example.com/proxy?url=https://cdn/example.ts", false},
	}
	for _, tt := range tests {
		got := isHLS(tt.url)
		if got != tt.want {
			t.Errorf("isHLS(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s      string
		n      int
		expect string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"short", 3, "sho"},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.expect {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.expect)
		}
	}
}

func TestParseProgressOutput(t *testing.T) {
	input := `out_time_ms=5000000
total_size=1024000
speed=2.5x
bitrate=3400.0kbits/s
progress=continue
out_time_ms=10000000
total_size=2048000
speed=3.0x
progress=continue
`
	var updates []Progress
	parseProgressOutput(strings.NewReader(input), func(p Progress) {
		updates = append(updates, p)
	})

	if len(updates) != 2 {
		t.Fatalf("len(updates) = %d, want 2", len(updates))
	}
	if updates[0].TimeMs != 5000000 {
		t.Errorf("updates[0].TimeMs = %d, want 5000000", updates[0].TimeMs)
	}
	if updates[0].Bytes != 1024000 {
		t.Errorf("updates[0].Bytes = %d, want 1024000", updates[0].Bytes)
	}
	if updates[0].Speed != "2.5x" {
		t.Errorf("updates[0].Speed = %q, want 2.5x", updates[0].Speed)
	}
	if updates[1].TimeMs != 10000000 {
		t.Errorf("updates[1].TimeMs = %d, want 10000000", updates[1].TimeMs)
	}
}

func TestParseProgressOutputWithDiags(t *testing.T) {
	input := `out_time_ms=5000000
some error message
progress=continue
progress=end
`
	var updates []Progress
	diags := parseProgressOutputWithDiags(strings.NewReader(input), func(p Progress) {
		updates = append(updates, p)
	})

	if len(updates) != 1 {
		t.Fatalf("len(updates) = %d, want 1", len(updates))
	}
	if updates[0].TimeMs != 5000000 {
		t.Errorf("TimeMs = %d, want 5000000", updates[0].TimeMs)
	}
	// Diagnostics should capture the error message and progress=end.
	foundErrorMsg := false
	foundEnd := false
	for _, d := range diags {
		if strings.Contains(d, "error message") {
			foundErrorMsg = true
		}
		if d == "progress=end" {
			foundEnd = true
		}
	}
	if !foundErrorMsg {
		t.Errorf("diags = %v, want error message", diags)
	}
	if !foundEnd {
		t.Errorf("diags = %v, want progress=end", diags)
	}
}

func TestParseKV(t *testing.T) {
	tests := []struct {
		line  string
		key   string
		value string
		ok    bool
	}{
		{"out_time_ms=5000000", "out_time_ms", "5000000", true},
		{"no_equals_sign", "", "", false},
		{"=value_only", "", "value_only", true},
		{"key=", "key", "", true},
	}
	for _, tt := range tests {
		key, value, ok := parseKV(tt.line)
		if ok != tt.ok || key != tt.key || value != tt.value {
			t.Errorf("parseKV(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.line, key, value, ok, tt.key, tt.value, tt.ok)
		}
	}
}

func TestExecuteFailsForMissingBinary(t *testing.T) {
	_, err := NewRunner(Config{FFmpegPath: "/nonexistent/ffmpeg/binary"})
	if err == nil {
		t.Fatal("NewRunner() error = nil for missing binary")
	}
}
