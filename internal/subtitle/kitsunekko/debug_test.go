package kitsunekko

import (
	"context"
	"testing"
	"time"

	"github.com/distiled/orphion/internal/subtitle"
)

func TestDebugSearch(t *testing.T) {
	cfg := DefaultConfig()
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := p.Search(ctx, "Dagashi Kashi")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	t.Logf("Search returned %d results", len(results))

	// Rank them like the CLI would.
	ranked := subtitle.RankResults("Dagashi Kashi", results, 20, 0.2)
	t.Logf("Ranked (top 20, minScore 0.15): %d results", len(ranked))
	for i, r := range ranked {
		t.Logf("  %d: %q (id=%s, source=%s, slug=%s)", i, r.Title, r.ID, r.Source, r.Slug)
	}

	// Auto-match.
	idx, match := subtitle.BestMatch("Dagashi Kashi", ranked)
	if idx < 0 {
		t.Log("BestMatch: no auto-match found")
	} else {
		t.Logf("BestMatch: %q (slug=%s)", match.Title, match.Slug)
	}
}

func TestDebugSearchSteinsGate(t *testing.T) {
	cfg := DefaultConfig()
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := p.Search(ctx, "Steins;Gate")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	t.Logf("Search returned %d results", len(results))
	ranked := subtitle.RankResults("Steins;Gate", results, 20, 0.2)
	t.Logf("Ranked: %d results", len(ranked))
	for i, r := range ranked {
		t.Logf("  %d: %q (slug=%s)", i, r.Title, r.Slug)
	}

	// Try fetching the page for the top match.
	if len(ranked) > 0 {
		top := ranked[0]
		page, err := p.Page(ctx, top.ID, top.Slug, "")
		if err != nil {
			t.Logf("Page for %q: error: %v", top.Title, err)
		} else {
			t.Logf("Page for %q: %d subtitles", top.Title, len(page.Subtitles))
			for i, s := range page.Subtitles {
				if i > 5 {
					break
				}
				t.Logf("  sub: %q (%s)", s.Title, s.Language)
			}
		}
	}
}
