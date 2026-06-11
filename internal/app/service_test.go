package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/bibimoni/orphion/internal/ffmpeg"
	"github.com/bibimoni/orphion/internal/paths"
	"github.com/bibimoni/orphion/internal/provider"
	"github.com/bibimoni/orphion/internal/subtitle"
)

type fakeProvider struct {
	searches []provider.Anime
	eps      []provider.Episode
	streams  []provider.Stream
	err      error
	queries  []string
}

func (p *fakeProvider) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	if p.err != nil {
		return nil, p.err
	}
	p.queries = append(p.queries, query+"/"+kind)
	return p.searches, nil
}

func (p *fakeProvider) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.eps, nil
}

func (p *fakeProvider) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.streams, nil
}

func newTestService(fp *fakeProvider) *Service {
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	return New(fp, r, Config{Concurrency: 1, PreferredQty: "1080p"})
}

func TestService_Search(t *testing.T) {
	fp := &fakeProvider{
		searches: []provider.Anime{
			{ID: "test", Title: "Frieren"},
		},
	}
	svc := newTestService(fp)
	result, err := svc.Search(context.Background(), "frieren", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Anime) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Anime))
	}
}

func TestService_SetProviderSwitchesSearchProvider(t *testing.T) {
	first := &fakeProvider{
		searches: []provider.Anime{{ID: "first", Title: "First"}},
	}
	second := &fakeProvider{
		searches: []provider.Anime{{ID: "second", Title: "Second"}},
	}
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

	if err := svc.SetProvider("second"); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Search(context.Background(), "query", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Anime[0].ID; got != "second" {
		t.Fatalf("Search() used provider result %q, want second", got)
	}
	if len(first.queries) != 0 {
		t.Fatalf("first provider was searched after switch: %v", first.queries)
	}
	if got := second.queries; len(got) != 1 || got[0] != "query/" {
		t.Fatalf("second provider queries = %v, want [query/]", got)
	}
	if got := svc.ProviderName(); got != "second" {
		t.Fatalf("ProviderName() = %q, want second", got)
	}
}

func TestService_SetProviderRejectsUnknownProvider(t *testing.T) {
	fp := &fakeProvider{}
	svc := newTestService(fp)

	err := svc.SetProvider("missing")
	if err == nil {
		t.Fatal("SetProvider() error = nil, want unknown provider error")
	}
}

func TestService_ResolveID(t *testing.T) {
	fp := &fakeProvider{
		searches: []provider.Anime{
			{ID: "ep1", Title: "Frieren"},
		},
	}
	svc := newTestService(fp)
	id, err := svc.ResolveID(context.Background(), "frieren", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if id != "ep1" {
		t.Errorf("ID = %q, want %q", id, "ep1")
	}

	// Multiple results should error.
	fp.searches = []provider.Anime{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "B"},
	}
	_, err = svc.ResolveID(context.Background(), "test", "anime")
	if err == nil {
		t.Fatal("expected error for multiple results")
	}

	// Zero results should error.
	fp.searches = nil
	_, err = svc.ResolveID(context.Background(), "test", "anime")
	if err == nil {
		t.Fatal("expected error for no results")
	}
}

func TestService_DownloadEpisodes(t *testing.T) {
	fp := &fakeProvider{
		eps: []provider.Episode{
			{ID: "e1", Number: "1", SortKey: 1.0},
			{ID: "e2", Number: "2", SortKey: 2.0},
		},
		streams: []provider.Stream{
			{URL: "https://example.com/1080.m3u8", Quality: "1080p"},
		},
	}
	svc := newTestService(fp)
	ctx := context.Background()

	result, raw, err := svc.DownloadEpisodes(ctx, "anime-id", "1-2", "Test Title")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2 {
		t.Errorf("expected 2 results, got %d", len(raw))
	}
	_ = result
}

func TestService_DownloadSelectedEpisodesUsesProvidedEpisodeIDs(t *testing.T) {
	tmp := t.TempDir()
	title := "Selected Title"
	fp := &fakeProvider{
		streams: []provider.Stream{
			{URL: "https://example.com/1080.m3u8", Quality: "1080p"},
		},
	}
	svc := newTestService(fp)
	svc.SetOutputDir(tmp)

	selected := []provider.Episode{
		{ID: "source-ep-1", Number: "1", Title: "Source episode 1"},
		{ID: "source-special", Number: "SP", Title: "Special source"},
	}
	for _, ep := range selected {
		outPath := paths.OutputLayout(tmp, title, ep.Number)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(outPath, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, raw, err := svc.DownloadSelectedEpisodes(context.Background(), selected, title)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 2 {
		t.Fatalf("Completed = %d, want 2", result.Completed)
	}
	if len(raw) != 2 {
		t.Fatalf("raw results = %d, want 2", len(raw))
	}
	if raw[0].JobID != "source-ep-1" || raw[1].JobID != "source-special" {
		t.Fatalf("job IDs = %q, %q; want selected source IDs", raw[0].JobID, raw[1].JobID)
	}
}

func TestService_ErrorPropagation(t *testing.T) {
	fp := &fakeProvider{err: errors.New("provider error")}
	svc := newTestService(fp)

	_, err := svc.Search(context.Background(), "test", "anime")
	if err == nil {
		t.Fatal("expected error from provider")
	}
}

// fakeSubtitleProvider implements subtitle.Provider for testing.
type fakeSubtitleProvider struct {
	results []subtitle.Result
	page    *subtitle.PageResult
	dlURL   string
}

func (f *fakeSubtitleProvider) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	return f.results, nil
}

func (f *fakeSubtitleProvider) Page(ctx context.Context, sdID, slug, seasonSlug string) (*subtitle.PageResult, error) {
	return f.page, nil
}

func (f *fakeSubtitleProvider) DownloadURL(sub subtitle.Subtitle) string {
	return f.dlURL
}

func TestService_SubtitleProvider(t *testing.T) {
	subProv := &fakeSubtitleProvider{
		results: []subtitle.Result{
			{ID: "sd1", Title: "Naruto", Type: "tv", Year: 2002, Slug: "naruto"},
		},
	}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(&fakeProvider{}, r, Config{
		Concurrency: 1,
		SubtitleSrc: subProv,
	})

	if svc.SubtitleProvider() == nil {
		t.Error("SubtitleProvider() = nil, want non-nil")
	}
}

func TestService_SubtitleLang(t *testing.T) {
	fp := &fakeProvider{}
	svc := newTestService(fp)

	// Default should be "english".
	if got := svc.SubtitleLang(); got != "english" {
		t.Errorf("SubtitleLang() = %q, want %q", got, "english")
	}

	svc.SetSubtitleLang("arabic")
	if got := svc.SubtitleLang(); got != "arabic" {
		t.Errorf("SubtitleLang() after Set = %q, want %q", got, "arabic")
	}
}

func TestService_SearchSubtitles(t *testing.T) {
	subProv := &fakeSubtitleProvider{
		results: []subtitle.Result{
			{ID: "sd1", Title: "Naruto", Type: "tv", Slug: "naruto"},
		},
	}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(&fakeProvider{}, r, Config{
		Concurrency: 1,
		SubtitleSrc: subProv,
	})

	results, err := svc.SearchSubtitles(context.Background(), "naruto")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Naruto" {
		t.Errorf("SearchSubtitles() = %#v", results)
	}
}

func TestService_SearchSubtitlesNoProvider(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	_, err := svc.SearchSubtitles(context.Background(), "test")
	if err == nil {
		t.Error("expected error when subtitle provider not configured")
	}
}

func TestService_SubtitlePage(t *testing.T) {
	subProv := &fakeSubtitleProvider{
		page: &subtitle.PageResult{
			Seasons: []subtitle.Season{{Slug: "first-season", Name: "Season 1"}},
			Subtitles: []subtitle.Subtitle{
				{ID: 1, Language: "english", Link: "test.zip"},
			},
		},
	}
	r, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := New(&fakeProvider{}, r, Config{
		Concurrency: 1,
		SubtitleSrc: subProv,
	})

	page, err := svc.SubtitlePage(context.Background(), "sd1", "naruto", "first-season")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Seasons) != 1 {
		t.Errorf("Seasons = %d, want 1", len(page.Seasons))
	}
	if len(page.Subtitles) != 1 {
		t.Errorf("Subtitles = %d, want 1", len(page.Subtitles))
	}
}

func TestService_DownloadSubtitleNoProvider(t *testing.T) {
	svc := newTestService(&fakeProvider{})
	_, err := svc.DownloadSubtitle(context.Background(), subtitle.Subtitle{ID: 1}, "/tmp")
	if err == nil {
		t.Error("expected error when subtitle provider not configured")
	}
}
