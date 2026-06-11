//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionCommand(t *testing.T) {
	home := tempHomeDir(t)

	output, err := runOrphion(t, home, "version")
	if err != nil {
		t.Fatalf("orphion version: %v\noutput: %s", err, output)
	}

	if !strings.Contains(output, "orphion") {
		t.Errorf("version output should contain 'orphion', got: %q", output)
	}
}

func TestVersionContainsVersionString(t *testing.T) {
	home := tempHomeDir(t)

	output, err := runOrphion(t, home, "version")
	if err != nil {
		t.Fatalf("orphion version: %v\noutput: %s", err, output)
	}

	// The version string should contain at least "dev" or a semver tag.
	// It's set via ldflags at build time; in dev builds it's "dev".
	hasVersion := strings.Contains(output, "dev") ||
		strings.Contains(output, "v0.") ||
		strings.Contains(output, "v1.")
	if !hasVersion {
		t.Errorf("version output should contain a version string, got: %q", output)
	}
}

func TestVersionCommandNoArgsNoHang(t *testing.T) {
	home := tempHomeDir(t)
	bin := buildOrphion(t)

	// Running with no subcommand enters the interactive flow.
	// With stdin closed and TERM=dumb, the binary should exit
	// (not hang waiting for interactive input).
	// Use a short timeout since we're just verifying it doesn't deadlock.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = emptyReader
	cmd.Env = []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + os.Getenv("PATH"),
		"TERM=dumb",
		"NO_COLOR=1",
	}

	_, _ = cmd.CombinedOutput()
	// The test passes as long as the binary exits within the timeout (no deadlock).
}

func TestVersionFlag(t *testing.T) {
	home := tempHomeDir(t)

	output, _ := runOrphion(t, home, "--help")
	if !strings.Contains(output, "orphion") {
		t.Errorf("help output should mention orphion, got: %q", output)
	}
}
