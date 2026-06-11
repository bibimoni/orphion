//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitCreatesFile(t *testing.T) {
	home := tempHomeDir(t)

	output, err := runOrphion(t, home, "config", "init")
	if err != nil {
		t.Fatalf("orphion config init: %v\noutput: %s", err, output)
	}

	cfgFile := configPath(home)
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Errorf("config file not created at %s", cfgFile)
	}
}

func TestConfigInitContainsDefaults(t *testing.T) {
	home := tempHomeDir(t)

	_, err := runOrphion(t, home, "config", "init")
	if err != nil {
		t.Fatalf("orphion config init: %v", err)
	}

	cfgFile := configPath(home)
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)

	// Verify all expected default keys are present.
	expectedKeys := []string{
		"output_dir:",
		"preferred_quality:",
		"concurrency:",
		"provider:",
		"ffmpeg_path:",
		"subtitle_lang:",
	}
	for _, key := range expectedKeys {
		if !strings.Contains(content, key) {
			t.Errorf("config missing key %q, got:\n%s", key, content)
		}
	}

	// Verify default values from common constants.
	expectedValues := []string{
		"1080p",    // preferred_quality
		"allanime", // provider
		"ffmpeg",   // ffmpeg_path
		"english",  // subtitle_lang
	}
	for _, val := range expectedValues {
		if !strings.Contains(content, val) {
			t.Errorf("config missing default value %q, got:\n%s", val, content)
		}
	}
}

func TestConfigInitCreatesDirectoryStructure(t *testing.T) {
	home := tempHomeDir(t)

	_, err := runOrphion(t, home, "config", "init")
	if err != nil {
		t.Fatalf("orphion config init: %v", err)
	}

	cfgDir := configDir(home)
	info, err := os.Stat(cfgDir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", cfgDir)
	}
}

func TestConfigInitIdempotentFailsOnSecondRun(t *testing.T) {
	home := tempHomeDir(t)

	// First run should succeed.
	output, err := runOrphion(t, home, "config", "init")
	if err != nil {
		t.Fatalf("first config init: %v\noutput: %s", err, output)
	}

	// Verify the config file exists.
	cfgFile := configPath(home)
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("config file not created after first init: %v", err)
	}

	// Second run should fail with "already exists" error.
	output, err = runOrphion(t, home, "config", "init")
	if err == nil {
		t.Errorf("second config init should fail, but succeeded; output: %s", output)
	}

	if !strings.Contains(output, "already exists") {
		t.Errorf("second config init should report 'already exists', got: %q", output)
	}
}

func TestConfigInitDoesNotModifyExistingConfig(t *testing.T) {
	home := tempHomeDir(t)
	cfgDir := configDir(home)

	// Create a custom config file first.
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	customContent := "output_dir: /custom/path\npreferred_quality: 720p\nconcurrency: 2\nprovider: bettermelon\nffmpeg_path: ffmpeg\nsubtitle_lang: japanese\n"
	cfgFile := configPath(home)
	if err := os.WriteFile(cfgFile, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Running config init should fail and NOT overwrite.
	output, _ := runOrphion(t, home, "config", "init")
	if !strings.Contains(output, "already exists") {
		t.Errorf("expected 'already exists' error, got: %q", output)
	}

	// Verify original content is preserved.
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != customContent {
		t.Errorf("existing config was modified:\ngot:  %q\nwant: %q", string(data), customContent)
	}
}

func TestConfigPathConsistency(t *testing.T) {
	home := tempHomeDir(t)

	// After running config init, the path reported in the output
	// should match the actual file location.
	output, err := runOrphion(t, home, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v", err)
	}

	expectedPath := filepath.Join(home, ".config", "orphion", "config.yaml")
	if !strings.Contains(output, expectedPath) {
		t.Errorf("output should mention path %q, got: %q", expectedPath, output)
	}
}
