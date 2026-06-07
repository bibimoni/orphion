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

// SetDefaults populates missing fields with defaults.
// Only fills fields that are not explicitly set from YAML.
func (c *Config) SetDefaults() {
	if c.OutputDir == "" {
		c.OutputDir = "~/Anime"
	}
	if c.PreferredQuality == "" {
		c.PreferredQuality = "1080p"
	}
	if !c.concurrencyExplicit && c.Concurrency == 0 {
		c.Concurrency = 1
	}
	if c.Provider == "" {
		c.Provider = "catalog"
	}
	if c.FFmpegPath == "" {
		c.FFmpegPath = "ffmpeg"
	}
}

func (c *Config) validate() error {
	if c.Concurrency < 1 || c.Concurrency > 4 {
		return fmt.Errorf("concurrency must be between 1 and 4, got %d", c.Concurrency)
	}
	return nil
}

func validateConcurrency(n int) error {
	if n < 1 || n > 4 {
		return fmt.Errorf("concurrency must be 1-4, got %d", n)
	}
	return nil
}

// ErrConfigExists is returned when a configuration file already exists.
var ErrConfigExists = fmt.Errorf("configuration file already exists")

// raw is used for YAML decoding without default application side-effects.
type raw struct {
	OutputDir        *string `yaml:"output_dir"`
	PreferredQuality *string `yaml:"preferred_quality"`
	Concurrency      *int    `yaml:"concurrency"`
	Provider         *string `yaml:"provider"`
	FFmpegPath       *string `yaml:"ffmpeg_path"`
}

// Load reads and validates a configuration YAML file. Missing files are
// silently treated as empty configuration. Unknown fields are rejected.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.SetDefaults()
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// First pass: strict decode into raw to detect unknown fields
	rawCfg := &raw{}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(rawCfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}

	// Map raw -> Config, tracking which fields were explicitly set.
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
	}
	cfg.SetDefaults()
	if rawCfg.Provider != nil {
		cfg.Provider = *rawCfg.Provider
	} else {
		cfg.Provider = "catalog"
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

// Init creates a default configuration file at the given path. It creates
// parent directories as needed and refuses to overwrite an existing file.
func Init(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrConfigExists, path)
	}

	cfg := Config{}
	SetRawDefaults(&cfg)
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

// SetRawDefaults fills the config with raw defaults suitable for marshaling.
func SetRawDefaults(cfg *Config) {
	cfg.OutputDir = "~/Anime"
	cfg.PreferredQuality = "1080p"
	cfg.Concurrency = 1
	cfg.Provider = "catalog"
	cfg.FFmpegPath = "ffmpeg"
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