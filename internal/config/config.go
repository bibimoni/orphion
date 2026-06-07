// Package config handles YAML configuration for Orphion.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds Orphion configuration.
type Config struct {
	OutputDir        string `yaml:"output_dir"`
	PreferredQuality string `yaml:"preferred_quality"`
	Concurrency      int    `yaml:"concurrency"`
	Provider         string `yaml:"provider"`
	FFmpegPath       string `yaml:"ffmpeg_path"`

	concurrencyExplicit bool
}

// ErrConfigExists is returned when a configuration file already exists.
var ErrConfigExists = fmt.Errorf("configuration file already exists")

// SetDefaults fills missing fields with the canonical defaults.
// This is the single source of truth for all default values.
func SetDefaults(cfg *Config) {
	if cfg.OutputDir == "" {
		cfg.OutputDir = "~/Anime"
	}
	if cfg.PreferredQuality == "" {
		cfg.PreferredQuality = "1080p"
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 1
	}
	if cfg.Provider == "" {
		cfg.Provider = "allanime"
	}
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = "ffmpeg"
	}
}

// raw is used for YAML decoding to track which fields were explicitly set.
type raw struct {
	OutputDir        *string `yaml:"output_dir"`
	PreferredQuality *string `yaml:"preferred_quality"`
	Concurrency      *int    `yaml:"concurrency"`
	Provider         *string `yaml:"provider"`
	FFmpegPath       *string `yaml:"ffmpeg_path"`
}

// Load reads and validates a configuration YAML file.
// Returns an error if the file does not exist or is invalid.
// Unknown fields are rejected.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg, err := decode(data, path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadOrCreate loads the config file if it exists. If it doesn't, it creates
// one with defaults and returns it. This ensures the app works on first run
// without requiring the user to run "config init" manually.
func LoadOrCreate(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return decode(data, path)
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// File doesn't exist — create it with defaults.
	cfg := &Config{}
	SetDefaults(cfg)

	if err := writeConfigFile(path, cfg); err != nil {
		return nil, fmt.Errorf("create default config: %w", err)
	}

	return cfg, nil
}

func decode(data []byte, path string) (*Config, error) {
	// Strict decode to detect unknown fields.
	rawCfg := &raw{}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(rawCfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	// Map raw -> Config. Apply defaults only for fields omitted from YAML.
	cfg := &Config{}
	if rawCfg.OutputDir != nil {
		cfg.OutputDir = expandTilde(*rawCfg.OutputDir)
	} else {
		cfg.OutputDir = "~/Anime"
	}
	if rawCfg.PreferredQuality != nil {
		cfg.PreferredQuality = *rawCfg.PreferredQuality
	} else {
		cfg.PreferredQuality = "1080p"
	}
	if rawCfg.Concurrency != nil {
		cfg.Concurrency = *rawCfg.Concurrency
		cfg.concurrencyExplicit = true
	} else {
		cfg.Concurrency = 1
	}
	if rawCfg.Provider != nil {
		cfg.Provider = *rawCfg.Provider
	} else {
		cfg.Provider = "allanime"
	}
	if rawCfg.FFmpegPath != nil {
		cfg.FFmpegPath = *rawCfg.FFmpegPath
	} else {
		cfg.FFmpegPath = "ffmpeg"
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Concurrency < 1 || c.Concurrency > 4 {
		return fmt.Errorf("concurrency must be between 1 and 4, got %d", c.Concurrency)
	}
	if c.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if c.FFmpegPath == "" {
		return fmt.Errorf("ffmpeg_path is required")
	}
	return nil
}

func validateConcurrency(n int) error {
	if n < 1 || n > 4 {
		return fmt.Errorf("concurrency must be 1-4, got %d", n)
	}
	return nil
}

// Init creates a default configuration file at the given path. It creates
// parent directories as needed and refuses to overwrite an existing file.
func Init(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrConfigExists, path)
	}

	cfg := &Config{}
	SetDefaults(cfg)
	return writeConfigFile(path, cfg)
}

// writeConfigFile marshals and writes the config to disk.
func writeConfigFile(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(p string) string {
	h, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return h
	}
	if len(p) > 1 && p[:2] == "~/" {
		return filepath.Join(h, p[2:])
	}
	return p
}
