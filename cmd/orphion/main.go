package main

import (
	"context"
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

	// Load or auto-create configuration. On first run, a default config
	// file is written so the user can discover and edit it later.
	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
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
