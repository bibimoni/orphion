package app

import (
	"context"
	"errors"
	"testing"

	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/provider"
	"github.com/distiled/orphion/internal/subtitle"
)

func TestService_ProviderNamesWithActiveProviderFirst(t *testing.T) {
	allanime := &fakeProvider{}
	bettermelon := &fakeProvider{}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(allanime, r, Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "allanime",
		Providers: map[string]provider.Provider{
			"allanime":    allanime,
			"bettermelon": bettermelon,
		},
	})

	names := svc.ProviderNames()
	if len(names) != 2 {
		t.Fatalf("ProviderNames() = %v, want 2 names", names)
	}
	// Active provider should be first.
	if names[0] != "allanime" {
		t.Fatalf("ProviderNames()[0] = %q, want allanime (active)", names[0])
	}
}

func TestService_ProviderNamesOrderedCorrectly(t *testing.T) {
	custom := &fakeProvider{}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(custom, r, Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "allanime",
		Providers: map[string]provider.Provider{
			"allanime":    custom,
			"bettermelon": custom,
			"custom":      custom,
		},
	})

	names := svc.ProviderNames()
	// Should be: active first, then known providers in order, then others.
	if names[0] != "allanime" {
		t.Fatalf("first name = %q, want allanime", names[0])
	}
}

func TestService_SetConcurrencyBounds(t *testing.T) {
	svc := newTestService(&fakeProvider{})

	// Below minimum → clamped to 1.
	svc.SetConcurrency(0)
	if svc.Config().Concurrency != 1 {
		t.Errorf("SetConcurrency(0) = %d, want 1", svc.Config().Concurrency)
	}
	svc.SetConcurrency(-1)
	if svc.Config().Concurrency != 1 {
		t.Errorf("SetConcurrency(-1) = %d, want 1", svc.Config().Concurrency)
	}

	// Above maximum → clamped to 4.
	svc.SetConcurrency(10)
	if svc.Config().Concurrency != 4 {
		t.Errorf("SetConcurrency(10) = %d, want 4", svc.Config().Concurrency)
	}

	// Valid value.
	svc.SetConcurrency(2)
	if svc.Config().Concurrency != 2 {
		t.Errorf("SetConcurrency(2) = %d, want 2", svc.Config().Concurrency)
	}
}

func TestService_SetForce(t *testing.T) {
	svc := newTestService(&fakeProvider{})

	if svc.Config().Force {
		t.Error("Force should default to false")
	}
	svc.SetForce(true)
	if !svc.Config().Force {
		t.Error("SetForce(true) should make Force true")
	}
	svc.SetForce(false)
	if svc.Config().Force {
		t.Error("SetForce(false) should make Force false")
	}
}

func TestService_SetOutputDir(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	svc.SetOutputDir("/tmp/test-anime")
	if svc.Config().OutputDir != "/tmp/test-anime" {
		t.Errorf("OutputDir = %q, want /tmp/test-anime", svc.Config().OutputDir)
	}
}

func TestService_SetPreferredQuality(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	svc.SetPreferredQuality("720p")
	if svc.Config().PreferredQty != "720p" {
		t.Errorf("PreferredQty = %q, want 720p", svc.Config().PreferredQty)
	}
}

func TestService_SubtitleProviderReturnsNilWhenNotConfigured(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	if svc.SubtitleProvider() != nil {
		t.Error("SubtitleProvider() should return nil when not configured")
	}
}

func TestService_SubtitleProviderReturnsProviderWhenConfigured(t *testing.T) {
	subProv := &fakeSubtitleProvider{}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(&fakeProvider{}, r, Config{
		Concurrency: 1,
		SubtitleSrc: subProv,
	})
	if svc.SubtitleProvider() == nil {
		t.Error("SubtitleProvider() should return non-nil when configured")
	}
}

func TestService_SubtitleLangDefaultsToEnglish(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	if svc.SubtitleLang() != common.DefaultSubtitleLang {
		t.Errorf("SubtitleLang() = %q, want %q", svc.SubtitleLang(), common.DefaultSubtitleLang)
	}
}

func TestService_SubtitleLangCanBeOverridden(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	svc.SetSubtitleLang("japanese")
	if svc.SubtitleLang() != "japanese" {
		t.Errorf("SubtitleLang() = %q, want japanese", svc.SubtitleLang())
	}
}

func TestService_DownloadEpisodesNoEpisodesError(t *testing.T) {
	fp := &fakeProvider{
		eps: []provider.Episode{},
	}
	svc := newTestService(fp)
	_, _, err := svc.DownloadEpisodes(context.Background(), "anime-id", "1", "Test")
	if err == nil {
		t.Fatal("expected error for no matching episodes")
	}
}

func TestService_DownloadEpisodesProviderError(t *testing.T) {
	fp := &fakeProvider{err: errors.New("provider error")}
	svc := newTestService(fp)
	_, _, err := svc.DownloadEpisodes(context.Background(), "anime-id", "1", "Test")
	if err == nil {
		t.Fatal("expected error from provider")
	}
}

func TestService_ResolveIDNoResults(t *testing.T) {
	fp := &fakeProvider{searches: nil}
	svc := newTestService(fp)
	_, err := svc.ResolveID(context.Background(), "nonexistent", "anime")
	if err == nil {
		t.Fatal("expected error for no results")
	}
}

func TestService_ResolveIDMultipleResults(t *testing.T) {
	fp := &fakeProvider{
		searches: []provider.Anime{
			{ID: "a", Title: "A"},
			{ID: "b", Title: "B"},
		},
	}
	svc := newTestService(fp)
	_, err := svc.ResolveID(context.Background(), "ambiguous", "anime")
	if err == nil {
		t.Fatal("expected error for multiple results")
	}
}

func TestService_OutputDirExpandsTilde(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	svc.SetOutputDir("~/Anime")
	// OutputDir() should expand ~ using paths.ExpandTilde.
	expanded := svc.OutputDir()
	if expanded == "~/Anime" {
		t.Errorf("OutputDir() = %q, should expand tilde", expanded)
	}
}

func TestService_SetProviderUpdatesProviderName(t *testing.T) {
	first := &fakeProvider{searches: []provider.Anime{{ID: "1", Title: "First"}}}
	second := &fakeProvider{searches: []provider.Anime{{ID: "2", Title: "Second"}}}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(first, r, Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "first",
		Providers: map[string]provider.Provider{
			"first":  first,
			"second": second,
		},
	})

	if svc.ProviderName() != "first" {
		t.Errorf("ProviderName() = %q, want first", svc.ProviderName())
	}
	if err := svc.SetProvider("second"); err != nil {
		t.Fatal(err)
	}
	if svc.ProviderName() != "second" {
		t.Errorf("ProviderName() after switch = %q, want second", svc.ProviderName())
	}
}

func TestService_SearchSubtitlesNoProviderError(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	_, err := svc.SearchSubtitles(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error when subtitle provider not configured")
	}
}

func TestService_DownloadSubtitleNoProviderError(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	_, err := svc.DownloadSubtitle(context.Background(), subtitle.Subtitle{ID: 1}, "/tmp")
	if err == nil {
		t.Fatal("expected error when subtitle provider not configured")
	}
}

func TestService_GetEpisodes(t *testing.T) {
	fp := &fakeProvider{
		eps: []provider.Episode{
			{ID: "e1", Number: "1", SortKey: 1.0},
			{ID: "e2", Number: "2", SortKey: 2.0},
		},
	}
	svc := newTestService(fp)
	eps, err := svc.GetEpisodes(context.Background(), "anime-id")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 {
		t.Errorf("len(GetEpisodes()) = %d, want 2", len(eps))
	}
}

func TestService_SubtitlePageNoProviderError(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	_, err := svc.SubtitlePage(context.Background(), "sd1", "naruto", "")
	if err == nil {
		t.Fatal("expected error when subtitle provider not configured")
	}
}
