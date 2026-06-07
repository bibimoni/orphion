package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/cli"
	"github.com/distiled/orphion/internal/config"
	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/provider/allanime"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

	cfgPath := defaultConfigPath()

	// Check if this command can run without a config file.
	// Commands like "config init" and "version" don't need one.
	if !configNeeded(os.Args[1:]) {
		runWithoutConfig(ctx, cfgPath)
		return
	}

	// Load configuration. The config file is required — all runtime
	// values come from the file, not from hardcoded defaults.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigRequired) {
			fmt.Fprintln(os.Stderr, "orphion: configuration file not found.")
			fmt.Fprintf(os.Stderr, "Run `orphion config init` to create one at %s\n", cfgPath)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "orphion: config:", err)
		os.Exit(2)
	}

	contentProvider, err := allanime.NewProvider(allanime.DefaultConfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, "orphion: provider:", err)
		os.Exit(2)
	}

	runner, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: cfg.FFmpegPath})
	if err != nil {
		fmt.Fprintln(os.Stderr, "orphion:", err)
		os.Exit(2)
	}

	appCfg := app.Config{
		OutputDir:    cfg.OutputDir,
		Concurrency:  cfg.Concurrency,
		PreferredQty: cfg.PreferredQuality,
	}
	service := app.New(contentProvider, runner, appCfg)

	root := cli.New(service)
	root.SetContext(ctx)

	if err := root.Execute(); err != nil {
		handleError(ctx, err)
	}
}

// runWithoutConfig runs commands that don't require a config file
// (config init, version) using a nil service.
func runWithoutConfig(ctx context.Context, cfgPath string) {
	root := cli.New(nil)
	root.SetContext(ctx)

	// Override the config init path to use the resolved default.
	cli.SetConfigInitPath(cfgPath)

	if err := root.Execute(); err != nil {
		handleError(ctx, err)
	}
}

// configNeeded returns true if the given args require a config file.
// Commands like "config init" and "version" can run without one.
func configNeeded(args []string) bool {
	if len(args) == 0 {
		// Root command (interactive mode) needs config.
		return false // Will be caught by service being nil
	}

	switch args[0] {
	case "config":
		// "config init" doesn't need config, but "config" alone is invalid.
		return false
	case "version":
		return false
	case "help":
		return false
	default:
		return true
	}
}

func handleError(ctx context.Context, err error) {
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "orphion:", err)
		os.Exit(130)
	}
	if e, ok := err.(*cli.ExitError); ok {
		fmt.Fprintln(os.Stderr, "orphion:", err)
		os.Exit(e.Code())
	}
	fmt.Fprintln(os.Stderr, "orphion:", err)
	os.Exit(classifyError(err))
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "orphion", "config.yaml")
}

func classifyError(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "usage") || strings.Contains(msg, "invalid") || strings.Contains(msg, "required") || strings.Contains(msg, "not configured") || strings.Contains(msg, "config"):
		return 2
	case strings.Contains(msg, "not found") || strings.Contains(msg, "no results") || strings.Contains(msg, "ambiguous") || strings.Contains(msg, "provider"):
		return 3
	default:
		return 1
	}
}
