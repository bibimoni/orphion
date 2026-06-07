package app

import (
	"context"
	"errors"
	"testing"

	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/provider"
)

type fakeProvider struct {
	searches []provider.Anime
	eps      []provider.Episode
	streams  []provider.Stream
	err      error
}

func (p *fakeProvider) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	if p.err != nil {
		return nil, p.err
	}
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

func TestService_ErrorPropagation(t *testing.T) {
	fp := &fakeProvider{err: errors.New("provider error")}
	svc := newTestService(fp)

	_, err := svc.Search(context.Background(), "test", "anime")
	if err == nil {
		t.Fatal("expected error from provider")
	}
}