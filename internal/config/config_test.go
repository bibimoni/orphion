package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.OutputDir != "~/Anime" {
		t.Errorf("default output_dir = %q, want %q", cfg.OutputDir, "~/Anime")
	}
	if cfg.PreferredQuality != "1080p" {
		t.Errorf("default preferred_quality = %q, want %q", cfg.PreferredQuality, "1080p")
	}
	if cfg.Concurrency != 1 {
		t.Errorf("default concurrency = %d, want 1", cfg.Concurrency)
	}
	if cfg.Provider != "catalog" {
		t.Errorf("default provider = %q, want %q", cfg.Provider, "catalog")
	}
	if cfg.FFmpegPath != "ffmpeg" {
		t.Errorf("default ffmpeg_path = %q, want %q", cfg.FFmpegPath, "ffmpeg")
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := expandTilde("~/Anime")
	want := filepath.Join(home, "Anime")
	if got != want {
		t.Errorf("expandTilde(%q) = %q, want %q", "~/Anime", got, want)
	}

	got2 := expandTilde("/abs/path")
	if got2 != "/abs/path" {
		t.Errorf("expandTilde(%q) = %q, want %q", "/abs/path", got2, "/abs/path")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OutputDir != "~/Anime" {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, "~/Anime")
	}
	if cfg.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1", cfg.Concurrency)
	}
}

func TestLoadConfig_UnknownKey(t *testing.T) {
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

func TestLoadConfig_RejectsOverwrite(t *testing.T) {
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
	return len(s) >= len(sub) && (s[:len(sub)] == sub || containsAfter(s, sub))
}
func containsAfter(s, sub string) bool {
	for i := 1; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
