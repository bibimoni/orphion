//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestSubtitlesCommandExists(t *testing.T) {
	home := tempHomeDir(t)

	output, _ := runOrphion(t, home, "subtitles", "--help")
	if !strings.Contains(output, "Search and download subtitles") && !strings.Contains(output, "Usage") {
		t.Errorf("subtitles --help should show usage, got: %q", output)
	}
}

func TestSubtitlesHelpShowsFlags(t *testing.T) {
	home := tempHomeDir(t)

	output, _ := runOrphion(t, home, "subtitles", "--help")

	expectedFlags := []string{"--lang", "--output"}
	for _, flag := range expectedFlags {
		if !strings.Contains(output, flag) {
			t.Errorf("subtitles --help should mention %s flag, got: %q", flag, output)
		}
	}
}

func TestSubtitlesMaxOneArgument(t *testing.T) {
	home := tempHomeDir(t)

	// subtitles accepts MaximumNArgs(1). Providing 2+ args should error.
	output, err := runOrphion(t, home, "subtitles", "arg1", "arg2")
	if err == nil {
		t.Errorf("subtitles with 2 args should fail, got output: %q", output)
	}
}

func TestSubtitlesRequiresNoArgsNoHang(t *testing.T) {
	home := tempHomeDir(t)

	// subtitles with no args enters the interactive flow which requires a TTY.
	// With stdin closed and TERM=dumb, the binary should exit (not hang).
	// This tests that the binary doesn't deadlock on interactive prompts.
	output, err := runOrphion(t, home, "subtitles")
	_ = output
	_ = err
	// The test passes as long as the binary exits within the timeout (no hang/panic).
}
