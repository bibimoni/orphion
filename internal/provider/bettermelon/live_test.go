package bettermelon

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveProviderFrieren(t *testing.T) {
	if os.Getenv("ORPHION_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("set ORPHION_LIVE_PROVIDER_TEST=1 to contact the live provider")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "154587", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("live search returned no results")
	}
	if results[0].Title == "" {
		t.Fatal("live search returned empty title")
	}

	episodes, err := client.Episodes(ctx, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) == 0 {
		t.Fatal("live episode lookup returned no episodes")
	}

	streams, err := client.Streams(ctx, episodes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) == 0 {
		t.Fatal("live stream lookup returned no streams")
	}
	t.Logf("stream URL (redacted): length=%d, quality=%q", len(streams[0].URL), streams[0].Quality)
}

// TestSmokeAPI verifies that the Bettermelon search and episodes APIs are
// reachable. It does NOT test stream downloads (which 403 from CI IPs).
// Triggered by ORPHION_SMOKE_ANIME_ID env var (set by CI).
func TestSmokeAPI(t *testing.T) {
	animeID := os.Getenv("ORPHION_SMOKE_ANIME_ID")
	if animeID == "" {
		t.Skip("set ORPHION_SMOKE_ANIME_ID to run smoke test")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	episodes, err := client.Episodes(ctx, animeID)
	if err != nil {
		t.Fatalf("episodes API unreachable: %v", err)
	}
	if len(episodes) == 0 {
		t.Fatal("episodes API returned zero episodes")
	}
	t.Logf("OK: %d episodes for anime ID %s", len(episodes), animeID)
}
