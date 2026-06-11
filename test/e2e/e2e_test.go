//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	binaryPath string
	buildOnce  sync.Once
	buildErr   error
)

var emptyReader = strings.NewReader("")

// projectRoot returns the root directory of the Go module.
func projectRoot() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list -m: %w", err)
	}
	// Trim whitespace from output (includes trailing newline).
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("empty module directory")
	}
	return dir, nil
}

// buildOrphion builds the orphion binary once per test suite run.
// It returns the path to the built binary, or fails the test if the build fails.
func buildOrphion(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		root, err := projectRoot()
		if err != nil {
			buildErr = fmt.Errorf("find project root: %w", err)
			return
		}

		tmpDir, err := os.MkdirTemp("", "orphion-e2e-")
		if err != nil {
			buildErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		outputPath := filepath.Join(tmpDir, "orphion")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", outputPath, "./cmd/orphion")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build: %w\n%s", err, out)
			return
		}

		binaryPath = outputPath
	})

	if buildErr != nil {
		t.Fatalf("build orphion: %v", buildErr)
	}

	return binaryPath
}

// runOrphion executes the orphion binary with the given arguments and environment.
// It returns the combined output and error. The HOME and XDG_CONFIG_HOME
// environment variables are set to tempDir so config isolation is guaranteed.
func runOrphion(t *testing.T, tempDir string, args ...string) (string, error) {
	t.Helper()
	bin := buildOrphion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = emptyReader
	cmd.Env = []string{
		"HOME=" + tempDir,
		"XDG_CONFIG_HOME=" + filepath.Join(tempDir, ".config"),
		"XDG_CACHE_HOME=" + filepath.Join(tempDir, ".cache"),
		"PATH=" + os.Getenv("PATH"),
		"TERM=dumb",
		"NO_COLOR=1",
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// tempHomeDir creates a temporary directory to be used as HOME.
func tempHomeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "orphion-e2e-home-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// configDir returns the expected orphion config directory for a given home.
func configDir(home string) string {
	return filepath.Join(home, ".config", "orphion")
}

// configPath returns the expected orphion config file path for a given home.
func configPath(home string) string {
	return filepath.Join(configDir(home), "config.yaml")
}
