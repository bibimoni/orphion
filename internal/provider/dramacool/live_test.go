package dramacool

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveProviderDramaSearch(t *testing.T) {
	if os.Getenv("ORPHION_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("set ORPHION_LIVE_PROVIDER_TEST=1 to contact the live provider")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "crash landing on you", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("live search returned no results")
	}
	t.Logf("Search results: %d", len(results))
	for i, r := range results {
		t.Logf("  [%d] %s (%s)", i, r.Title, r.ID)
	}

	episodes, err := client.Episodes(ctx, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) == 0 {
		t.Fatal("live episode lookup returned no episodes")
	}
	t.Logf("Episodes: %d", len(episodes))
	for i, ep := range episodes {
		if i > 5 {
			t.Logf("  ... and %d more", len(episodes)-5)
			break
		}
		t.Logf("  [%d] %s (ID: %s)", i, ep.Number, ep.ID)
	}

	streams, err := client.Streams(ctx, episodes[0].ID)
	if err != nil {
		t.Logf("Stream lookup failed (expected with Cloudflare): %v", err)
		return
	}
	if len(streams) == 0 {
		t.Fatal("live stream lookup returned no streams")
	}
	t.Logf("Streams: %d", len(streams))
	for i, s := range streams {
		t.Logf("  [%d] %s (Quality: %q)", i, s.URL, s.Quality)
	}
}
