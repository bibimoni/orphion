// Package config handles YAML configuration for Orphion.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bibimoni/orphion/internal/common"
	"github.com/bibimoni/orphion/internal/paths"
)

// Config holds Orphion configuration.
type Config struct {
	OutputDir        string `yaml:"output_dir"`
	PreferredQuality string `yaml:"preferred_quality"`
	Concurrency      int    `yaml:"concurrency"`
	Provider         string `yaml:"provider"`
	FFmpegPath       string `yaml:"ffmpeg_path"`
	SubtitleLang     string `yaml:"subtitle_lang"`
	SegmentWorkers   int    `yaml:"segment_workers"`
}

// ErrConfigExists is returned when a configuration file already exists.
var ErrConfigExists = fmt.Errorf("configuration file already exists")

// SetDefaults fills missing fields with the canonical defaults.
// This is the single source of truth for all default values.
func SetDefaults(cfg *Config) {
	if cfg.OutputDir == "" {
		cfg.OutputDir = common.DefaultOutputDir
	}
	if cfg.PreferredQuality == "" {
		cfg.PreferredQuality = common.DefaultQuality
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = common.DefaultConcurrency
	}
	if cfg.Provider == "" {
		cfg.Provider = common.DefaultProvider
	}
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = common.DefaultFFmpegPath
	}
	if cfg.SubtitleLang == "" {
		cfg.SubtitleLang = common.DefaultSubtitleLang
	}
	if cfg.SegmentWorkers == 0 {
		cfg.SegmentWorkers = common.DefaultSegmentWorkers
	}
}

// raw is used for YAML decoding to track which fields were explicitly set.
type raw struct {
	OutputDir        *string `yaml:"output_dir"`
	PreferredQuality *string `yaml:"preferred_quality"`
	Concurrency      *int    `yaml:"concurrency"`
	Provider         *string `yaml:"provider"`
	FFmpegPath       *string `yaml:"ffmpeg_path"`
	SubtitleLang     *string `yaml:"subtitle_lang"`
	SegmentWorkers   *int    `yaml:"segment_workers"`
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

// LoadOrCreate loads the config file if it exists. If it doesn't, it returns
// in-memory defaults without writing to disk. Use "config init" to explicitly
// create the file.
func LoadOrCreate(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return decode(data, path)
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// File doesn't exist — return defaults in-memory without writing to disk.
	cfg := &Config{}
	SetDefaults(cfg)
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
		cfg.OutputDir = paths.ExpandTilde(*rawCfg.OutputDir)
	} else {
		cfg.OutputDir = common.DefaultOutputDir
	}
	if rawCfg.PreferredQuality != nil {
		cfg.PreferredQuality = *rawCfg.PreferredQuality
	} else {
		cfg.PreferredQuality = common.DefaultQuality
	}
	if rawCfg.Concurrency != nil {
		cfg.Concurrency = *rawCfg.Concurrency
	} else {
		cfg.Concurrency = common.DefaultConcurrency
	}
	if rawCfg.Provider != nil {
		cfg.Provider = *rawCfg.Provider
	} else {
		cfg.Provider = common.DefaultProvider
	}
	if rawCfg.FFmpegPath != nil {
		cfg.FFmpegPath = *rawCfg.FFmpegPath
	} else {
		cfg.FFmpegPath = common.DefaultFFmpegPath
	}
	if rawCfg.SubtitleLang != nil {
		cfg.SubtitleLang = *rawCfg.SubtitleLang
	} else {
		cfg.SubtitleLang = common.DefaultSubtitleLang
	}
	if rawCfg.SegmentWorkers != nil {
		cfg.SegmentWorkers = *rawCfg.SegmentWorkers
	} else {
		cfg.SegmentWorkers = common.DefaultSegmentWorkers
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
	if c.SegmentWorkers < 1 || c.SegmentWorkers > common.MaxSegmentWorkers {
		return fmt.Errorf("segment_workers must be between 1 and %d, got %d", common.MaxSegmentWorkers, c.SegmentWorkers)
	}
	if c.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if c.FFmpegPath == "" {
		return fmt.Errorf("ffmpeg_path is required")
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
