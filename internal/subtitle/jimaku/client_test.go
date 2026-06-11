package jimaku

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bibimoni/orphion/internal/subtitle"
)

func testJimakuClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	cfg := Config{
		BaseURL:    "https://jimaku.example.test",
		UserAgent:  "test-agent",
		Timeout:    5 * time.Second,
		HTTPClient: &http.Client{Transport: transport},
	}
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

func htmlResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"text/html"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewClientRequiresBaseURL(t *testing.T) {
	_, err := NewClient(Config{})
	if err == nil {
		t.Fatal("NewClient() error = nil, want base URL required error")
	}
	if !strings.Contains(err.Error(), "base URL") {
		t.Fatalf("error = %v, want base URL error", err)
	}
}

func TestNewClientWithValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}

func TestSearchParsesHomeEntriesAndFilters(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("User-Agent") == "" {
			t.Fatal("missing User-Agent header")
		}
		return htmlResp(http.StatusOK, `<html><body>
<a href="/entry/1315">"Steins;Gate"</a>
<a href="/entry/2048">"Boku no Hero Academia"</a>
<a href="/entry/9999">"Other Anime"</a>
</body></html>`), nil
	}))

	results, err := client.Search(context.Background(), "Steins Gate")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(Search()) = %d, want 1", len(results))
	}
	if results[0].ID != "jimaku:1315" {
		t.Fatalf("ID = %q, want jimaku:1315", results[0].ID)
	}
	if results[0].Title != "Steins;Gate" {
		t.Fatalf("Title = %q, want Steins;Gate (quotes stripped)", results[0].Title)
	}
	if results[0].Source != "jimaku" {
		t.Fatalf("Source = %q, want jimaku", results[0].Source)
	}
	if results[0].Slug != "1315" {
		t.Fatalf("Slug = %q, want 1315", results[0].Slug)
	}
}

func TestSearchDeduplicatesEntries(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResp(http.StatusOK, `<html><body>
<a href="/entry/1315">"Steins;Gate"</a>
<a href="/entry/1315">"Steins;Gate"</a>
</body></html>`), nil
	}))

	results, err := client.Search(context.Background(), "Steins")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(Search()) = %d, want 1 (deduplicated)", len(results))
	}
}

func TestSearchCachesHomePage(t *testing.T) {
	fetchCount := 0
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		fetchCount++
		return htmlResp(http.StatusOK, `<html><body>
<a href="/entry/1315">"Steins;Gate"</a>
</body></html>`), nil
	}))

	// First search fetches the page.
	_, err := client.Search(context.Background(), "Steins")
	if err != nil {
		t.Fatal(err)
	}
	// Second search should use cache.
	_, err = client.Search(context.Background(), "Gate")
	if err != nil {
		t.Fatal(err)
	}
	if fetchCount != 1 {
		t.Fatalf("fetchCount = %d, want 1 (cached)", fetchCount)
	}
}

func TestSearchHandlesBadStatus(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResp(http.StatusBadGateway, "error"), nil
	}))

	_, err := client.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("Search() error = nil, want error for bad status")
	}
}

func TestSearchCancellationPropagates(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Search(ctx, "test")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Search() error = %v, want context.Canceled", err)
	}
}

func TestPageParsesSubtitleFiles(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/entry/1315") {
			t.Fatalf("path = %s, want /entry/1315", req.URL.Path)
		}
		return htmlResp(http.StatusOK, `<html><body>
<a href="/entry/1315/download/steins-gate-s01e01.en.srt">Steins;Gate S01E01 [BD].en.srt</a>
<a href="/entry/1315/download/steins-gate-s01e02.ja-en.ass">Steins;Gate S01E02.ja-en.ass</a>
</body></html>`), nil
	}))

	page, err := client.Page(context.Background(), "1315", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Subtitles) != 2 {
		t.Fatalf("len(Subtitles) = %d, want 2", len(page.Subtitles))
	}

	sub1 := page.Subtitles[0]
	if sub1.Language != "english" {
		t.Fatalf("Subtitles[0].Language = %q, want english", sub1.Language)
	}
	if sub1.Quality != "bluray" {
		t.Fatalf("Subtitles[0].Quality = %q, want bluray", sub1.Quality)
	}
	if !strings.Contains(sub1.Link, "/entry/1315/download/") {
		t.Fatalf("Subtitles[0].Link = %q, want full URL", sub1.Link)
	}
	if sub1.Source != "jimaku" {
		t.Fatalf("Subtitles[0].Source = %q, want jimaku", sub1.Source)
	}

	sub2 := page.Subtitles[1]
	if sub2.Language != "japanese" {
		t.Fatalf("Subtitles[1].Language = %q, want japanese", sub2.Language)
	}
}

func TestPageStripsJimakuPrefix(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/entry/1315") {
			t.Fatalf("path = %s, want /entry/1315", req.URL.Path)
		}
		return htmlResp(http.StatusOK, `<html><body>
<a href="/entry/1315/download/test.srt">Test.srt</a>
</body></html>`), nil
	}))

	_, err := client.Page(context.Background(), "jimaku:1315", "", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPageUsesSlugWhenProvided(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/entry/2048") {
			t.Fatalf("path = %s, want /entry/2048", req.URL.Path)
		}
		return htmlResp(http.StatusOK, `<html><body></body></html>`), nil
	}))

	_, err := client.Page(context.Background(), "id", "2048", "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPageHandlesBadStatus(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResp(http.StatusNotFound, "not found"), nil
	}))

	_, err := client.Page(context.Background(), "1315", "", "")
	if err == nil {
		t.Fatal("Page() error = nil, want error for bad status")
	}
}

func TestDownloadURLReturnsLinkDirectly(t *testing.T) {
	client := testJimakuClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not be called")
	}))

	sub := subtitle.Subtitle{Link: "https://jimaku.cc/entry/1315/download/test.srt"}
	got := client.DownloadURL(sub)
	if got != "https://jimaku.cc/entry/1315/download/test.srt" {
		t.Fatalf("DownloadURL() = %q, want direct link", got)
	}
}

func TestCleanTitleStripsDoubleQuotes(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{`"Steins;Gate"`, "Steins;Gate"},
		{"\u201CSteins;Gate\u201D", "Steins;Gate"},
		{"No Quotes", "No Quotes"},
		{`"Only Start`, `"Only Start`},
		{"  padded  ", "padded"},
	}
	for _, tt := range tests {
		got := cleanTitle(tt.input)
		if got != tt.expect {
			t.Errorf("cleanTitle(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name   string
		expect string
	}{
		{"show.en.srt", "english"},
		{"show.ja-en.ass", "japanese"},
		{"show.ja.srt", "japanese"},
		{"show.english.srt", "english"},
		{"show.japanese.srt", "japanese"},
		{"show.ja[cc].srt", "japanese"},
		{"日本語字幕.srt", "japanese"},
		{"plain.srt", "english"},
	}
	for _, tt := range tests {
		got := detectLanguage(tt.name)
		if got != tt.expect {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.name, got, tt.expect)
		}
	}
}

func TestDetectQuality(t *testing.T) {
	tests := []struct {
		name   string
		expect string
	}{
		{"Show.BD.srt", "bluray"},
		{"Show.bluray.srt", "bluray"},
		{"Show.webrip.srt", "webrip"},
		{"Show.web-dl.srt", "webrip"},
		{"Show.netflix.srt", "webrip"},
		{"Show.srt", "other"},
	}
	for _, tt := range tests {
		got := detectQuality(tt.name)
		if got != tt.expect {
			t.Errorf("detectQuality(%q) = %q, want %q", tt.name, got, tt.expect)
		}
	}
}

func TestTokenizeQuery(t *testing.T) {
	tokens := tokenizeQuery("Steins Gate")
	if !tokens["steins"] || !tokens["gate"] {
		t.Fatalf("tokenizeQuery() = %v, want steins and gate", tokens)
	}
	// Short tokens (< 2 chars) should be excluded.
	tokens2 := tokenizeQuery("a b cd")
	if tokens2["a"] || tokens2["b"] {
		t.Fatalf("tokenizeQuery() included short tokens: %v", tokens2)
	}
	if !tokens2["cd"] {
		t.Fatalf("tokenizeQuery() missing 'cd': %v", tokens2)
	}
}

func TestHasTokenOverlap(t *testing.T) {
	tokens := tokenizeQuery("Steins Gate")
	if !hasTokenOverlap("Steins;Gate", tokens) {
		t.Error("hasTokenOverlap should match prefix 'steins'")
	}
	if hasTokenOverlap("Totally Different", tokens) {
		t.Error("hasTokenOverlap should not match unrelated title")
	}
}

func TestNewServerIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
<a href="/entry/1315">"Steins;Gate"</a>
<a href="/entry/2048">"Boku no Hero Academia"</a>
</body></html>`))
	})
	mux.HandleFunc("/entry/1315", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
<a href="/entry/1315/download/sg-s01e01.en.srt">Steins;Gate S01E01.en.srt</a>
</body></html>`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := Config{
		BaseURL:   server.URL,
		UserAgent: "test-agent",
		Timeout:   5 * time.Second,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.Search(context.Background(), "Steins")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Steins;Gate" {
		t.Fatalf("Search() = %#v", results)
	}

	page, err := client.Page(context.Background(), "1315", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Subtitles) != 1 {
		t.Fatalf("len(Subtitles) = %d, want 1", len(page.Subtitles))
	}
	if page.Subtitles[0].Language != "english" {
		t.Fatalf("Subtitles[0].Language = %q, want english", page.Subtitles[0].Language)
	}
}
