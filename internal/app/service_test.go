package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/paths"
	"github.com/distiled/orphion/internal/provider"
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
