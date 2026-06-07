package cli

import (
	"context"
	"testing"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/provider"
)

type interactiveFakeProvider struct {
	results []provider.Anime
	kinds   []string
}

func (p *interactiveFakeProvider) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	p.kinds = append(p.kinds, kind)
	return p.results, nil
}

func (p *interactiveFakeProvider) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	return nil, nil
}

func (p *interactiveFakeProvider) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	return nil, nil
}

func TestSelectInteractiveProviderSwitchesProviderWithoutImplyingType(t *testing.T) {
	originalSelect := interactiveSelect
	t.Cleanup(func() { interactiveSelect = originalSelect })
	interactiveSelect = func(options []string, defaultText string) (string, error) {
		if defaultText != "Select provider:" {
			t.Fatalf("default text = %q, want Select provider:", defaultText)
		}
		return "nyaa", nil
	}

	allanimeProvider := &interactiveFakeProvider{
		results: []provider.Anime{{ID: "allanime", Title: "AllAnime"}},
	}
	nyaaProvider := &interactiveFakeProvider{
		results: []provider.Anime{{ID: "nyaa", Title: "Nyaa"}},
	}
	runner, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	service := app.New(allanimeProvider, runner, app.Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "allanime",
		Providers: map[string]provider.Provider{
			"allanime": allanimeProvider,
			"nyaa":     nyaaProvider,
		},
	})

	if err := selectInteractiveProvider(service); err != nil {
		t.Fatal(err)
	}
	if got := service.ProviderName(); got != "nyaa" {
		t.Fatalf("ProviderName() = %q, want nyaa", got)
	}
	if _, err := service.Search(context.Background(), "逃げ恥", ""); err != nil {
		t.Fatal(err)
	}
	if got := nyaaProvider.kinds; len(got) != 1 || got[0] != "" {
		t.Fatalf("Nyaa search kinds = %v, want empty kind", got)
	}
	if len(allanimeProvider.kinds) != 0 {
		t.Fatalf("AllAnime was searched: %v", allanimeProvider.kinds)
	}
}
