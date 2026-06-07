package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunnerArgs(t *testing.T) {
	r, err := NewRunner(Config{FFmpegPath: "/usr/bin/ffmpeg"})
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
	for _, a := range args {
		if a == "-c" {
			hasCopy = true
		}
		if a == "-nostdin" {
			hasStdin = true
		}
	}
	if !hasCopy || !hasStdin {
		t.Error("missing critical args")
	}
}

func TestEnsureDir(t *testing.T) {
	dir := t.TempDir()
	r, _ := NewRunner(Config{FFmpegPath: "/usr/bin/ffmpeg"})
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
	r, _ := NewRunner(Config{FFmpegPath: "/usr/bin/ffmpeg"})

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
	r, _ := NewRunner(Config{FFmpegPath: "/usr/bin/ffmpeg"})
	p := filepath.Join(dir, "ep.part.mkv")
	if err := os.WriteFile(p, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.CleanupPartial(p)
	if _, err := os.Stat(p); err == nil {
		t.Fatal("partial file still exists after cleanup")
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
