package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/bibimoni/orphion/internal/app"
	"github.com/bibimoni/orphion/internal/cli"
	"github.com/bibimoni/orphion/internal/config"
	"github.com/bibimoni/orphion/internal/ffmpeg"
	"github.com/bibimoni/orphion/internal/provider"
	"github.com/bibimoni/orphion/internal/provider/allanime"
	"github.com/bibimoni/orphion/internal/provider/bettermelon"
	"github.com/bibimoni/orphion/internal/subtitle"
	"github.com/bibimoni/orphion/internal/subtitle/jimaku"
	"github.com/bibimoni/orphion/internal/subtitle/kitsunekko"
	"github.com/bibimoni/orphion/internal/subtitle/subdl"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		shutdownSignals()...,
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

	providers, err := newProviders()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orphion: provider:", err)
		os.Exit(2)
	}
	providerName := normalizeProviderName(cfg.Provider)
	contentProvider, ok := providers[providerName]
	if !ok {
		fmt.Fprintf(os.Stderr, "orphion: provider: unknown provider %q, falling back to allanime\n", cfg.Provider)
		providerName = "allanime"
		contentProvider = providers[providerName]
	}

	runner, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: cfg.FFmpegPath})
	if err != nil {
		fmt.Fprintf(os.Stderr, "orphion: %v (download commands will be unavailable)\n", err)
	}

	// Initialize subtitle providers (non-fatal on error).
	subProviders := make(map[string]subtitle.Provider)
	if p, err := subdl.NewProvider(subdl.DefaultConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "orphion: subdl: %v\n", err)
	} else {
		subProviders["subdl"] = p
	}
	if p, err := kitsunekko.NewProvider(kitsunekko.DefaultConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "orphion: kitsunekko: %v\n", err)
	} else {
		subProviders["kitsunekko"] = p
	}
	if p, err := jimaku.NewProvider(jimaku.DefaultConfig()); err != nil {
		fmt.Fprintf(os.Stderr, "orphion: jimaku: %v\n", err)
	} else {
		subProviders["jimaku"] = p
	}

	var subtitleProvider subtitle.Provider
	switch len(subProviders) {
	case 0:
		fmt.Fprintln(os.Stderr, "orphion: no subtitle providers available (subtitles disabled)")
	case 1:
		for _, p := range subProviders {
			subtitleProvider = p
		}
	default:
		subtitleProvider = subtitle.NewMultiProvider(subProviders)
	}

	appCfg := app.Config{
		OutputDir:    cfg.OutputDir,
		Concurrency:  cfg.Concurrency,
		PreferredQty: cfg.PreferredQuality,
		ProviderName: providerName,
		Providers:    providers,
		SubtitleLang: cfg.SubtitleLang,
		SubtitleSrc:  subtitleProvider,
	}
	service := app.New(contentProvider, runner, appCfg)

	root := cli.New(service)
	root.SetContext(ctx)

	if err := root.Execute(); err != nil {
		handleError(ctx, err)
	}
}

func newProvider(name string) (provider.Provider, error) {
	switch name {
	case "allanime", "catalog":
		return allanime.NewProvider(allanime.DefaultConfig())
	case "bettermelon":
		return bettermelon.NewProvider(bettermelon.DefaultConfig())
	default:
		return nil, fmt.Errorf("unknown provider %q (available: allanime, bettermelon)", name)
	}
}

func newProviders() (map[string]provider.Provider, error) {
	allanimeProvider, err := newProvider("allanime")
	if err != nil {
		return nil, err
	}
	bettermelonProvider, err := newProvider("bettermelon")
	if err != nil {
		return nil, err
	}
	return map[string]provider.Provider{
		"allanime":    allanimeProvider,
		"bettermelon": bettermelonProvider,
	}, nil
}

func normalizeProviderName(name string) string {
	if name == "catalog" {
		return "allanime"
	}
	return name
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

func classifyError(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "usage") || strings.Contains(msg, "invalid") || strings.Contains(msg, "required") || strings.Contains(msg, "not configured") || strings.Contains(msg, "config") || strings.Contains(msg, "ffmpeg not found"):
		return 2
	case strings.Contains(msg, "not found") || strings.Contains(msg, "no results") || strings.Contains(msg, "ambiguous") || strings.Contains(msg, "provider") || strings.Contains(msg, "no streams") || strings.Contains(msg, "no episodes"):
		return 3
	default:
		return 1
	}
}
