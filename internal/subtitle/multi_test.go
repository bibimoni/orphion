package subtitle

import (
	"context"
	"errors"
	"testing"
)

// mockProvider is a simple test provider.
type mockProvider struct {
	searchResults []Result
	pageResult    *PageResult
	downloadURL   string
	searchErr     error
	pageErr       error
}

func (m *mockProvider) Search(_ context.Context, _ string) ([]Result, error) {
	return m.searchResults, m.searchErr
}

func (m *mockProvider) Page(_ context.Context, _, _, _ string) (*PageResult, error) {
	return m.pageResult, m.pageErr
}

func (m *mockProvider) DownloadURL(_ Subtitle) string {
	return m.downloadURL
}

func TestMultiProviderSearch(t *testing.T) {
	p1 := &mockProvider{
		searchResults: []Result{
			{ID: "subdl:sd1", Title: "Steins;Gate", Source: "subdl"},
		},
	}
	p2 := &mockProvider{
		searchResults: []Result{
			{ID: "kitsunekko:ja:Steins_Gate", Title: "Steins_Gate", Source: "kitsunekko"},
		},
	}

	mp := NewMultiProvider(map[string]Provider{
		"subdl":      p1,
		"kitsunekko": p2,
	})

	results, err := mp.Search(context.Background(), "Steins;Gate")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Check that both sources are present (order is non-deterministic).
	sources := map[string]bool{}
	for _, r := range results {
		sources[r.Source] = true
	}
	if !sources["subdl"] || !sources["kitsunekko"] {
		t.Errorf("expected sources subdl and kitsunekko, got %v", sources)
	}
}

func TestMultiProviderSearchWithFailure(t *testing.T) {
	p1 := &mockProvider{
		searchResults: []Result{
			{ID: "subdl:sd1", Title: "Naruto", Source: "subdl"},
		},
	}
	p2 := &mockProvider{
		searchErr: errors.New("network error"),
	}

	mp := NewMultiProvider(map[string]Provider{
		"subdl":      p1,
		"kitsunekko": p2,
	})

	results, err := mp.Search(context.Background(), "Naruto")
	if err != nil {
		t.Fatal(err)
	}
	// Should still return results from the working provider.
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMultiProviderPage(t *testing.T) {
	p1 := &mockProvider{
		pageResult: &PageResult{Subtitles: []Subtitle{
			{ID: 1, Title: "sub.srt", Source: "subdl"},
		}},
	}
	p2 := &mockProvider{
		pageResult: &PageResult{Subtitles: []Subtitle{
			{ID: 1, Title: "kit.srt", Source: "kitsunekko"},
		}},
	}

	mp := NewMultiProvider(map[string]Provider{
		"subdl":      p1,
		"kitsunekko": p2,
	})

	// With source prefix in ID, should route to subdl.
	page, err := mp.Page(context.Background(), "subdl:sd1", "slug", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Subtitles) != 1 || page.Subtitles[0].Source != "subdl" {
		t.Errorf("expected subdl page, got %v", page.Subtitles)
	}
}

func TestMultiProviderDownloadURL(t *testing.T) {
	p1 := &mockProvider{downloadURL: "https://subdl.com/file.zip"}
	p2 := &mockProvider{downloadURL: "https://kitsunekko.net/file.zip"}

	mp := NewMultiProvider(map[string]Provider{
		"subdl":      p1,
		"kitsunekko": p2,
	})

	// Route to subdl.
	url := mp.DownloadURL(Subtitle{Source: "subdl"})
	if url != "https://subdl.com/file.zip" {
		t.Errorf("DownloadURL(subdl) = %q", url)
	}

	// Route to kitsunekko.
	url = mp.DownloadURL(Subtitle{Source: "kitsunekko"})
	if url != "https://kitsunekko.net/file.zip" {
		t.Errorf("DownloadURL(kitsunekko) = %q", url)
	}
}

func TestSplitSourcePrefix(t *testing.T) {
	mp := NewMultiProvider(map[string]Provider{
		"subdl":      &mockProvider{},
		"kitsunekko": &mockProvider{},
	})

	tests := []struct {
		id       string
		wantSrc  string
		wantRest string
	}{
		{"subdl:sd123", "subdl", "sd123"},
		{"kitsunekko:ja:Steins_Gate", "kitsunekko", "ja:Steins_Gate"},
		{"sd123", "", "sd123"},
		{"a:sd123", "", "a:sd123"},         // unknown prefix
		{"movie:sd123", "", "movie:sd123"}, // not a known provider
	}

	for _, tt := range tests {
		src, rest := mp.splitSourcePrefix(tt.id)
		if src != tt.wantSrc {
			t.Errorf("splitSourcePrefix(%q) src = %q, want %q", tt.id, src, tt.wantSrc)
		}
		if rest != tt.wantRest {
			t.Errorf("splitSourcePrefix(%q) rest = %q, want %q", tt.id, rest, tt.wantRest)
		}
	}
}
