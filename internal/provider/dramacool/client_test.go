package dramacool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.HTTPClient = ts.Client()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// --- Search Tests ---

// WordPress search results page format (used by ?s=query)
const searchResultsHTML = `<!DOCTYPE html>
<html><body>
<h2><a href="http://example.com/nigeru-wa-haji-da-ga-yaku-ni-tatsu" rel="bookmark">Nigeru wa Haji da ga Yaku ni Tatsu</a></h2>
<h2><a href="http://example.com/nigeru-wa-haji-da-ga-yaku-ni-tatsu-ganbare-jinrui-shinshun-special" rel="bookmark">Nigeru wa Haji da ga Yaku ni Tatsu SP</a></h2>
</body></html>`

func TestSearchParsesH2Results(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("s") != "" {
			_, _ = w.Write([]byte(searchResultsHTML))
			return
		}
		http.NotFound(w, r)
	}))

	got, err := client.Search(context.Background(), "nigeru wa haji", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Search() = %d results, want 2", len(got))
	}
	if got[0].ID != "nigeru-wa-haji-da-ga-yaku-ni-tatsu" {
		t.Fatalf("Search()[0].ID = %q", got[0].ID)
	}
	if got[0].Title != "Nigeru wa Haji da ga Yaku ni Tatsu" {
		t.Fatalf("Search()[0].Title = %q", got[0].Title)
	}
	if got[1].ID != "nigeru-wa-haji-da-ga-yaku-ni-tatsu-ganbare-jinrui-shinshun-special" {
		t.Fatalf("Search()[1].ID = %q", got[1].ID)
	}
}

func TestSearchUsesQueryParam(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := r.URL.Query().Get("s")
		if s == "" {
			t.Fatalf("missing ?s= query parameter")
		}
		_, _ = w.Write([]byte(searchResultsHTML))
	}))

	_, _ = client.Search(context.Background(), "test", "drama")
}

func TestSearchFallbackSwitchBlock(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<ul class="switch-block">
  <li>
    <a href="/crash-landing-on-you">
      <h3 class="title">Crash Landing on You</h3>
    </a>
  </li>
</ul>
</body></html>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	got, err := client.Search(context.Background(), "crash landing", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Search() = %d results, want 1", len(got))
	}
	if got[0].ID != "crash-landing-on-you" {
		t.Fatalf("Search()[0].ID = %q", got[0].ID)
	}
}

func TestSearchDetectsRedirectedDramaPage(t *testing.T) {
	// With the ?s= WordPress search, redirected drama pages no longer happen.
	// This test verifies search returns empty when the page has no h2 results.
	html := `<!DOCTYPE html>
<html><body>
<h1>Crash Landing on You</h1>
<div id="episode-list">
  <div class="block">
    <ul class="episode-list">
      <li><h3><a href="/crash-landing-on-you-episode-1">Episode 1</a></h3></li>
    </ul>
  </div>
</div>
</body></html>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	got, err := client.Search(context.Background(), "crash landing on you", "drama")
	if err != nil {
		t.Fatal(err)
	}
	// No h2 search results → empty (redirected pages are handled differently)
	if len(got) != 0 {
		t.Fatalf("Search() = %d results, want 0 (no h2 elements)", len(got))
	}
}

func TestSearchReturnsEmptyOnNoResults(t *testing.T) {
	html := `<!DOCTYPE html><html><body><p>No results found</p></body></html>`
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	got, err := client.Search(context.Background(), "nonexistent", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Search() = %d results, want 0", len(got))
	}
}

func TestSearchDeduplicatesResults(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<h2><a href="http://example.com/my-drama" rel="bookmark">My Drama</a></h2>
<h2><a href="http://example.com/my-drama" rel="bookmark">My Drama</a></h2>
</body></html>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	got, err := client.Search(context.Background(), "my drama", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Search() = %d results, want 1 (deduplicated)", len(got))
	}
}

// --- Episodes Tests ---

const dramaDetailHTML = `<!DOCTYPE html>
<html><body>
<div id="episode-list">
  <div class="block">
    <ul class="episode-list">
      <li><h3><a href="/my-drama-episode-10">My Drama Episode 10</a></h3></li>
      <li><h3><a href="/my-drama-episode-9">My Drama Episode 9</a></h3></li>
      <li><h3><a href="/my-drama-episode-1">My Drama Episode 1</a></h3></li>
    </ul>
  </div>
</div>
</body></html>`

func TestEpisodesParsesEpisodeList(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my-drama" {
			_, _ = w.Write([]byte(dramaDetailHTML))
			return
		}
		http.NotFound(w, r)
	}))

	got, err := client.Episodes(context.Background(), "my-drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("Episodes() = %d episodes, want 3", len(got))
	}
	// Should be sorted ascending (newest first in HTML, reversed).
	if got[0].Number != "1" {
		t.Fatalf("Episodes()[0].Number = %q, want 1", got[0].Number)
	}
	if got[2].Number != "10" {
		t.Fatalf("Episodes()[2].Number = %q, want 10", got[2].Number)
	}
	if got[0].ID != "my-drama-episode-1" {
		t.Fatalf("Episodes()[0].ID = %q", got[0].ID)
	}
	if got[0].SortKey != 1.0 {
		t.Fatalf("Episodes()[0].SortKey = %v, want 1.0", got[0].SortKey)
	}
}

func TestEpisodesStripsDramaDetailPrefix(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my-drama" {
			_, _ = w.Write([]byte(dramaDetailHTML))
			return
		}
		http.NotFound(w, r)
	}))

	got, err := client.Episodes(context.Background(), "drama-detail/my-drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("Episodes() = %d episodes, want 3", len(got))
	}
}

func TestEpisodesErrorOnNoEpisodeList(t *testing.T) {
	html := `<!DOCTYPE html><html><body><h1>Some Page</h1></body></html>`
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	_, err := client.Episodes(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Episodes() error = nil")
	}
}

func TestEpisodesDeduplicatesSlugs(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<div id="episode-list">
  <div class="block">
    <ul class="episode-list">
      <li><h3><a href="/drama-episode-1">Episode 1</a></h3></li>
      <li><h3><a href="/drama-episode-1">Episode 1 (duplicate)</a></h3></li>
      <li><h3><a href="/drama-episode-2">Episode 2</a></h3></li>
    </ul>
  </div>
</div>
</body></html>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	got, err := client.Episodes(context.Background(), "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Episodes() = %d episodes, want 2 (deduplicated)", len(got))
	}
}

// --- Streams Tests ---

func TestStreamsFindsStandardServer(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "my-drama") && !strings.Contains(r.URL.Path, "embed") {
			html := `<!DOCTYPE html>
<html><body>
<div class="serverslist Standard Server active" data-server="http://` + r.Host + `/embed/stream">Standard Server<span>Choose this server</span></div>
<div class="serverslist Streamtape" data-server="https://streamtape.com/e/xyz">Streamtape</div>
</body></html>`
			_, _ = w.Write([]byte(html))
			return
		}
		// Embed server page - return an m3u8
		if strings.Contains(r.URL.Path, "embed/stream") {
			_, _ = w.Write([]byte(`<html><script>var source = "https://cdn.example.com/stream.m3u8";</script></html>`))
			return
		}
		http.NotFound(w, r)
	}))

	got, err := client.Streams(context.Background(), "my-drama-episode-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("Streams() returned no streams")
	}
	if !strings.Contains(got[0].URL, ".m3u8") {
		t.Fatalf("Streams()[0].URL = %q, want m3u8 URL", got[0].URL)
	}
	if got[0].Headers.Get("Referer") == "" {
		t.Fatal("Streams()[0].Headers missing Referer")
	}
	if got[0].Headers.Get("User-Agent") == "" {
		t.Fatal("Streams()[0].Headers missing User-Agent")
	}
}

func TestStreamsDirectM3U8(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<div class="serverslist Standard Server" data-server="https://cdn.example.com/video.m3u8">Direct</div>
</body></html>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	got, err := client.Streams(context.Background(), "my-drama-episode-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Streams() = %d streams, want 1", len(got))
	}
	if got[0].URL != "https://cdn.example.com/video.m3u8" {
		t.Fatalf("Streams()[0].URL = %q", got[0].URL)
	}
}

func TestStreamsFallsBackToIframe(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "my-drama") && !strings.Contains(r.URL.Path, "embed") {
			html := `<!DOCTYPE html>
<html><body>
<iframe src="http://` + r.Host + `/embed/video" allowfullscreen></iframe>
</body></html>`
			_, _ = w.Write([]byte(html))
			return
		}
		if strings.Contains(r.URL.Path, "embed/video") {
			_, _ = w.Write([]byte(`<html><script>var url = "https://cdn.example.com/iframe.m3u8";</script></html>`))
			return
		}
		http.NotFound(w, r)
	}))

	got, err := client.Streams(context.Background(), "my-drama-episode-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("Streams() returned no streams from iframe fallback")
	}
}

func TestStreamsErrorOnNoServers(t *testing.T) {
	html := `<!DOCTYPE html><html><body><p>No video here</p></body></html>`
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))

	_, err := client.Streams(context.Background(), "my-drama-episode-1")
	if err == nil {
		t.Fatal("Streams() error = nil, want error for no servers")
	}
}

// --- Config Tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != "https://dramacool.sh" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.UserAgent == "" {
		t.Fatal("UserAgent is empty")
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %s", cfg.Timeout)
	}
}

func TestNewClientValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "missing base URL", cfg: Config{BaseURL: ""}},
		{name: "invalid base URL", cfg: Config{BaseURL: "://bad"}},
		{name: "invalid scheme", cfg: Config{BaseURL: "ftp://example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewClient(tt.cfg); err == nil {
				t.Fatal("NewClient() error = nil")
			}
		})
	}
}

func TestNewClientUsesInjectedHTTPClient(t *testing.T) {
	httpClient := &http.Client{}
	cfg := DefaultConfig()
	cfg.HTTPClient = httpClient

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if client.httpClient != httpClient {
		t.Fatal("NewClient() did not retain injected HTTP client")
	}
}

// --- HTML Helper Tests ---

func TestSlugFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://dramacool.sh/my-drama-episode-1/", "my-drama-episode-1"},
		{"/my-drama-episode-1", "my-drama-episode-1"},
		{"https://dramacool.sh/drama-detail/my-drama/", "my-drama"},
		{"", ""},
	}

	for _, tt := range tests {
		got := slugFromURL(tt.input)
		if got != tt.want {
			t.Errorf("slugFromURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHTTPStatusAndCancellation(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		_, err := client.Search(context.Background(), "test", "drama")
		if err == nil {
			t.Fatal("Search() error = nil")
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Search(ctx, "test", "drama")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Search() error = %v, want context.Canceled", err)
		}
	})
}

func TestEmbedServerDetection(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://plcool1.com/streaming.php?id=abc", true},
		{"https://asianload.io/embed/123", true},
		{"https://gogoanime.gg/embed", true},
		{"https://streamtape.com/e/xyz", false},
		{"https://cdn.example.com/video.m3u8", false},
	}

	for _, tt := range tests {
		got := isEmbedServer(tt.url)
		if got != tt.want {
			t.Errorf("isEmbedServer(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
