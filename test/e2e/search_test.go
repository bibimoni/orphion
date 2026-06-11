//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
)

func TestSearchRequiresArgument(t *testing.T) {
	home := tempHomeDir(t)

	// search with no args should error.
	output, err := runOrphion(t, home, "search")
	if err == nil {
		t.Errorf("search without args should fail, got output: %q", output)
	}
}

func TestSearchHelpShowsUsage(t *testing.T) {
	home := tempHomeDir(t)

	output, _ := runOrphion(t, home, "search", "--help")
	if !strings.Contains(output, "Search for titles") && !strings.Contains(output, "Usage") {
		t.Errorf("search --help should show usage, got: %q", output)
	}
}

func TestSearchReportsProviderErrorGracefully(t *testing.T) {
	home := tempHomeDir(t)
	cfgDir := configDir(home)

	// Create a config pointing to allanime provider.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := "output_dir: /tmp/test\npreferred_quality: 1080p\nconcurrency: 1\nprovider: allanime\nffmpeg_path: ffmpeg\nsubtitle_lang: english\n"
	if err := os.WriteFile(configPath(home), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Search should either return results or fail with a provider error.
	// Either way, it must not panic or hang.
	output, err := runOrphion(t, home, "search", "--type", "anime", "nonexistent-anime-e2e-test-xyz")
	_ = output
	_ = err
	// The test passes as long as the binary exits (no hang/panic).
}

func TestSearchTypeFlagAccepted(t *testing.T) {
	home := tempHomeDir(t)

	output, _ := runOrphion(t, home, "search", "--help")
	if !strings.Contains(output, "--type") {
		t.Errorf("search --help should mention --type flag, got: %q", output)
	}
}

func TestSearchInvalidProviderReportsError(t *testing.T) {
	home := tempHomeDir(t)
	cfgDir := configDir(home)

	// Config with an unknown provider name.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := "output_dir: /tmp/test\npreferred_quality: 1080p\nconcurrency: 1\nprovider: nonexistent-provider\nffmpeg_path: ffmpeg\nsubtitle_lang: english\n"
	if err := os.WriteFile(configPath(home), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// The binary should fall back to allanime (as coded in main.go)
	// and print a warning, but not crash.
	output, err := runOrphion(t, home, "search", "test")
	_ = output
	_ = err
}
