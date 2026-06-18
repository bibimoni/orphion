package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bibimoni/orphion/internal/download"
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

type fallbackPreparer struct {
	*fakeProvider
	failures map[string]error
	attempts []string
}

func (p *fallbackPreparer) PrepareStream(
	ctx context.Context,
	stream provider.Stream,
	progress provider.SegmentProgressFunc,
) (provider.Stream, error) {
	p.attempts = append(p.attempts, stream.URL)
	if err := p.failures[stream.URL]; err != nil {
		return provider.Stream{}, err
	}
	return stream, nil
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

func newTestService(t *testing.T, fp *fakeProvider) *Service {
	t.Helper()
	r, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
	return New(fp, r, Config{Concurrency: 1, PreferredQty: "1080p"})
}

func TestService_Search(t *testing.T) {
	fp := &fakeProvider{
		searches: []provider.Anime{
			{ID: "test", Title: "Frieren"},
		},
	}
	svc := newTestService(t, fp)
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
	r, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
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
	svc := newTestService(t, fp)

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
	svc := newTestService(t, fp)
	id, err := svc.ResolveID(context.Background(), "frieren", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if id != "ep1" {
		t.Errorf("ID = %q, want %q", id, "ep1")
	}

	// Multiple results with an exact title match resolve to that match.
	fp.searches = []provider.Anime{
		{ID: "a", Title: "Sentenced to Be a Hero"},
		{ID: "b", Title: "Sentenced to Be a Hero Season 2"},
	}
	id, err = svc.ResolveID(context.Background(), "Sentenced to Be a Hero", "anime")
	if err != nil {
		t.Fatalf("expected exact match, got error: %v", err)
	}
	if id != "a" {
		t.Errorf("ID = %q, want %q for exact match", id, "a")
	}

	// Exact match is case- and punctuation-insensitive.
	fp.searches = []provider.Anime{
		{ID: "a", Title: "Sentenced: to Be a Hero!"},
		{ID: "b", Title: "Sentenced to Be a Hero Season 2"},
	}
	id, err = svc.ResolveID(context.Background(), "sentenced to be a hero", "anime")
	if err != nil {
		t.Fatalf("expected normalized match, got error: %v", err)
	}
	if id != "a" {
		t.Errorf("ID = %q, want %q for normalized match", id, "a")
	}

	// Multiple results with no exact match should error.
	fp.searches = []provider.Anime{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "B"},
	}
	_, err = svc.ResolveID(context.Background(), "test", "anime")
	if err == nil {
		t.Fatal("expected error for multiple results with no exact match")
	}

	// Multiple results with more than one exact match are ambiguous.
	fp.searches = []provider.Anime{
		{ID: "a", Title: "A"},
		{ID: "b", Title: "A"},
	}
	_, err = svc.ResolveID(context.Background(), "A", "anime")
	if err == nil {
		t.Fatal("expected error for duplicate exact matches")
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
	svc := newTestService(t, fp)
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

func TestServiceFallsBackWhenPreferredStreamPreparationFails(t *testing.T) {
	p := &fallbackPreparer{
		fakeProvider: &fakeProvider{
			streams: []provider.Stream{
				{URL: "https://example.com/high.m3u8", Quality: "1080p", Bandwidth: 7800000},
				{URL: "https://example.com/fallback.m3u8", Quality: "1080p", Bandwidth: 1700000},
			},
		},
		failures: map[string]error{
			"https://example.com/high.m3u8": errors.New("segment status 403"),
		},
	}
	svc := New(p, nil, Config{
		OutputDir:    t.TempDir(),
		Concurrency:  1,
		PreferredQty: "1080p",
	})

	_, err := svc.executeJob(context.Background(), download.Job{
		ID:      "episode-1",
		Episode: "1",
		Title:   "Test",
	})
	if err == nil || !strings.Contains(err.Error(), "runner not available") {
		t.Fatalf("executeJob() error = %v, want runner unavailable after fallback preparation", err)
	}
	wantAttempts := []string{
		"https://example.com/high.m3u8",
		"https://example.com/fallback.m3u8",
	}
	if !reflect.DeepEqual(p.attempts, wantAttempts) {
		t.Fatalf("preparation attempts = %v, want %v", p.attempts, wantAttempts)
	}
}

func TestService_DownloadSelectedEpisodesUsesProvidedEpisodeIDs(t *testing.T) {
	tmp := t.TempDir()
	title := "Selected Title"
	fp := &fakeProvider{
		streams: []provider.Stream{
			{URL: "https://example.com/1080.m3u8", Quality: "1080p"},
		},
	}
	svc := newTestService(t, fp)
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
	svc := newTestService(t, fp)

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
	r, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
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
	svc := newTestService(t, fp)

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
	r, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
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
	svc := newTestService(t, &fakeProvider{})
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
	r, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
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
	svc := newTestService(t, &fakeProvider{})
	_, err := svc.DownloadSubtitle(context.Background(), subtitle.Subtitle{ID: 1}, "/tmp")
	if err == nil {
		t.Error("expected error when subtitle provider not configured")
	}
}
