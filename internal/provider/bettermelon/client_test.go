package bettermelon

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.APIURL = "https://api.example.test"
	cfg.ProxyURL = "https://api.example.test" // same host so mock intercepts proxy calls too
	cfg.HTTPClient = &http.Client{Transport: transport}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestSearchWithNumericIDReturnsAnime(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s", req.Method)
		}
		if req.URL.Path != "/anime/154587/1/hianime" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"provider": "hianime",
				"anime": {
					"id": 154587,
					"title": {"english": "Frieren: Beyond Journey's End", "romaji": "Sousou no Frieren", "native": "葬送のフリーレン"},
					"format": "TV",
					"status": "FINISHED"
				},
				"episode": {
					"details": {"id": "353471", "attributes": {"number": 1, "canonicalTitle": "The Journey's End"}},
					"sources": {"type": "SOFT", "sources": {"file": "https://cdn.example.test/master.m3u8"}}
				}
			}
		}`), nil
	})

	got, err := client.Search(context.Background(), "154587", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Search()) = %d", len(got))
	}
	if got[0].ID != "154587" {
		t.Fatalf("ID = %q", got[0].ID)
	}
	if got[0].Title != "Frieren: Beyond Journey's End" {
		t.Fatalf("Title = %q", got[0].Title)
	}
}

func TestSearchFallsBackToRomajiTitle(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"provider": "hianime",
				"anime": {"id": 1, "title": {"english": "", "romaji": "Sousou no Frieren", "native": "日本語"}},
				"episode": {"details": {"id": "1", "attributes": {"number": 1}}, "sources": {"sources": {"file": "https://cdn.example.test/master.m3u8"}}}
			}
		}`), nil
	})

	got, err := client.Search(context.Background(), "1", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "Sousou no Frieren" {
		t.Fatalf("Title = %q", got[0].Title)
	}
}

func TestSearchWithTextQueryResolvesViaAniList(t *testing.T) {
	callCount := 0
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		callCount++
		// First call should be to AniList.
		if callCount == 1 {
			if req.Method != http.MethodPost {
				t.Fatalf("anilist method = %s", req.Method)
			}
			if req.URL.Host != "graphql.anilist.co" {
				t.Fatalf("anilist host = %s", req.URL.Host)
			}
			return jsonResponse(http.StatusOK, `{
				"data": {
					"Page": {
						"media": [
							{"id": 21365, "title": {"english": "", "romaji": "Dagashi Kashi", "native": "だがしかし"}},
							{"id": 99734, "title": {"english": "", "romaji": "Dagashi Kashi 2", "native": "だがしかし 2"}}
						]
					}
				}
			}`), nil
		}
		t.Fatalf("unexpected request %d to %s", callCount, req.URL.String())
		return nil, nil
	})

	got, err := client.Search(context.Background(), "dagashi", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Search()) = %d, want 2", len(got))
	}
	if got[0].ID != "21365" {
		t.Fatalf("ID = %q", got[0].ID)
	}
	if got[0].Title != "Dagashi Kashi" {
		t.Fatalf("Title = %q", got[0].Title)
	}
	if got[1].ID != "99734" {
		t.Fatalf("ID[1] = %q", got[1].ID)
	}
}

func TestSearchWithTextQueryNoResults(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"data": {
				"Page": {
					"media": []
				}
			}
		}`), nil
	})

	_, err := client.Search(context.Background(), "xyznonexistent", "anime")
	if err == nil {
		t.Fatal("Search() error = nil for no results")
	}
	if !strings.Contains(err.Error(), "no results") {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchWithTextQueryAniListError(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"errors": [{"message": "not found"}]
		}`), nil
	})

	_, err := client.Search(context.Background(), "something", "anime")
	if err == nil {
		t.Fatal("Search() error = nil for anilist errors")
	}
	if !strings.Contains(err.Error(), "anilist") {
		t.Fatalf("error = %v", err)
	}
}

func TestEpisodesFetchesAndSortsEpisodes(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/anime/154587/episodes" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"episodes": [
					{"id": "ep-2", "type": "episodes", "attributes": {"number": 2, "canonicalTitle": "Episode Two"}},
					{"id": "ep-1", "type": "episodes", "attributes": {"number": 1, "canonicalTitle": "Episode One"}},
					{"id": "ep-12.5", "type": "episodes", "attributes": {"number": 12.5, "canonicalTitle": "OVA"}}
				]
			}
		}`), nil
	})

	got, err := client.Episodes(context.Background(), "154587")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len(Episodes()) = %d", len(got))
	}
	if got[0].Number != "1" || got[1].Number != "2" || got[2].Number != "12.5" {
		t.Fatalf("Episodes() order = %v", got)
	}
	if got[0].Title != "Episode One" {
		t.Fatalf("Title = %q", got[0].Title)
	}
	// Verify episode IDs are opaque (no raw provider data leaked).
	if strings.Contains(got[0].ID, "154587") || strings.Contains(got[0].ID, `"`) {
		t.Fatalf("episode ID exposes provider structure: %q", got[0].ID)
	}
}

func TestEpisodesWithMissingNumberAreSkipped(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"episodes": [
					{"id": "ep-1", "type": "episodes", "attributes": {"number": 1}},
					{"id": "ep-bad", "type": "episodes", "attributes": {}}
				]
			}
		}`), nil
	})

	got, err := client.Episodes(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Episodes()) = %d, want 1", len(got))
	}
}

func TestEpisodesDefaultTitleWhenCanonicalEmpty(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"episodes": [
					{"id": "ep-1", "type": "episodes", "attributes": {"number": 1}}
				]
			}
		}`), nil
	})

	got, err := client.Episodes(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "Episode 1" {
		t.Fatalf("Title = %q, want default 'Episode 1'", got[0].Title)
	}
}

func TestStreamsReturnsM3U8WithHeaders(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasPrefix(req.URL.Path, "/anime/154587/1/"):
			return jsonResponse(http.StatusOK, `{
				"success": true,
				"data": {
					"provider": "hianime",
					"anime": {"id": 154587, "title": {"english": "Test"}},
					"episode": {
						"sources": {
							"type": "SOFT",
							"sources": {"file": "https://cdn.example.test/anime/master.m3u8"}
						}
					}
				}
			}`), nil
		case req.URL.Path == "/proxy":
			urlParam := req.URL.Query().Get("url")
			if strings.Contains(urlParam, "index-") {
				// Sub-playlist with segments using fake extensions.
				return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\n/proxy?url=https://cdn.example.test/seg1.jpg\n#EXT-X-ENDLIST"), nil
			}
			// Master playlist pointing to sub-playlist.
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1800000\n/proxy?url=https://cdn.example.test/anime/index-f1.m3u8\n"), nil
		default:
			t.Fatalf("unexpected path = %s", req.URL.Path)
			return nil, nil
		}
	})

	episodeID := encodeEpisodeID(episodeRef{AniListID: "154587", Number: "1", Provider: "hianime"})
	got, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Streams()) = %d", len(got))
	}
	// Stream URL should be a local file:// (rewritten m3u8).
	if !strings.HasPrefix(got[0].URL, "file://") {
		t.Fatalf("URL = %q, want file:// URL", got[0].URL)
	}
	if !strings.HasSuffix(got[0].URL, ".m3u8") {
		t.Fatalf("URL = %q, want .m3u8 extension", got[0].URL)
	}
	if got[0].Headers.Get("Referer") == "" {
		t.Fatal("missing Referer header")
	}
	if got[0].Headers.Get("User-Agent") == "" {
		t.Fatal("missing User-Agent header")
	}
}

func TestStreamsTriesFallbackProviders(t *testing.T) {
	hianimeAttempts := 0
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/anime/154587/1/hianime":
			hianimeAttempts++
			return jsonResponse(http.StatusBadGateway, "error"), nil
		case strings.HasPrefix(path, "/anime/154587/1/"):
			// Any other provider returns a working stream.
			return jsonResponse(http.StatusOK, `{
				"success": true,
				"data": {
					"provider": "animekai",
					"anime": {"id": 154587, "title": {"english": "Test"}},
					"episode": {
						"sources": {"type": "SOFT", "sources": {"file": "https://cdn.example.test/fallback.m3u8"}}
					}
				}
			}`), nil
		case path == "/proxy":
			urlParam := req.URL.Query().Get("url")
			if strings.Contains(urlParam, "index-") {
				return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\nseg1.ts\n#EXT-X-ENDLIST"), nil
			}
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1800000\n/proxy?url=https://cdn.example.test/index-f1.m3u8\n"), nil
		default:
			t.Fatalf("unexpected request to %s", path)
			return nil, nil
		}
	})

	episodeID := encodeEpisodeID(episodeRef{AniListID: "154587", Number: "1", Provider: "hianime"})
	got, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	// Fallback provider returns a local rewritten m3u8.
	if len(got) != 1 || !strings.HasPrefix(got[0].URL, "file://") {
		t.Fatalf("Streams() = %#v", got)
	}
	if hianimeAttempts != 3 {
		t.Fatalf("hianime attempts = %d, want 3 (with retries)", hianimeAttempts)
	}
}

func TestStreamsAllProvidersFail(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		// Return 403 (not 5xx) so retries are skipped and providers fail fast.
		return jsonResponse(http.StatusForbidden, "error"), nil
	})

	episodeID := encodeEpisodeID(episodeRef{AniListID: "1", Number: "1", Provider: "hianime"})
	_, err := client.Streams(context.Background(), episodeID)
	if err == nil {
		t.Fatal("Streams() error = nil when all providers fail")
	}
}

func TestStreamsInvalidEpisodeID(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected request")
		return nil, nil
	})

	_, err := client.Streams(context.Background(), "not-base64!!!")
	if err == nil {
		t.Fatal("Streams() error = nil for invalid episode ID")
	}
}

func TestHTTPStatusErrorsDoNotLeakURLs(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		// Use 403 (not 5xx) to avoid slow retries in tests.
		return jsonResponse(http.StatusForbidden, `signed=https://secret.example/token`), nil
	})

	_, err := client.Search(context.Background(), "154587", "anime")
	if err == nil {
		t.Fatal("Search() error = nil")
	}
	if strings.Contains(err.Error(), "signed=") || strings.Contains(err.Error(), "secret.example") {
		t.Fatalf("error leaks URL: %v", err)
	}
}

func TestHTTPErrorsRedactURL(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return nil, errors.New(`Get "https://api.example.test/anime/154587/1/hianime?signed=secret": timeout`)
	})

	_, err := client.Search(context.Background(), "154587", "anime")
	if err == nil {
		t.Fatal("Search() error = nil")
	}
	if strings.Contains(err.Error(), "signed=") || strings.Contains(err.Error(), "api.example.test") {
		t.Fatalf("error leaks URL: %v", err)
	}
}

func TestCancellation(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Search(ctx, "154587", "anime")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Search() error = %v", err)
	}
}

func TestEpisodeIDRoundTrip(t *testing.T) {
	ref := episodeRef{AniListID: "154587", Number: "1", Provider: "hianime"}
	encoded := encodeEpisodeID(ref)
	decoded, err := decodeEpisodeID(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != ref {
		t.Fatalf("round-trip: got %+v, want %+v", decoded, ref)
	}
}

func TestDecodeEpisodeIDRejectsIncomplete(t *testing.T) {
	tests := []struct {
		name string
		ref  episodeRef
	}{
		{"missing AniListID", episodeRef{Number: "1", Provider: "hianime"}},
		{"missing Number", episodeRef{AniListID: "154587", Provider: "hianime"}},
		{"missing Provider", episodeRef{AniListID: "154587", Number: "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeEpisodeID(tt.ref)
			_, err := decodeEpisodeID(encoded)
			if err == nil {
				t.Fatal("expected error for incomplete episode ID")
			}
		})
	}
}

func TestProviderOrderPreferredFirst(t *testing.T) {
	order := providerOrder("kickassanime")
	if order[0] != "kickassanime" {
		t.Fatalf("first provider = %q, want kickassanime", order[0])
	}
	if len(order) != len(availableProviders) {
		t.Fatalf("len = %d, want %d", len(order), len(availableProviders))
	}
	// Verify no duplicates.
	seen := make(map[string]bool)
	for _, p := range order {
		if seen[p] {
			t.Fatalf("duplicate provider %q", p)
		}
		seen[p] = true
	}
}

func TestSearchWithNumericIDWithWhitespace(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/anime/154587/1/hianime" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"provider": "hianime",
				"anime": {"id": 154587, "title": {"english": "Test Anime"}},
				"episode": {"details": {"id": "1", "attributes": {"number": 1}}, "sources": {"sources": {"file": "https://cdn.example.test/master.m3u8"}}}
			}
		}`), nil
	})

	got, err := client.Search(context.Background(), "  154587  ", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "154587" {
		t.Fatalf("ID = %q", got[0].ID)
	}
}

func TestSearchWithNoEnglishTitleFallsBackToAniListLabel(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"provider": "hianime",
				"anime": {"id": 123, "title": {"english": "", "romaji": "", "native": ""}},
				"episode": {"details": {"id": "1", "attributes": {"number": 1}}, "sources": {"sources": {"file": "https://cdn.example.test/master.m3u8"}}}
			}
		}`), nil
	})

	got, err := client.Search(context.Background(), "123", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "AniList #123" {
		t.Fatalf("Title = %q, want 'AniList #123'", got[0].Title)
	}
}

func TestStreamsCDNURLIsProxied(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/anime/1/1/hianime":
			return jsonResponse(http.StatusOK, `{
				"success": true,
				"data": {
					"provider": "hianime",
					"anime": {"id": 1, "title": {"english": "Test"}},
					"episode": {
						"sources": {
							"type": "SOFT",
							"sources": {"file": "https://cdn.mewstream.buzz/anime/abc123/master.m3u8"}
						}
					}
				}
			}`), nil
		case "/proxy":
			urlParam := req.URL.Query().Get("url")
			if strings.Contains(urlParam, "index-") {
				return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\nseg1.ts\n#EXT-X-ENDLIST"), nil
			}
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1800000\n/proxy?url=https://cdn.mewstream.buzz/anime/abc123/index-f1.m3u8\n"), nil
		default:
			t.Fatalf("unexpected path = %s", req.URL.Path)
			return nil, nil
		}
	})

	episodeID := encodeEpisodeID(episodeRef{AniListID: "1", Number: "1", Provider: "hianime"})
	got, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	// Should be a local file:// URL (rewritten m3u8).
	if !strings.HasPrefix(got[0].URL, "file://") {
		t.Fatalf("URL not local file: %q", got[0].URL)
	}
}

func TestStreamsAlreadyProxiedURLIsNotDoubleProxied(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/anime/1/1/hianime":
			return jsonResponse(http.StatusOK, `{
				"success": true,
				"data": {
					"provider": "hianime",
					"anime": {"id": 1, "title": {"english": "Test"}},
					"episode": {
						"sources": {
							"type": "SOFT",
							"sources": {"file": "https://api.example.test/proxy?url=https%3A%2F%2Fcdn.example.test%2Fmaster.m3u8"}
						}
					}
				}
			}`), nil
		case "/proxy":
			urlParam := req.URL.Query().Get("url")
			if strings.Contains(urlParam, "index-") {
				return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\nseg1.ts\n#EXT-X-ENDLIST"), nil
			}
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1800000\n/proxy?url=https://cdn.example.test/index-f1.m3u8\n"), nil
		default:
			t.Fatalf("unexpected path = %s", req.URL.Path)
			return nil, nil
		}
	})

	episodeID := encodeEpisodeID(episodeRef{AniListID: "1", Number: "1", Provider: "hianime"})
	got, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	// Should be a local file:// URL (rewritten m3u8).
	if !strings.HasPrefix(got[0].URL, "file://") {
		t.Fatalf("URL = %q, want file:// URL", got[0].URL)
	}
}
