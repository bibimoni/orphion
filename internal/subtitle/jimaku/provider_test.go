package jimaku

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bibimoni/orphion/internal/subtitle"
)

func TestNewProviderCreatesProvider(t *testing.T) {
	cfg := DefaultConfig()
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if prov == nil {
		t.Fatal("provider is nil")
	}
}

func TestNewProviderRejectsEmptyBaseURL(t *testing.T) {
	cfg := Config{UserAgent: "test", Timeout: 5 * time.Second}
	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatal("NewProvider() error = nil, want base URL required error")
	}
}

func TestProviderSearchDelegatesToClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
<a href="/entry/1315">"Steins;Gate"</a>
</body></html>`))
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, UserAgent: "test", Timeout: 5 * time.Second}
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := prov.Search(context.Background(), "Steins")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(Search()) = %d, want 1", len(results))
	}
}

func TestProviderPageDelegatesToClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/entry/1315") {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<a href="/entry/1315/download/test.en.srt">Test.en.srt</a>
</body></html>`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html><body></body></html>")
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, UserAgent: "test", Timeout: 5 * time.Second}
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}

	page, err := prov.Page(context.Background(), "1315", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Subtitles) != 1 {
		t.Fatalf("len(Subtitles) = %d, want 1", len(page.Subtitles))
	}
}

func TestProviderDownloadURL(t *testing.T) {
	cfg := Config{BaseURL: "https://jimaku.example.test", UserAgent: "test", Timeout: 5 * time.Second}
	prov, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sub := subtitle.Subtitle{Link: "https://jimaku.example.test/entry/1315/download/test.srt"}
	got := prov.DownloadURL(sub)
	if got != sub.Link {
		t.Fatalf("DownloadURL() = %q, want %q", got, sub.Link)
	}
}

func TestProviderImplementsInterface(t *testing.T) {
	// Compile-time check: Provider must satisfy subtitle.Provider.
	var _ subtitle.Provider = (*Provider)(nil)
}
