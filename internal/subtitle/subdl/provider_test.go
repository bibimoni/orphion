package subdl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bibimoni/orphion/internal/subtitle"
)

func TestNewProviderWithValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if prov == nil {
		t.Fatal("provider is nil")
	}
}

func TestNewProviderRejectsInvalidSiteURL(t *testing.T) {
	cfg := Config{
		SiteURL:     "not-a-url",
		DownloadURL: "https://dl.subdl.com",
		UserAgent:   "test",
		Timeout:     5 * time.Second,
	}
	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatal("NewProvider() error = nil for invalid site URL")
	}
}

func TestProviderSearchDelegatesToClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/search/") {
			pageProps := map[string]any{
				"list": []map[string]any{
					{"type": "tv", "sd_id": "sd1", "name": "Test Show", "year": 2020, "slug": "test-show", "subtitles_count": 5},
				},
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(nextDataPage(pageProps)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := Config{
		SiteURL:     server.URL,
		DownloadURL: server.URL,
		UserAgent:   "test-agent",
		Timeout:     5 * time.Second,
	}
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := prov.Search(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Test Show" {
		t.Fatalf("Search() = %#v", results)
	}
}

func TestProviderPageDelegatesToClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/subtitle/") {
			pageProps := map[string]any{
				"movieInfo": map[string]any{
					"seasons": []map[string]any{},
				},
				"groupedSubtitles": map[string]any{
					"english": []map[string]any{
						{"id": 42, "language": "english", "quality": "bluray", "link": "42-abc.zip", "bucketLink": "42/abc.zip", "author": "tester", "season": 1, "episode": 0, "title": "Full", "downloads": 10, "releases": []any{}},
					},
				},
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(nextDataPage(pageProps)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := Config{
		SiteURL:     server.URL,
		DownloadURL: server.URL,
		UserAgent:   "test-agent",
		Timeout:     5 * time.Second,
	}
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}

	page, err := prov.Page(context.Background(), "sd1", "test-show", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Subtitles) != 1 || page.Subtitles[0].Quality != "bluray" {
		t.Fatalf("Subtitles = %#v", page.Subtitles)
	}
}

func TestProviderDownloadURL(t *testing.T) {
	cfg := Config{
		SiteURL:     "https://subdl.example.test",
		DownloadURL: "https://dl.subdl.example.test",
		UserAgent:   "test-agent",
		Timeout:     5 * time.Second,
	}
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sub := subtitle.Subtitle{Link: "42-abc.zip"}
	got := prov.DownloadURL(sub)
	if !strings.Contains(got, "dl.subdl.example.test") {
		t.Fatalf("DownloadURL() = %q, want download URL", got)
	}
}

func TestProviderImplementsInterface(t *testing.T) {
	// Compile-time check.
	var _ subtitle.Provider = (*Provider)(nil)
}
