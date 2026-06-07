package nyaa

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

// --- RSS fixture ---

const rssSearchResult = `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:atom="http://www.w3.org/2005/Atom" xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
<channel>
<title>Nyaa - Test Query</title>
<item>
<title>[J-Drama] Test Drama - 720p (01-11) (2016)</title>
<link>https://nyaa.si/download/12345.torrent</link>
<guid isPermaLink="true">https://nyaa.si/view/12345</guid>
<nyaa:seeders>8</nyaa:seeders>
<nyaa:infoHash>abcdef1234567890abcdef1234567890abcdef12</nyaa:infoHash>
<nyaa:size>2.0 GiB</nyaa:size>
</item>
<item>
<title>[J-Drama] Another Drama SP</title>
<link>https://nyaa.si/download/67890.torrent</link>
<guid isPermaLink="true">https://nyaa.si/view/67890</guid>
<nyaa:seeders>3</nyaa:seeders>
<nyaa:infoHash>fedcba0987654321fedcba0987654321fedcba09</nyaa:infoHash>
<nyaa:size>1.5 GiB</nyaa:size>
</item>
</channel>
</rss>`

// --- Search Tests ---

func TestSearchParsesRSSItems(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "rss" {
			t.Fatalf("expected page=rss, got %q", r.URL.Query().Get("page"))
		}
		_, _ = w.Write([]byte(rssSearchResult))
	}))

	got, err := client.Search(context.Background(), "test drama", "drama")
	if err != nil {
		t.Fatal(err)
	}
	// Two different show titles → 2 groups
	if len(got) != 2 {
		t.Fatalf("Search() = %d results, want 2", len(got))
	}
	// Results should be grouped (show IDs are infoHashes)
	for _, r := range got {
		if r.ID == "" || r.Title == "" {
			t.Fatalf("Search() result has empty ID or Title: %+v", r)
		}
	}
}

func TestSearchUsesCorrectCategory(t *testing.T) {
	tests := []struct {
		kind         string
		wantCategory string
	}{
		{"drama", "4_3"},
		{"anime", "1_2"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got := r.URL.Query().Get("c")
				if got != tt.wantCategory {
					t.Fatalf("category = %q, want %q", got, tt.wantCategory)
				}
				_, _ = w.Write([]byte(rssSearchResult))
			}))
			_, _ = client.Search(context.Background(), "test", tt.kind)
		})
	}
}

func TestSearchSendsQueryParameter(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q != "nigeru wa haji" {
			t.Fatalf("query = %q, want %q", q, "nigeru wa haji")
		}
		_, _ = w.Write([]byte(rssSearchResult))
	}))
	_, _ = client.Search(context.Background(), "nigeru wa haji", "drama")
}

func TestSearchDeduplicatesResults(t *testing.T) {
	dupRSS := `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
<channel>
<item>
<title>Drama A</title><link>https://nyaa.si/download/1.torrent</link>
<nyaa:infoHash>aaaa1111bbbb2222cccc3333dddd4444eeee5555</nyaa:infoHash>
</item>
<item>
<title>Drama A (duplicate)</title><link>https://nyaa.si/download/2.torrent</link>
<nyaa:infoHash>aaaa1111bbbb2222cccc3333dddd4444eeee5555</nyaa:infoHash>
</item>
</channel>
</rss>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(dupRSS))
	}))

	got, err := client.Search(context.Background(), "drama", "drama")
	if err != nil {
		t.Fatal(err)
	}
	// Deduplicated by infoHash → 1 group with 1 torrent
	if len(got) != 1 {
		t.Fatalf("Search() = %d results, want 1 (deduplicated)", len(got))
	}
}

func TestSearchReturnsEmptyOnNoResults(t *testing.T) {
	emptyRSS := `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
<channel></channel>
</rss>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(emptyRSS))
	}))

	got, err := client.Search(context.Background(), "nonexistent", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("Search() = %d results, want 0", len(got))
	}
}

func TestSearchSkipsItemsWithoutInfoHash(t *testing.T) {
	noHashRSS := `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
<channel>
<item>
<title>No Hash</title><link>https://nyaa.si/download/1.torrent</link>
</item>
<item>
<title>Has Hash</title><link>https://nyaa.si/download/2.torrent</link>
<nyaa:infoHash>abcdef1234567890abcdef1234567890abcdef12</nyaa:infoHash>
</item>
</channel>
</rss>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(noHashRSS))
	}))

	got, err := client.Search(context.Background(), "test", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Search() = %d results, want 1", len(got))
	}
}

func TestSearchGroupsBySimilarTitle(t *testing.T) {
	groupRSS := `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
<channel>
<item>
<title>[SubGroup] Test Drama EP01 [720p]</title><link>https://nyaa.si/download/1.torrent</link>
<nyaa:infoHash>aaaa1111bbbb2222cccc3333dddd4444eeee5555</nyaa:infoHash>
</item>
<item>
<title>[SubGroup] Test Drama EP02 [720p]</title><link>https://nyaa.si/download/2.torrent</link>
<nyaa:infoHash>bbbb2222cccc3333dddd4444eeee5555ffff6666</nyaa:infoHash>
</item>
<item>
<title>[OtherGroup] Different Show EP01 [1080p]</title><link>https://nyaa.si/download/3.torrent</link>
<nyaa:infoHash>cccc3333dddd4444eeee5555ffff6666aaaa7777</nyaa:infoHash>
</item>
</channel>
</rss>`

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(groupRSS))
	}))

	got, err := client.Search(context.Background(), "test drama", "drama")
	if err != nil {
		t.Fatal(err)
	}
	// Two different show titles: "Test Drama" and "Different Show"
	if len(got) != 2 {
		t.Fatalf("Search() = %d results, want 2 groups", len(got))
	}

	// Find the "Test Drama" group and verify it has 2 torrents
	for _, r := range got {
		if strings.Contains(r.Title, "Test Drama") {
			group, ok := client.cache.GetShow(r.ID)
			if !ok {
				t.Fatal("cache miss for Test Drama group")
			}
			if len(group.torrents) != 2 {
				t.Fatalf("Test Drama group has %d torrents, want 2", len(group.torrents))
			}
		}
	}
}

// --- Episodes Tests ---

func TestEpisodesFromCachedSearch(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssSearchResult))
	}))

	// Must search first to populate cache
	results, err := client.Search(context.Background(), "test drama", "drama")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no results")
	}

	// Now Episodes should work with the cached show ID
	episodes, err := client.Episodes(context.Background(), results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) == 0 {
		t.Fatal("Episodes() returned no episodes")
	}
}

func TestEpisodesUsesJapaneseNumeralsAndDoesNotPromoteSpecialToEpisodeOne(t *testing.T) {
	rss := `<?xml version="1.0" encoding="utf-8"?>
<rss xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
<channel>
<item>
<title>【逃げるは恥だが役に立つ (2021)】 ガンバレ人類! 新春SP!!.mp4</title>
<link>https://nyaa.si/download/sp.torrent</link>
<nyaa:infoHash>add312cfb18efee75d8c5067ad66ef4fcb0b5e7d</nyaa:infoHash>
</item>
<item>
<title>逃げるは恥だが役に立つ 第二話/NIGERUHA.HAJIDAGA.YAKUNITATSU.Ep02.mp4</title>
<link>https://nyaa.si/download/ep2.torrent</link>
<nyaa:infoHash>b77dc035f2b95579c7f44d77e2fcc8b1d053fd73</nyaa:infoHash>
</item>
<item>
<title>逃げるは恥だが役に立つ 第一話/NIGERUHA.HAJIDAGA.YAKUNITATSU.Ep01.mp4</title>
<link>https://nyaa.si/download/ep1.torrent</link>
<nyaa:infoHash>0fe5fdebfd07bb8252f28b583adabdaea8fb2b77</nyaa:infoHash>
</item>
</channel>
</rss>`
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rss))
	}))

	results, err := client.Search(context.Background(), "逃げ恥", "drama")
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := client.Episodes(context.Background(), results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 2 {
		t.Fatalf("Episodes() = %d episodes, want 2 parsed episodes", len(episodes))
	}
	if episodes[0].ID != "0fe5fdebfd07bb8252f28b583adabdaea8fb2b77:1" {
		t.Fatalf("episode 1 ID = %q, want actual episode 1 torrent", episodes[0].ID)
	}
	if episodes[1].ID != "b77dc035f2b95579c7f44d77e2fcc8b1d053fd73:2" {
		t.Fatalf("episode 2 ID = %q, want actual episode 2 torrent", episodes[1].ID)
	}
	if episodes[0].Title != "逃げるは恥だが役に立つ 第一話/NIGERUHA.HAJIDAGA.YAKUNITATSU.Ep01.mp4" {
		t.Fatalf("episode 1 title = %q, want source title", episodes[0].Title)
	}
}

func TestEpisodesErrorOnEmptyID(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	_, err := client.Episodes(context.Background(), "")
	if err == nil {
		t.Fatal("Episodes() error = nil, want error for empty ID")
	}
}

func TestEpisodesFallbackOnUnknownShow(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	// When the show ID is not in cache (e.g. --title-id used without prior
	// search), Episodes() should return a single fallback episode using
	// the ID as the infoHash.
	eps, err := client.Episodes(context.Background(), "nonexistent_show_id")
	if err != nil {
		t.Fatalf("Episodes() error = %v, want nil (fallback)", err)
	}
	if len(eps) != 1 {
		t.Fatalf("Episodes() = %d episodes, want 1 fallback", len(eps))
	}
	if eps[0].ID != "nonexistent_show_id" {
		t.Fatalf("fallback episode ID = %q, want %q", eps[0].ID, "nonexistent_show_id")
	}
	if eps[0].Number != "1" {
		t.Fatalf("fallback episode Number = %q, want 1", eps[0].Number)
	}
}

// --- EpisodesFromTitle Tests ---

func TestEpisodesFromTitleRange(t *testing.T) {
	got := EpisodesFromTitle("hash123", "[J-Drama] Test Drama - 720p (01-11) (2016)")
	if len(got) != 11 {
		t.Fatalf("EpisodesFromTitle() = %d episodes, want 11", len(got))
	}
	if got[0].Number != "1" {
		t.Fatalf("EpisodesFromTitle()[0].Number = %q, want 1", got[0].Number)
	}
	if got[10].Number != "11" {
		t.Fatalf("EpisodesFromTitle()[10].Number = %q, want 11", got[10].Number)
	}
	if got[0].SortKey != 1.0 {
		t.Fatalf("EpisodesFromTitle()[0].SortKey = %v, want 1.0", got[0].SortKey)
	}
	if got[0].ID != "hash123:1" {
		t.Fatalf("EpisodesFromTitle()[0].ID = %q", got[0].ID)
	}
}

func TestEpisodesFromTitleSingle(t *testing.T) {
	got := EpisodesFromTitle("hash456", "[J-Drama] Special EP03")
	if len(got) != 1 {
		t.Fatalf("EpisodesFromTitle() = %d episodes, want 1", len(got))
	}
	if got[0].Number != "3" {
		t.Fatalf("EpisodesFromTitle()[0].Number = %q, want 3", got[0].Number)
	}
	if got[0].SortKey != 3.0 {
		t.Fatalf("EpisodesFromTitle()[0].SortKey = %v, want 3.0", got[0].SortKey)
	}
}

func TestEpisodesFromTitleBracket(t *testing.T) {
	got := EpisodesFromTitle("hash789", "[SubGroup] Test Drama [05] [1080p]")
	if len(got) != 1 {
		t.Fatalf("EpisodesFromTitle() = %d episodes, want 1", len(got))
	}
	if got[0].Number != "5" {
		t.Fatalf("EpisodesFromTitle()[0].Number = %q, want 5", got[0].Number)
	}
}

func TestEpisodesFromTitleJapaneseKanjiEpisode(t *testing.T) {
	got := EpisodesFromTitle("hash-kanji", "逃げるは恥だが役に立つ 第一話")
	if len(got) != 1 {
		t.Fatalf("EpisodesFromTitle() = %d episodes, want 1", len(got))
	}
	if got[0].Number != "1" {
		t.Fatalf("EpisodesFromTitle()[0].Number = %q, want 1", got[0].Number)
	}
	if got[0].ID != "hash-kanji:1" {
		t.Fatalf("EpisodesFromTitle()[0].ID = %q, want hash-kanji:1", got[0].ID)
	}
}

func TestEpisodesFromTitleFallback(t *testing.T) {
	got := EpisodesFromTitle("hash789", "[J-Drama] Some Batch Release")
	if len(got) != 1 {
		t.Fatalf("EpisodesFromTitle() = %d episodes, want 1", len(got))
	}
	if got[0].Number != "1" {
		t.Fatalf("EpisodesFromTitle()[0].Number = %q, want 1", got[0].Number)
	}
	if got[0].ID != "hash789" {
		t.Fatalf("EpisodesFromTitle()[0].ID = %q, want hash789", got[0].ID)
	}
}

func TestEpisodesFromTitleEmptyHash(t *testing.T) {
	got := EpisodesFromTitle("", "Some Title")
	if got != nil {
		t.Fatalf("EpisodesFromTitle() = %v, want nil", got)
	}
}

// --- Streams Tests ---

func TestStreamsReturnsMagnet(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	got, err := client.Streams(context.Background(), "abcdef1234567890abcdef1234567890abcdef12")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Streams() = %d streams, want 1", len(got))
	}
	if !strings.Contains(got[0].URL, "magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("Streams()[0].URL = %q, want magnet URI", got[0].URL)
	}
	if got[0].Quality != "torrent" {
		t.Fatalf("Streams()[0].Quality = %q, want torrent", got[0].Quality)
	}
	// Verify trackers are present
	if !strings.Contains(got[0].URL, "tracker.opentrackr.org") {
		t.Fatalf("Streams()[0].URL missing tracker: %q", got[0].URL)
	}
	if !strings.Contains(got[0].URL, "open.stealth.si") {
		t.Fatalf("Streams()[0].URL missing stealth tracker: %q", got[0].URL)
	}
}

func TestStreamsIncludesTorrentFileAsMetadataSource(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssSearchResult))
	}))

	results, err := client.Search(context.Background(), "test drama", "drama")
	if err != nil {
		t.Fatal(err)
	}
	streams, err := client.Streams(context.Background(), results[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(streams[0].URL, "xs=https%3A%2F%2Fnyaa.si%2Fdownload%2F") {
		t.Fatalf("magnet missing torrent metadata source: %q", streams[0].URL)
	}
}

func TestStreamsStripsEpisodeSuffix(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	got, err := client.Streams(context.Background(), "abcdef1234567890abcdef1234567890abcdef12:5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got[0].URL, "magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("Streams()[0].URL = %q, want magnet URI with correct hash", got[0].URL)
	}
}

func TestStreamsErrorOnEmptyID(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	_, err := client.Streams(context.Background(), "")
	if err == nil {
		t.Fatal("Streams() error = nil, want error for empty ID")
	}
}

func TestStreamsErrorOnShortHash(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	_, err := client.Streams(context.Background(), "abc")
	if err == nil {
		t.Fatal("Streams() error = nil, want error for short hash")
	}
}

// --- Config Tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != "https://nyaa.si" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Category != CategoryLiveActionSubbed {
		t.Fatalf("Category = %q", cfg.Category)
	}
	if cfg.UserAgent == "" {
		t.Fatal("UserAgent is empty")
	}
	if cfg.Timeout != 60*time.Second {
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

// --- Helper Tests ---

func TestBuildMagnetURI(t *testing.T) {
	got := buildMagnetURI("abc123", []string{"http://tracker.example.com/announce"}, "https://example.com/file.torrent")
	if !strings.Contains(got, "magnet:?xt=urn:btih:abc123") {
		t.Fatalf("buildMagnetURI() = %q, want magnet prefix", got)
	}
	if !strings.Contains(got, "tracker.example.com") {
		t.Fatalf("buildMagnetURI() = %q, want tracker included", got)
	}
	if !strings.Contains(got, "xs=https%3A%2F%2Fexample.com%2Ffile.torrent") {
		t.Fatalf("buildMagnetURI() = %q, want xs metadata source", got)
	}
}

func TestExtractHashFromGUID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://nyaa.si/view/12345", "12345"},
		{"https://nyaa.si/view/12345/", "12345"},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractHashFromGUID(tt.input)
		if got != tt.want {
			t.Errorf("extractHashFromGUID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- HTTP Error Tests ---

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
