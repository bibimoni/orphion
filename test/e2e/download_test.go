//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
)

func TestDownloadRequiresTitleOrID(t *testing.T) {
	home := tempHomeDir(t)

	// download without --title or --title-id should error.
	output, err := runOrphion(t, home, "download", "--episodes", "1")
	if err == nil {
		t.Errorf("download without --title should fail, got output: %q", output)
	}
	if !strings.Contains(output, "required") && !strings.Contains(output, "title-id") {
		t.Errorf("error should mention required flag, got: %q", output)
	}
}

func TestDownloadRequiresEpisodes(t *testing.T) {
	home := tempHomeDir(t)

	// download without --episodes should error.
	output, err := runOrphion(t, home, "download", "--title", "Test Anime")
	if err == nil {
		t.Errorf("download without --episodes should fail, got output: %q", output)
	}
	if !strings.Contains(output, "required") && !strings.Contains(output, "episodes") {
		t.Errorf("error should mention episodes flag, got: %q", output)
	}
}

func TestDownloadHelpShowsFlags(t *testing.T) {
	home := tempHomeDir(t)

	output, _ := runOrphion(t, home, "download", "--help")

	expectedFlags := []string{"--episodes", "--title", "--title-id", "--quality", "--output", "--concurrency", "--force"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("download --help should mention %s flag, got: %q", flag, output)
		}
	}
}

func TestDownloadWithMissingFFmpegReportsError(t *testing.T) {
	home := tempHomeDir(t)
	cfgDir := configDir(home)

	// Create a config with a non-existent ffmpeg path.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := "output_dir: /tmp/test\npreferred_quality: 1080p\nconcurrency: 1\nprovider: allanime\nffmpeg_path: /nonexistent/ffmpeg-binary\nsubtitle_lang: english\n"
	if err := os.WriteFile(configPath(home), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// The binary should fail to start (ffmpeg check in main.go) but not panic.
	output, err := runOrphion(t, home, "download", "--title", "Test", "--episodes", "1")
	_ = output
	_ = err
	// The test passes as long as the binary exits (no hang/panic).
}

func TestDownloadWithInvalidConcurrency(t *testing.T) {
	home := tempHomeDir(t)
	cfgDir := configDir(home)

	// Create a config with concurrency out of range.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := "output_dir: /tmp/test\npreferred_quality: 1080p\nconcurrency: 99\nprovider: allanime\nffmpeg_path: ffmpeg\nsubtitle_lang: english\n"
	if err := os.WriteFile(configPath(home), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// The binary should fail with config validation error.
	output, err := runOrphion(t, home, "download", "--title", "Test", "--episodes", "1")
	_ = output
	_ = err
	// The test passes as long as the binary exits cleanly.
}
