package subdl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bibimoni/orphion/internal/subtitle"
)

// nextDataPage wraps pageProps as a full __NEXT_DATA__ HTML document.
func nextDataPage(pageProps any) string {
	propsJSON, _ := json.Marshal(pageProps)
	envelope := map[string]any{
		"props": map[string]any{
			"pageProps": json.RawMessage(propsJSON),
		},
	}
	envelopeJSON, _ := json.Marshal(envelope)
	return fmt.Sprintf(
		`<!DOCTYPE html><html><head><script id="__NEXT_DATA__" type="application/json">%s</script></head><body></body></html>`,
		string(envelopeJSON),
	)
}

func testClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.SiteURL = "https://subdl.example.test"
	cfg.DownloadURL = "https://dl.subdl.example.test"
	cfg.HTTPClient = &http.Client{Transport: transport}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func htmlResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"text/html"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSearchParsesNextDataAndMapsResults(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s", req.Method)
		}
		if !strings.Contains(req.URL.Path, "/search/") {
			t.Fatalf("path = %s", req.URL.Path)
		}
		if req.Header.Get("User-Agent") == "" {
			t.Fatal("missing User-Agent header")
		}
		pageProps := map[string]any{
			"list": []map[string]any{
				{"type": "tv", "sd_id": "sd1300065", "name": "Naruto", "year": 2002, "slug": "naruto", "subtitles_count": 10},
				{"type": "movie", "sd_id": "sd999", "name": "Spirited Away", "year": 2001, "slug": "spirited-away", "subtitles_count": 5},
			},
		}
		return htmlResponse(http.StatusOK, nextDataPage(pageProps)), nil
	}))

	got, err := client.Search(context.Background(), "naruto")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Search()) = %d", len(got))
	}
	if got[0].ID != "subdl:sd1300065" || got[0].Title != "Naruto" || got[0].Type != "tv" {
		t.Fatalf("Search()[0] = %#v", got[0])
	}
	if got[1].Year != 2001 || got[1].Slug != "spirited-away" {
		t.Fatalf("Search()[1] = %#v", got[1])
	}
}

func TestSearchSkipsEntriesWithMissingIDOrName(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		pageProps := map[string]any{
			"list": []map[string]any{
				{"sd_id": "", "name": "No ID"},
				{"sd_id": "sd1", "name": ""},
				{"sd_id": "sd2", "name": "Valid"},
			},
		}
		return htmlResponse(http.StatusOK, nextDataPage(pageProps)), nil
	}))

	got, err := client.Search(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Valid" {
		t.Fatalf("Search() = %#v", got)
	}
}

func TestPageReturnsSeasonsAndSubtitles(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/subtitle/sd1300065/naruto") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		pageProps := map[string]any{
			"movieInfo": map[string]any{
				"seasons": []map[string]any{
					{"number": "first-season", "name": "Season 1"},
					{"number": "second-season", "name": "Season 2"},
				},
			},
			"groupedSubtitles": map[string]any{
				"english": []map[string]any{
					{
						"id": 3455495, "language": "english", "quality": "other",
						"link": "3455495-8378310.zip", "bucketLink": "3455495/8378310.zip",
						"author": "mo92", "season": 1, "episode": 0,
						"title": "Naruto S01", "downloads": 1466,
						"releases": []any{"Naruto Season 1 x264 v3 JySzE"},
					},
				},
			},
		}
		return htmlResponse(http.StatusOK, nextDataPage(pageProps)), nil
	}))

	got, err := client.Page(context.Background(), "sd1300065", "naruto", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Seasons) != 2 {
		t.Fatalf("len(Seasons) = %d", len(got.Seasons))
	}
	if got.Seasons[0].Slug != "first-season" || got.Seasons[0].Name != "Season 1" {
		t.Fatalf("Seasons[0] = %#v", got.Seasons[0])
	}
	if len(got.Subtitles) != 1 {
		t.Fatalf("len(Subtitles) = %d", len(got.Subtitles))
	}
	sub := got.Subtitles[0]
	if sub.ID != 3455495 || sub.Language != "english" || sub.Link != "3455495-8378310.zip" {
		t.Fatalf("Subtitles[0] = %#v", sub)
	}
	if sub.Episode != 0 {
		t.Fatalf("Episode = %d, want 0 (all episodes)", sub.Episode)
	}
	if len(sub.Releases) != 1 || sub.Releases[0] != "Naruto Season 1 x264 v3 JySzE" {
		t.Fatalf("Releases = %#v", sub.Releases)
	}
}

func TestPageWithSeasonSlug(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/subtitle/sd1300065/naruto/first-season") {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		pageProps := map[string]any{
			"movieInfo": map[string]any{
				"seasons": []map[string]any{},
			},
			"groupedSubtitles": map[string]any{
				"english": []map[string]any{
					{"id": 1, "language": "english", "quality": "webdl", "link": "test.zip", "bucketLink": "", "author": "user", "season": 1, "episode": 5, "title": "Ep 5", "downloads": 100, "releases": []any{}},
				},
			},
		}
		return htmlResponse(http.StatusOK, nextDataPage(pageProps)), nil
	}))

	got, err := client.Page(context.Background(), "sd1300065", "naruto", "first-season")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Subtitles) != 1 || got.Subtitles[0].Episode != 5 {
		t.Fatalf("Subtitles = %#v", got.Subtitles)
	}
}

func TestPageWithEmptyGroupedSubtitles(t *testing.T) {
	// SubDL returns groupedSubtitles as [] (empty array) when the base page
	// has seasons but no subtitles for the default view.
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		pageProps := map[string]any{
			"movieInfo": map[string]any{
				"seasons": []map[string]any{
					{"number": "first-season", "name": "Season 1"},
				},
			},
			"groupedSubtitles": []any{},
		}
		return htmlResponse(http.StatusOK, nextDataPage(pageProps)), nil
	}))

	got, err := client.Page(context.Background(), "sd1234", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Seasons) != 1 {
		t.Fatalf("len(Seasons) = %d, want 1", len(got.Seasons))
	}
	if len(got.Subtitles) != 0 {
		t.Fatalf("len(Subtitles) = %d, want 0", len(got.Subtitles))
	}
}

func TestDownloadURL(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not be called")
	}))

	sub := subtitle.Subtitle{Link: "3455495-8378310.zip"}
	got := client.DownloadURL(sub)
	want := "https://dl.subdl.example.test/subtitle/3455495-8378310.zip"
	if got != want {
		t.Fatalf("DownloadURL() = %q, want %q", got, want)
	}
}

func TestFetchNextDataHandlesBadStatus(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResponse(http.StatusBadGateway, "<html>error</html>"), nil
	}))

	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("Search() error = nil")
	}
	if !strings.Contains(err.Error(), "upstream status") {
		t.Fatalf("error = %v", err)
	}
}

func TestFetchNextDataHandlesMissingNextData(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResponse(http.StatusOK, "<html><body>No next data here</body></html>"), nil
	}))

	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("Search() error = nil")
	}
	if !strings.Contains(err.Error(), "__NEXT_DATA__") {
		t.Fatalf("error = %v", err)
	}
}

func TestCancellationPropagates(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Search(ctx, "test")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Search() error = %v", err)
	}
}

func TestNewServerIntegration(t *testing.T) {
	// Use httptest.NewServer for a full integration test.
	mux := http.NewServeMux()
	mux.HandleFunc("/search/", func(w http.ResponseWriter, r *http.Request) {
		pageProps := map[string]any{
			"list": []map[string]any{
				{"type": "tv", "sd_id": "sd1", "name": "Test Show", "year": 2020, "slug": "test-show", "subtitles_count": 3},
			},
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(nextDataPage(pageProps)))
	})
	mux.HandleFunc("/subtitle/", func(w http.ResponseWriter, r *http.Request) {
		pageProps := map[string]any{
			"movieInfo": map[string]any{
				"seasons": []map[string]any{
					{"number": "first-season", "name": "Season 1"},
				},
			},
			"groupedSubtitles": map[string]any{
				"english": []map[string]any{
					{"id": 42, "language": "english", "quality": "bluray", "link": "42-abc.zip", "bucketLink": "42/abc.zip", "author": "tester", "season": 1, "episode": 1, "title": "S01E01", "downloads": 50, "releases": []any{"TestRelease"}},
				},
			},
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(nextDataPage(pageProps)))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := Config{
		SiteURL:     server.URL,
		DownloadURL: server.URL,
		UserAgent:   "test-agent",
		Timeout:     5 * time.Second,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.Search(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Test Show" {
		t.Fatalf("Search() = %#v", results)
	}

	page, err := client.Page(context.Background(), "sd1", "test-show", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Seasons) != 1 || page.Seasons[0].Name != "Season 1" {
		t.Fatalf("Seasons = %#v", page.Seasons)
	}
	if len(page.Subtitles) != 1 || page.Subtitles[0].Quality != "bluray" {
		t.Fatalf("Subtitles = %#v", page.Subtitles)
	}
}
