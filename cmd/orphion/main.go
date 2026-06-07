package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/cli"
	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/provider/catalog"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

	// Build the application service with catalog provider and FFmpeg runner.
	provider := catalog.NewProvider(catalog.Config{BaseURL: catalog.DefaultBaseURL})
	runner, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "orphion:", err)
		os.Exit(2)
	}

	cfg := app.Config{
		OutputDir:    "~/Anime",
		Concurrency:  1,
		PreferredQty: "1080p",
	}
	service := app.New(provider, runner, cfg)

	root := cli.New(service)
	root.SetContext(ctx)

	if err := root.Execute(); err != nil {
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
}

func classifyError(err error) int {
	msg := err.Error()
	switch {
	case contains(msg, "usage") || contains(msg, "invalid") || contains(msg, "required") || contains(msg, "not configured") || contains(msg, "config"):
		return 2
	case contains(msg, "not found") || contains(msg, "no results") || contains(msg, "ambiguous") || contains(msg, "provider"):
		return 3
	default:
		return 1
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}