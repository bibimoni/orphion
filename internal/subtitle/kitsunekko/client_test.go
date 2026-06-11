package kitsunekko

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bibimoni/orphion/internal/subtitle"
)

func TestParseDirListing(t *testing.T) {
	html := `<html>
<head><title>Index of /subtitles/japanese</title></head>
<body>
<h1>Index of /subtitles/japanese</h1>
<table>
<tr><td valign="top"><a href="/subtitles/">Parent Directory</a></td><td>&nbsp;</td></tr>
<tr><td><a href="Dagashi_Kashi/">Dagashi_Kashi/</a></td><td>2024-01-01</td></tr>
<tr><td><a href="Steins_Gate/">Steins_Gate/</a></td><td>2024-01-02</td></tr>
<tr><td><a href="Naruto/">Naruto/</a></td><td>2024-01-03</td></tr>
</table>
</body>
</html>`

	entries := parseDirListing(html)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// First is Parent Directory.
	if entries[0].Name != "Parent Directory" {
		t.Errorf("entry[0].Name = %q, want Parent Directory", entries[0].Name)
	}
	if entries[1].Name != "Dagashi_Kashi/" {
		t.Errorf("entry[1].Name = %q, want Dagashi_Kashi/", entries[1].Name)
	}
	if entries[1].Href != "Dagashi_Kashi/" {
		t.Errorf("entry[1].Href = %q, want Dagashi_Kashi/", entries[1].Href)
	}
}

func TestParseDirListingFiles(t *testing.T) {
	html := `<html>
<head><title>Index of /subtitles/japanese/Dagashi_Kashi</title></head>
<body>
<h1>Index of /subtitles/japanese/Dagashi_Kashi</h1>
<table>
<tr><td><a href="/subtitles/japanese/">Parent Directory</a></td></tr>
<tr><td><a href="Dagashi%20Kashi%20(01-12)%20(Webrip).zip">Dagashi Kashi (01-12) (Webrip).zip</a></td></tr>
<tr><td><a href="Dagashi_Kashi%5b1-12%5d.zip">Dagashi_Kashi[1-12].zip</a></td></tr>
</table>
</body>
</html>`

	entries := parseDirListing(html)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[1].Name != "Dagashi Kashi (01-12) (Webrip).zip" {
		t.Errorf("entry[1].Name = %q", entries[1].Name)
	}
}

func TestClientSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subtitles/japanese/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<a href="/subtitles/japanese/">Parent Directory</a>
<a href="Dagashi_Kashi/">Dagashi_Kashi/</a>
<a href="Steins_Gate/">Steins_Gate/</a>
</body></html>`))
			return
		}
		if r.URL.Path == "/subtitles/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<a href="/subtitles/">Parent Directory</a>
<a href="Attack%20On%20Titan/">Attack On Titan/</a>
</body></html>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:   server.URL,
		Languages: []string{"japanese", ""},
		UserAgent: "test",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.Search(t.Context(), "Dagashi")
	if err != nil {
		t.Fatal(err)
	}
	// Pre-filtering drops directories that share no tokens with the query.
	// "Dagashi_Kashi" matches "Dagashi"; "Steins_Gate" and "Attack On Titan" don't.
	if len(results) != 1 {
		t.Fatalf("expected 1 result (pre-filtered), got %d", len(results))
	}
	if results[0].Title != "Dagashi_Kashi" {
		t.Fatalf("expected Dagashi_Kashi, got %s", results[0].Title)
	}
	if results[0].Source != "kitsunekko" {
		t.Errorf("Source = %q, want kitsunekko", results[0].Source)
	}
	if !strings.HasPrefix(results[0].ID, "kitsunekko:ja:") {
		t.Errorf("ID = %q, want kitsunekko:ja: prefix", results[0].ID)
	}
}

func TestClientPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subtitles/japanese/Dagashi_Kashi/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<a href="/subtitles/japanese/">Parent Directory</a>
<a href="Dagashi%20Kashi%20(01-12)%20(Webrip).zip">Dagashi Kashi (01-12) (Webrip).zip</a>
<a href="Dagashi_Kashi%5b1-12%5d.zip">Dagashi_Kashi[1-12].zip</a>
</body></html>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:   server.URL,
		Languages: []string{"japanese"},
		UserAgent: "test",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	page, err := client.Page(t.Context(), "ja:Dagashi_Kashi", "ja:Dagashi_Kashi", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Subtitles) != 2 {
		t.Fatalf("expected 2 subtitles, got %d", len(page.Subtitles))
	}
	if page.Subtitles[0].Language != "japanese" {
		t.Errorf("subtitle[0].Language = %q, want japanese", page.Subtitles[0].Language)
	}
	if !strings.HasSuffix(page.Subtitles[0].Link, ".zip") {
		t.Errorf("subtitle[0].Link = %q, want .zip URL", page.Subtitles[0].Link)
	}
}

func TestClientDownloadURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "https://example.com"
	client, _ := NewClient(cfg)

	sub := subtitle.Subtitle{Link: "https://example.com/subtitles/japanese/Anime/file.zip"}
	url := client.DownloadURL(sub)
	if url != sub.Link {
		t.Errorf("DownloadURL = %q, want %q", url, sub.Link)
	}
}

func TestParseSlugLang(t *testing.T) {
	tests := []struct {
		slug     string
		wantLang string
		wantSlug string
	}{
		{"ja:Steins_Gate", "ja", "Steins_Gate"},
		{"Steins_Gate", "", "Steins_Gate"},
		{"english:Attack_On_Titan", "english", "Attack_On_Titan"},
	}

	for _, tt := range tests {
		lang, slug := parseSlugLang(tt.slug)
		if lang != tt.wantLang {
			t.Errorf("parseSlugLang(%q) lang = %q, want %q", tt.slug, lang, tt.wantLang)
		}
		if slug != tt.wantSlug {
			t.Errorf("parseSlugLang(%q) slug = %q, want %q", tt.slug, slug, tt.wantSlug)
		}
	}
}

func TestLangLabel(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"japanese", "japanese"},
		{"ja", "japanese"},
		{"", "english"},
		{"english", "english"},
		{"korean", "korean"},
	}
	for _, tt := range tests {
		got := langLabel(tt.lang)
		if got != tt.want {
			t.Errorf("langLabel(%q) = %q, want %q", tt.lang, got, tt.want)
		}
	}
}

func TestClientCachesDirListings(t *testing.T) {
	fetchCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
<a href="Naruto/">Naruto/</a>
</body></html>`))
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:   server.URL,
		Languages: []string{"japanese"},
		UserAgent: "test",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// First search triggers a network fetch.
	_, err = client.Search(t.Context(), "naruto")
	if err != nil {
		t.Fatal(err)
	}
	firstFetch := fetchCount

	// Second search should use the cache — no additional network requests.
	_, err = client.Search(t.Context(), "naruto")
	if err != nil {
		t.Fatal(err)
	}
	if fetchCount != firstFetch {
		t.Errorf("expected no additional fetches for cached search, got %d total (was %d)", fetchCount, firstFetch)
	}
}

func TestNormalizeDirNameSemicolons(t *testing.T) {
	// Semicolons should be treated as separators, not part of tokens.
	// This ensures "Steins;Gate" produces tokens {"steins", "gate"}
	// instead of {"steins;gate"}, which matches the ranking algorithm.
	tests := []struct {
		input string
		want  string
	}{
		{"Steins;Gate", "steins gate"},
		{"Stein;Gate", "stein gate"},
		{"Re:Zero", "re zero"},
		{"Dagashi_Kashi", "dagashi kashi"},
		{"Bleach:", "bleach"},
		{"A.I.C.O.", "a i c o"},
	}
	for _, tt := range tests {
		got := normalizeDirName(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDirName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClientSearchTypoMatch(t *testing.T) {
	// Simulate the Kitsunekko scenario where "Stein;Gate" (typo, no 's')
	// is listed alongside "Steins;Gate 0" (correctly spelled sequel).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/subtitles/japanese/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>
<a href="/subtitles/japanese/">Parent Directory</a>
<a href="Stein;Gate/">Stein;Gate/</a>
<a href="Steins;Gate%200/">Steins;Gate 0/</a>
<a href="Naruto/">Naruto/</a>
</body></html>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:   server.URL,
		Languages: []string{"japanese"},
		UserAgent: "test",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	results, err := client.Search(t.Context(), "Steins;Gate")
	if err != nil {
		t.Fatal(err)
	}

	// Both "Stein;Gate" and "Steins;Gate 0" should pass the pre-filter.
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results (typo + sequel), got %d", len(results))
	}

	// Rank results — the typo'd original should rank higher than the sequel.
	ranked := subtitle.RankResults("Steins;Gate", results, 20, 0.2)
	if len(ranked) < 2 {
		t.Fatalf("RankResults: expected at least 2 ranked results, got %d", len(ranked))
	}
	if ranked[0].Title != "Stein;Gate" {
		t.Errorf("RankResults first = %q, want %q (typo'd original should rank first)", ranked[0].Title, "Stein;Gate")
	}
}
