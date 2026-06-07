package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	SetDefaults(cfg)
	if cfg.OutputDir != "~/Anime" {
		t.Errorf("default output_dir = %q, want %q", cfg.OutputDir, "~/Anime")
	}
	if cfg.PreferredQuality != "1080p" {
		t.Errorf("default preferred_quality = %q, want %q", cfg.PreferredQuality, "1080p")
	}
	if cfg.Concurrency != 1 {
		t.Errorf("default concurrency = %d, want 1", cfg.Concurrency)
	}
	if cfg.Provider != "allanime" {
		t.Errorf("default provider = %q, want %q", cfg.Provider, "allanime")
	}
	if cfg.FFmpegPath != "ffmpeg" {
		t.Errorf("default ffmpeg_path = %q, want %q", cfg.FFmpegPath, "ffmpeg")
	}
}

func TestSetDefaultsDoesNotOverwrite(t *testing.T) {
	cfg := &Config{
		OutputDir:        "/custom/path",
		PreferredQuality: "720p",
		Concurrency:      3,
		Provider:         "custom",
		FFmpegPath:       "/usr/local/bin/ffmpeg",
	}
	SetDefaults(cfg)
	if cfg.OutputDir != "/custom/path" {
		t.Errorf("OutputDir overwritten: got %q", cfg.OutputDir)
	}
	if cfg.PreferredQuality != "720p" {
		t.Errorf("PreferredQuality overwritten: got %q", cfg.PreferredQuality)
	}
	if cfg.Concurrency != 3 {
		t.Errorf("Concurrency overwritten: got %d", cfg.Concurrency)
	}
	if cfg.Provider != "custom" {
		t.Errorf("Provider overwritten: got %q", cfg.Provider)
	}
	if cfg.FFmpegPath != "/usr/local/bin/ffmpeg" {
		t.Errorf("FFmpegPath overwritten: got %q", cfg.FFmpegPath)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandTilde("~/Anime")
	want := filepath.Join(home, "Anime")
	if got != want {
		t.Errorf("expandTilde(%q) = %q, want %q", "~/Anime", got, want)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadOrCreateReturnsDefaultsWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if cfg.OutputDir != "~/Anime" {
		t.Errorf("OutputDir = %q, want ~/Anime", cfg.OutputDir)
	}
	// File should NOT be created on disk — only "config init" writes it.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config file should not be auto-created, but it exists")
	}
}

func TestLoadOrCreateLoadsExisting(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := "output_dir: /custom/path\npreferred_quality: 720p"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if cfg.OutputDir != "/custom/path" {
		t.Errorf("OutputDir = %q, want /custom/path", cfg.OutputDir)
	}
	if cfg.PreferredQuality != "720p" {
		t.Errorf("PreferredQuality = %q, want 720p", cfg.PreferredQuality)
	}
}

func TestLoadConfigUnknownKey(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := "unknown_field: xyz\noutput_dir: /tmp/test"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestLoadConfigRejectsZeroConcurrency(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := "output_dir: /tmp/test\nconcurrency: 0"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for zero concurrency, got nil")
	}
}

func TestLoadConfigRejectsInvalidConcurrency(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := "output_dir: /tmp/test\nconcurrency: 5"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for concurrency=5, got nil")
	}
}

func TestLoadConfigTildeExpanded(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := "output_dir: ~/Anime"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "Anime")
	if cfg.OutputDir != expected {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, expected)
	}
}

func TestLoadConfigAppliesDefaultsForOmittedFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	// Only specify output_dir; all other fields should get defaults.
	content := "output_dir: /tmp/test"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OutputDir != "/tmp/test" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "/tmp/test")
	}
	if cfg.PreferredQuality != "1080p" {
		t.Errorf("PreferredQuality = %q, want %q", cfg.PreferredQuality, "1080p")
	}
	if cfg.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1", cfg.Concurrency)
	}
	if cfg.Provider != "allanime" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "allanime")
	}
	if cfg.FFmpegPath != "ffmpeg" {
		t.Errorf("FFmpegPath = %q, want %q", cfg.FFmpegPath, "ffmpeg")
	}
}

func TestLoadConfigRejectsOverwrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	content := "output_dir: /tmp/test"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Init(path)
	if err == nil {
		t.Fatal("expected error when config already exists, got nil")
	}
}

func TestConfigInit(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	err := Init(path)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !contains(got, "output_dir") {
		t.Error("init config missing output_dir")
	}
	if !contains(got, "ffmpeg_path") {
		t.Error("init config missing ffmpeg_path")
	}
}

func TestConcurrencyValidation(t *testing.T) {
	tests := []struct {
		val int
		ok  bool
	}{
		{0, false},
		{1, true},
		{4, true},
		{5, false},
	}
	for _, tt := range tests {
		err := validateConcurrency(tt.val)
		if (err == nil) != tt.ok {
			t.Errorf("validateConcurrency(%d) = %v, want ok=%v", tt.val, err, tt.ok)
		}
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
