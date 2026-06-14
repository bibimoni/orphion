package bettermelon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bibimoni/orphion/internal/provider"
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
			if strings.Contains(urlParam, "seg1.jpg") {
				return jsonResponse(http.StatusOK, "mpeg-ts-data"), nil
			}
			if strings.Contains(urlParam, "index-") {
				// Sub-playlist with segments using fake extensions.
				return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\n/proxy?url=https://cdn.example.test/seg1.jpg\n#EXT-X-ENDLIST"), nil
			}
			// Master playlist pointing to sub-playlist.
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1800000,RESOLUTION=1920x1080\n/proxy?url=https://cdn.example.test/anime/index-f1.m3u8\n"), nil
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
	if strings.HasPrefix(got[0].URL, "file://") {
		t.Fatalf("URL = %q, stream should not be prepared before selection", got[0].URL)
	}
	if got[0].Quality != "1080p" {
		t.Fatalf("Quality = %q, want 1080p", got[0].Quality)
	}
	if got[0].Headers.Get("Referer") == "" {
		t.Fatal("missing Referer header")
	}
	if got[0].Headers.Get("User-Agent") == "" {
		t.Fatal("missing User-Agent header")
	}

	prepared, err := client.PrepareStream(context.Background(), got[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		path := strings.TrimPrefix(prepared.URL, "file://")
		_ = os.RemoveAll(filepath.Dir(path))
	})
	if !strings.HasPrefix(prepared.URL, "file://") {
		t.Fatalf("prepared URL = %q, want local playlist", prepared.URL)
	}
}

func TestStreamsExposesMasterPlaylistQualitiesWithoutFetchingVariants(t *testing.T) {
	proxyRequests := 0
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/anime/167152/1/hianime":
			return jsonResponse(http.StatusOK, `{
				"success": true,
				"data": {
					"provider": "hianime",
					"anime": {"id": 167152, "title": {"english": "Sentenced to Be a Hero"}},
					"episode": {
						"sources": {
							"type": "SOFT",
							"sources": {"file": "https://cdn.example.test/master.m3u8"}
						}
					}
				}
			}`), nil
		case "/proxy":
			proxyRequests++
			if req.URL.Query().Get("url") != "https://cdn.example.test/master.m3u8" {
				t.Fatalf("fetched unselected variant: %s", req.URL.Query().Get("url"))
			}
			return jsonResponse(http.StatusOK, `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1588679,RESOLUTION=1920x1080
/proxy?url=https://cdn.example.test/1080.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=918351,RESOLUTION=1280x720
/proxy?url=https://cdn.example.test/720.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=460382,RESOLUTION=640x360
/proxy?url=https://cdn.example.test/360.m3u8
`), nil
		default:
			t.Fatalf("unexpected path = %s", req.URL.Path)
			return nil, nil
		}
	})

	episodeID := encodeEpisodeID(episodeRef{AniListID: "167152", Number: "1", Provider: "hianime"})
	got, err := client.Streams(context.Background(), episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if proxyRequests != 1 {
		t.Fatalf("master playlist requests = %d, want 1", proxyRequests)
	}
	if len(got) != 3 {
		t.Fatalf("len(Streams()) = %d, want 3 variants", len(got))
	}
	wantQualities := []string{"1080p", "720p", "360p"}
	wantBandwidths := []int64{1588679, 918351, 460382}
	for i, want := range wantQualities {
		if got[i].Quality != want {
			t.Fatalf("Streams()[%d].Quality = %q, want %q", i, got[i].Quality, want)
		}
		if strings.HasPrefix(got[i].URL, "file://") {
			t.Fatalf("Streams()[%d].URL = %q, variant should be prepared only after selection", i, got[i].URL)
		}
		if got[i].Bandwidth != wantBandwidths[i] {
			t.Fatalf("Streams()[%d].Bandwidth = %d, want %d", i, got[i].Bandwidth, wantBandwidths[i])
		}
	}
}

func TestStreamsParsesBandwidthFromManifest(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected request")
		return nil, nil
	})
	headers := make(http.Header)
	manifest := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=7780750,RESOLUTION=1920x1080
/proxy?url=https://cdn.example.test/1080.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=918351,RESOLUTION=1280x720
/proxy?url=https://cdn.example.test/720.m3u8
`

	got := client.masterVariants(manifest, "https://api.example.test/proxy?url=master", headers)
	if len(got) != 2 {
		t.Fatalf("len(masterVariants()) = %d, want 2", len(got))
	}
	if got[0].Bandwidth != 7780750 {
		t.Fatalf("1080p Bandwidth = %d, want 7780750", got[0].Bandwidth)
	}
	if got[1].Bandwidth != 918351 {
		t.Fatalf("720p Bandwidth = %d, want 918351", got[1].Bandwidth)
	}
}

func TestMasterVariantsPreserveDefaultAudioRendition(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected request")
		return nil, nil
	})
	headers := make(http.Header)
	manifest := `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="stereo",NAME="Japanese",DEFAULT=YES,LANGUAGE="jpn",URI="/proxy?url=https%3A%2F%2Fcdn.example.test%2Faudio-jpn.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="stereo",NAME="English",LANGUAGE="eng",URI="/proxy?url=https%3A%2F%2Fcdn.example.test%2Faudio-eng.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=7780750,CODECS="avc1.4D4C28,mp4a.40.2",RESOLUTION=1920x1080,AUDIO="stereo"
/proxy?url=https%3A%2F%2Fcdn.example.test%2Fvideo-1080.m3u8
`

	got := client.masterVariants(manifest, "https://api.example.test/proxy?url=master", headers)
	if len(got) != 1 {
		t.Fatalf("len(masterVariants()) = %d, want 1", len(got))
	}
	want := "https://api.example.test/proxy?url=https%3A%2F%2Fcdn.example.test%2Faudio-jpn.m3u8"
	if got[0].AudioURL != want {
		t.Fatalf("AudioURL = %q, want %q", got[0].AudioURL, want)
	}
	if got[0].Bandwidth != 7780750 {
		t.Fatalf("Bandwidth = %d, want 7780750", got[0].Bandwidth)
	}
}

func TestBandwidthFromStreamInfo(t *testing.T) {
	tests := []struct {
		name       string
		attributes string
		want       int64
	}{
		{"standard", "BANDWIDTH=1588679,RESOLUTION=1920x1080", 1588679},
		{"at_start", "BANDWIDTH=8000000,CODECS=\"avc1\"", 8000000},
		{"at_end", "RESOLUTION=1920x1080,BANDWIDTH=4500000", 4500000},
		{"missing", "RESOLUTION=1920x1080", 0},
		{"empty", "", 0},
		{"invalid", "BANDWIDTH=notanumber", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bandwidthFromStreamInfo(tt.attributes)
			if got != tt.want {
				t.Errorf("bandwidthFromStreamInfo(%q) = %d, want %d", tt.attributes, got, tt.want)
			}
		})
	}
}

func TestMasterVariantsParsesBandwidth(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("unexpected request")
		return nil, nil
	})
	headers := make(http.Header)
	manifest := `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=7780750,RESOLUTION=1920x1080
https://cdn.example.test/1080-high.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=1920x1080
https://cdn.example.test/1080-low.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=918351,RESOLUTION=1280x720
https://cdn.example.test/720.m3u8
`

	got := client.masterVariants(manifest, "https://cdn.example.test/master.m3u8", headers)
	if len(got) != 3 {
		t.Fatalf("len(masterVariants()) = %d, want 3", len(got))
	}

	// Verify bandwidth is parsed correctly for each variant.
	if got[0].Bandwidth != 7780750 {
		t.Errorf("variant 0 Bandwidth = %d, want 7780750", got[0].Bandwidth)
	}
	if got[0].Quality != "1080p" {
		t.Errorf("variant 0 Quality = %q, want 1080p", got[0].Quality)
	}
	if got[1].Bandwidth != 1500000 {
		t.Errorf("variant 1 Bandwidth = %d, want 1500000", got[1].Bandwidth)
	}
	if got[1].Quality != "1080p" {
		t.Errorf("variant 1 Quality = %q, want 1080p", got[1].Quality)
	}
	if got[2].Bandwidth != 918351 {
		t.Errorf("variant 2 Bandwidth = %d, want 918351", got[2].Bandwidth)
	}
}

func TestPrepareStreamDownloadsSegmentsInParallel(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var progressMu sync.Mutex
	var progressDone, progressTotal int

	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/proxy" {
			t.Fatalf("unexpected path = %s", req.URL.Path)
		}
		rawURL := req.URL.Query().Get("url")
		if strings.HasSuffix(rawURL, "1080.m3u8") {
			return jsonResponse(http.StatusOK, `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:10,
https://cdn.example.test/seg-1.jpg
#EXTINF:10,
https://cdn.example.test/seg-2.jpg
#EXTINF:10,
https://cdn.example.test/seg-3.jpg
#EXTINF:10,
https://cdn.example.test/seg-4.jpg
#EXT-X-ENDLIST
`), nil
		}
		if strings.Contains(rawURL, "/seg-") {
			current := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				seen := maxInFlight.Load()
				if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
					break
				}
			}
			time.Sleep(25 * time.Millisecond)
			return jsonResponse(http.StatusOK, "mpeg-ts-"+filepath.Base(rawURL)), nil
		}
		t.Fatalf("unexpected proxy URL = %s", rawURL)
		return nil, nil
	})

	prepared, err := client.PrepareStream(
		context.Background(),
		provider.Stream{URL: "https://cdn.example.test/1080.m3u8", Quality: "1080p"},
		func(done, total int) {
			progressMu.Lock()
			progressDone, progressTotal = done, total
			progressMu.Unlock()
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		path := strings.TrimPrefix(prepared.URL, "file://")
		_ = os.RemoveAll(filepath.Dir(path))
	})

	if maxInFlight.Load() < 2 {
		t.Fatalf("maximum concurrent downloads = %d, want parallel segment fetching", maxInFlight.Load())
	}
	progressMu.Lock()
	if progressDone != 4 || progressTotal != 4 {
		t.Fatalf("final progress = %d/%d, want 4/4", progressDone, progressTotal)
	}
	progressMu.Unlock()

	playlistPath := strings.TrimPrefix(prepared.URL, "file://")
	playlist, err := os.ReadFile(playlistPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 4; i++ {
		segmentPath := filepath.Join(filepath.Dir(playlistPath), fmt.Sprintf("segment-%05d.ts", i))
		if _, err := os.Stat(segmentPath); err != nil {
			t.Fatalf("segment %d missing: %v\nplaylist:\n%s", i, err, playlist)
		}
		if !strings.Contains(string(playlist), filepath.Base(segmentPath)) {
			t.Fatalf("playlist does not reference %s:\n%s", filepath.Base(segmentPath), playlist)
		}
	}
}

func TestPrepareStreamMaterializesVideoAndAudio(t *testing.T) {
	var audioPlaylistRequests atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		rawURL := req.URL.Query().Get("url")
		switch {
		case strings.HasSuffix(rawURL, "video.m3u8"):
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXTINF:10,\nhttps://cdn.example.test/video-seg.jpg\n#EXT-X-ENDLIST\n"), nil
		case strings.HasSuffix(rawURL, "audio.m3u8"):
			audioPlaylistRequests.Add(1)
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXTINF:10,\nhttps://cdn.example.test/audio-seg.jpg\n#EXT-X-ENDLIST\n"), nil
		case strings.HasSuffix(rawURL, "video-seg.jpg"):
			return jsonResponse(http.StatusOK, "video-data"), nil
		case strings.HasSuffix(rawURL, "audio-seg.jpg"):
			return jsonResponse(http.StatusOK, "audio-data"), nil
		default:
			t.Fatalf("unexpected proxy URL = %s", rawURL)
			return nil, nil
		}
	})

	var progressDone, progressTotal int
	prepared, err := client.PrepareStream(
		context.Background(),
		provider.Stream{
			URL:      "https://cdn.example.test/video.m3u8",
			AudioURL: "https://cdn.example.test/audio.m3u8",
			Quality:  "1080p",
		},
		func(done, total int) {
			progressDone, progressTotal = done, total
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		path := strings.TrimPrefix(prepared.URL, "file://")
		_ = os.RemoveAll(filepath.Dir(path))
	})

	masterPath := strings.TrimPrefix(prepared.URL, "file://")
	master, err := os.ReadFile(masterPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(master), "#EXT-X-MEDIA:TYPE=AUDIO") {
		t.Fatalf("prepared playlist has no audio rendition:\n%s", master)
	}
	if !strings.Contains(string(master), "video/media.m3u8") ||
		!strings.Contains(string(master), "audio/media.m3u8") {
		t.Fatalf("prepared master does not reference localized tracks:\n%s", master)
	}
	if audioPlaylistRequests.Load() != 1 {
		t.Fatalf("audio playlist requests = %d, want 1", audioPlaylistRequests.Load())
	}
	if progressDone != 2 || progressTotal != 2 {
		t.Fatalf("final progress = %d/%d, want 2/2", progressDone, progressTotal)
	}

	root := filepath.Dir(masterPath)
	for _, path := range []string{
		filepath.Join(root, "video", "segment-00001.ts"),
		filepath.Join(root, "audio", "segment-00001.ts"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("localized resource missing at %s: %v", path, err)
		}
	}
}

func TestPrepareStreamRetriesTransientResourceFailures(t *testing.T) {
	segmentAttempts := 0
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		rawURL := req.URL.Query().Get("url")
		switch {
		case strings.HasSuffix(rawURL, "1080.m3u8"):
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXTINF:10,\nhttps://cdn.example.test/seg-1.jpg\n#EXT-X-ENDLIST\n"), nil
		case strings.HasSuffix(rawURL, "seg-1.jpg"):
			segmentAttempts++
			if segmentAttempts <= 4 {
				return jsonResponse(http.StatusInternalServerError, "temporary"), nil
			}
			return jsonResponse(http.StatusOK, "mpeg-ts-data"), nil
		case req.URL.Host == "cdn.example.test" && strings.HasSuffix(req.URL.Path, "seg-1.jpg"):
			return jsonResponse(http.StatusInternalServerError, "temporary"), nil
		default:
			t.Fatalf("unexpected proxy URL = %s", rawURL)
			return nil, nil
		}
	})

	prepared, err := client.PrepareStream(
		context.Background(),
		provider.Stream{URL: "https://cdn.example.test/1080.m3u8", Quality: "1080p"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		path := strings.TrimPrefix(prepared.URL, "file://")
		_ = os.RemoveAll(filepath.Dir(path))
	})
	if segmentAttempts != 5 {
		t.Fatalf("segment attempts = %d, want retries after transient 500s", segmentAttempts)
	}
}

func TestPrepareStreamRetriesCDNForbiddenBeforeGivingUp(t *testing.T) {
	// Verify that a 403 from the CDN is retried a limited number of times
	// (maxCDNForbiddenRetries) before failing, rather than all 6 retries.
	var segmentAttempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		rawURL := req.URL.Query().Get("url")
		switch {
		case strings.HasSuffix(rawURL, "1080.m3u8"):
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXTINF:10,\nhttps://cdn.example.test/seg-1.jpg\n#EXT-X-ENDLIST\n"), nil
		case strings.HasSuffix(rawURL, "seg-1.jpg"):
			segmentAttempts.Add(1)
			return jsonResponse(http.StatusForbidden, "forbidden"), nil
		case req.URL.Host == "cdn.example.test" && strings.HasSuffix(req.URL.Path, "seg-1.jpg"):
			return jsonResponse(http.StatusForbidden, "forbidden"), nil
		default:
			t.Fatalf("unexpected proxy URL = %s", rawURL)
			return nil, nil
		}
	})

	_, err := client.PrepareStream(
		context.Background(),
		provider.Stream{URL: "https://cdn.example.test/1080.m3u8", Quality: "1080p"},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for permanently forbidden segment")
	}
	// Each downloadResource attempt may try both proxy and direct (2 requests).
	// With maxCDNForbiddenRetries=2 and maxPlaylistRefreshes=3, the total
	// should be bounded well below the 6×2=12+ requests that full retries
	// would generate. 8 = 2 (initial proxy+direct) × (2 CDN retries + 2 playlist refreshes).
	if segmentAttempts.Load() > 10 {
		t.Fatalf("segment attempts = %d, want bounded retries (not full 6-attempt backoff)", segmentAttempts.Load())
	}
}

func TestPrepareStreamRejectsIncompleteResources(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		rawURL := req.URL.Query().Get("url")
		switch {
		case strings.HasSuffix(rawURL, "1080.m3u8"):
			return jsonResponse(http.StatusOK, `#EXTM3U
#EXTINF:10,
https://cdn.example.test/good.jpg
#EXTINF:10,
https://cdn.example.test/missing.jpg
#EXT-X-ENDLIST
`), nil
		case strings.HasSuffix(rawURL, "good.jpg"):
			return jsonResponse(http.StatusOK, "video-data"), nil
		case strings.HasSuffix(rawURL, "missing.jpg"), strings.HasSuffix(req.URL.Path, "missing.jpg"):
			return jsonResponse(http.StatusForbidden, "forbidden"), nil
		default:
			t.Fatalf("unexpected request URL = %s", req.URL.String())
			return nil, nil
		}
	})

	prepared, err := client.PrepareStream(
		context.Background(),
		provider.Stream{URL: "https://cdn.example.test/1080.m3u8", Quality: "1080p"},
		nil,
	)
	if err == nil {
		path := strings.TrimPrefix(prepared.URL, "file://")
		_ = os.RemoveAll(filepath.Dir(path))
		t.Fatal("PrepareStream() succeeded with a permanently missing segment")
	}
	if prepared.URL != "" {
		t.Fatalf("prepared URL = %q, want empty on failure", prepared.URL)
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
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1800000,RESOLUTION=1920x1080\n/proxy?url=https://cdn.example.test/index-f1.m3u8\n"), nil
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
	if len(got) != 1 || got[0].Quality != "1080p" {
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
	if got[0].URL != "https://cdn.mewstream.buzz/anime/abc123/master.m3u8" {
		t.Fatalf("URL = %q, want original media URL for deferred preparation", got[0].URL)
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
	if got[0].URL != "https://api.example.test/proxy?url=https%3A%2F%2Fcdn.example.test%2Fmaster.m3u8" {
		t.Fatalf("URL = %q, already-proxied URL changed", got[0].URL)
	}
}

func TestFetchViaProxyRetriesOn5xx(t *testing.T) {
	var attempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		n := attempts.Add(1)
		if n <= 2 {
			return jsonResponse(http.StatusServiceUnavailable, "temporarily unavailable"), nil
		}
		return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\nseg1.ts\n#EXT-X-ENDLIST"), nil
	})

	got, err := client.fetchViaProxy(context.Background(), "https://cdn.example.test/playlist.m3u8")
	if err != nil {
		t.Fatalf("fetchViaProxy() error = %v", err)
	}
	if !strings.Contains(got, "#EXTM3U") {
		t.Fatalf("unexpected body = %q", got)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestFetchViaProxyFailsAfterMaxRetries(t *testing.T) {
	var attempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return jsonResponse(521, "web server is down"), nil
	})

	_, err := client.fetchViaProxy(context.Background(), "https://cdn.example.test/playlist.m3u8")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "521") {
		t.Fatalf("error = %v, want 521 status", err)
	}
	if attempts.Load() != 5 {
		t.Fatalf("attempts = %d, want 5", attempts.Load())
	}
}

func TestFetchViaProxyDoesNotRetry4xx(t *testing.T) {
	var attempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return jsonResponse(http.StatusForbidden, "forbidden"), nil
	})

	_, err := client.fetchViaProxy(context.Background(), "https://cdn.example.test/playlist.m3u8")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %v, want 403 status", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1 (no retry on 4xx)", attempts.Load())
	}
}

func TestDownloadResourceFallsBackToDirectOnProxy403(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "segment-00001.ts")

	var proxyAttempts, directAttempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		// Requests to the proxy host return 403; direct CDN requests succeed.
		if strings.Contains(req.URL.Host, "example.test") {
			proxyAttempts.Add(1)
			return jsonResponse(http.StatusForbidden, "forbidden"), nil
		}
		directAttempts.Add(1)
		return jsonResponse(http.StatusOK, "segment-data"), nil
	})

	retry, err := client.downloadResourceOnce(context.Background(), localResource{
		url:  "https://cdn.bettermelon.test/seg1.jpg",
		path: path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retry {
		t.Fatal("retry = true, want false (success)")
	}
	if proxyAttempts.Load() != 1 {
		t.Fatalf("proxy attempts = %d, want 1", proxyAttempts.Load())
	}
	if directAttempts.Load() != 1 {
		t.Fatalf("direct attempts = %d, want 1 (fallback)", directAttempts.Load())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "segment-data" {
		t.Fatalf("file content = %q, want %q", data, "segment-data")
	}
}

func TestDownloadResourceUnwrapsProxyURLForDirectFallback(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "segment-00001.ts")

	var proxyAttempts, directAttempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "api.example.test" {
			proxyAttempts.Add(1)
			return jsonResponse(http.StatusForbidden, "forbidden"), nil
		}
		if req.URL.Host == "cdn.bettermelon.test" {
			directAttempts.Add(1)
			return jsonResponse(http.StatusOK, "segment-data"), nil
		}
		t.Fatalf("unexpected host = %s", req.URL.Host)
		return nil, nil
	})

	proxied := client.proxiedURL("https://cdn.bettermelon.test/seg1.jpg")
	retry, err := client.downloadResourceOnce(context.Background(), localResource{
		url:  proxied,
		path: path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retry {
		t.Fatal("retry = true, want false (success)")
	}
	if proxyAttempts.Load() != 1 {
		t.Fatalf("proxy attempts = %d, want 1", proxyAttempts.Load())
	}
	if directAttempts.Load() != 1 {
		t.Fatalf("direct attempts = %d, want 1", directAttempts.Load())
	}
}

func TestDownloadResourceFallsBackToDirectOnProxy5xx(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "segment-00001.ts")

	var proxyAttempts, directAttempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "api.example.test" {
			proxyAttempts.Add(1)
			return jsonResponse(521, "web server is down"), nil
		}
		if req.URL.Host == "cdn.bettermelon.test" {
			directAttempts.Add(1)
			return jsonResponse(http.StatusOK, "segment-data"), nil
		}
		t.Fatalf("unexpected host = %s", req.URL.Host)
		return nil, nil
	})

	retry, err := client.downloadResourceOnce(context.Background(), localResource{
		url:  "https://cdn.bettermelon.test/seg1.jpg",
		path: path,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retry {
		t.Fatal("retry = true, want false (success)")
	}
	if proxyAttempts.Load() != 1 {
		t.Fatalf("proxy attempts = %d, want 1", proxyAttempts.Load())
	}
	if directAttempts.Load() != 1 {
		t.Fatalf("direct attempts = %d, want 1", directAttempts.Load())
	}
}

func TestDownloadResourceNoDirectFallbackWithoutProxy(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "segment-00001.ts")

	var attempts atomic.Int32
	// Create a client with no proxy URL.
	cfg := DefaultConfig()
	cfg.APIURL = "https://api.example.test"
	cfg.ProxyURL = "" // no proxy
	cfg.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return jsonResponse(http.StatusForbidden, "forbidden"), nil
	})}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.downloadResourceOnce(context.Background(), localResource{
		url:  "https://cdn.example.test/seg1.jpg",
		path: path,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Without proxy, there's no fallback — only one attempt.
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1 (no fallback without proxy)", attempts.Load())
	}
}

func TestFetchPlaylistUnwrapsProxyURLForDirectFallback(t *testing.T) {
	var proxyAttempts, directAttempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "api.example.test" {
			proxyAttempts.Add(1)
			return jsonResponse(521, "web server is down"), nil
		}
		if req.URL.Host == "cdn.bettermelon.test" {
			directAttempts.Add(1)
			return jsonResponse(http.StatusOK, "#EXTM3U\n#EXTINF:10,\nseg1.ts\n"), nil
		}
		t.Fatalf("unexpected host = %s", req.URL.Host)
		return nil, nil
	})

	proxied := client.proxiedURL("https://cdn.bettermelon.test/playlist.m3u8")
	playlist, baseURL, err := client.fetchPlaylist(context.Background(), proxied)
	if err != nil {
		t.Fatalf("fetchPlaylist() error = %v", err)
	}
	if !strings.Contains(playlist, "#EXTM3U") {
		t.Fatalf("unexpected playlist = %q", playlist)
	}
	if baseURL != "https://cdn.bettermelon.test/playlist.m3u8" {
		t.Fatalf("baseURL = %q, want unwrapped CDN URL", baseURL)
	}
	if proxyAttempts.Load() != 5 {
		t.Fatalf("proxy attempts = %d, want 5", proxyAttempts.Load())
	}
	if directAttempts.Load() != 1 {
		t.Fatalf("direct attempts = %d, want 1", directAttempts.Load())
	}
}

func TestFetchPlaylistFallsBackToDirectOnProxy5xx(t *testing.T) {
	var proxyAttempts, directAttempts atomic.Int32
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		// Requests to the proxy host (containing "proxy" in the URL path)
		// return 521; direct CDN requests succeed.
		if strings.Contains(req.URL.Path, "proxy") || strings.Contains(req.URL.RawQuery, "proxy") {
			proxyAttempts.Add(1)
			return jsonResponse(521, "web server is down"), nil
		}
		directAttempts.Add(1)
		return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\nseg1.ts\n#EXT-X-ENDLIST"), nil
	})

	// Simulate proxy being down — fetchViaProxy will retry 5 times on 521,
	// then fetchPlaylist falls back to direct.
	playlist, baseURL, err := client.fetchPlaylist(context.Background(), "https://cdn.bettermelon.test/playlist.m3u8")
	if err != nil {
		t.Fatalf("fetchPlaylist() error = %v", err)
	}
	if !strings.Contains(playlist, "#EXTM3U") {
		t.Fatalf("unexpected playlist = %q", playlist)
	}
	// Base URL should be the raw URL (not proxied) since we fell back to direct.
	if baseURL != "https://cdn.bettermelon.test/playlist.m3u8" {
		t.Fatalf("baseURL = %q, want raw CDN URL", baseURL)
	}
	if directAttempts.Load() < 1 {
		t.Fatalf("direct attempts = %d, want at least 1", directAttempts.Load())
	}
}

func TestFetchPlaylistReturnsProxiedBaseURLOnProxySuccess(t *testing.T) {
	client := testClient(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10,\nseg1.ts\n#EXT-X-ENDLIST"), nil
	})

	playlist, baseURL, err := client.fetchPlaylist(context.Background(), "https://cdn.bettermelon.test/playlist.m3u8")
	if err != nil {
		t.Fatalf("fetchPlaylist() error = %v", err)
	}
	if !strings.Contains(playlist, "#EXTM3U") {
		t.Fatalf("unexpected playlist = %q", playlist)
	}
	// When proxy succeeds, base URL should be proxied.
	if !strings.Contains(baseURL, "proxy") {
		t.Fatalf("baseURL = %q, want proxied URL", baseURL)
	}
}
