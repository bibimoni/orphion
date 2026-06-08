package bettermelon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/provider"
)

// availableProviders is the list of upstream Bettermelon providers in fallback order.
var availableProviders = []string{"hianime", "animekai", "kickassanime", "anikoto"}

// episodeRef encodes the opaque episode identifier used between Episodes and Streams.
type episodeRef struct {
	AniListID string `json:"a"`
	Number    string `json:"e"`
	Provider  string `json:"p"`
}

// redactedRequestError wraps an error without exposing request URLs.
type redactedRequestError struct {
	err error
}

func (e redactedRequestError) Error() string {
	return "request failed"
}

func (e redactedRequestError) Unwrap() error {
	return e.err
}

// Client fetches and normalizes Bettermelon data.
type Client struct {
	httpClient *http.Client // for API calls (short timeout)
	apiURL     *url.URL
	proxyURL   *url.URL
	userAgent  string
	provider   string
}

// NewClient validates configuration and creates a Bettermelon client.
func NewClient(cfg Config) (*Client, error) {
	apiURL, err := parseEndpoint("API", cfg.APIURL)
	if err != nil {
		return nil, err
	}
	proxyURL, err := parseEndpoint("proxy", cfg.ProxyURL)
	if err != nil {
		return nil, err
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = common.DefaultUserAgent
	}
	if cfg.Provider == "" {
		cfg.Provider = common.BettermelonDefaultProvider
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{
		httpClient: cfg.HTTPClient,
		apiURL:     apiURL,
		proxyURL:   proxyURL,
		userAgent:  cfg.UserAgent,
		provider:   cfg.Provider,
	}, nil
}

// Search resolves a query to anime entries.
// Numeric queries are treated as AniList IDs directly.
// Text queries are resolved via the AniList GraphQL API to find matching IDs.
func (c *Client) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	query = strings.TrimSpace(query)

	// Fast path: numeric query is an AniList ID.
	if _, err := strconv.Atoi(query); err == nil {
		return c.searchByID(ctx, query)
	}

	// Text query: resolve via AniList.
	results, err := c.searchAniList(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("bettermelon: text search for %q: %w", query, err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("bettermelon: no results for %q", query)
	}
	return results, nil
}

// searchByID fetches anime info by AniList ID via the Bettermelon stream endpoint.
func (c *Client) searchByID(ctx context.Context, animeID string) ([]provider.Anime, error) {
	var resp streamResponse
	if err := c.fetchStream(ctx, animeID, "1", c.provider, &resp); err != nil {
		return nil, fmt.Errorf("bettermelon search: %w", err)
	}
	title := resp.animeTitle()
	if title == "" {
		title = "AniList #" + animeID
	}
	return []provider.Anime{{ID: animeID, Title: title}}, nil
}

// anilistSearchResult models a single result from the AniList media search.
type anilistSearchResult struct {
	ID    int `json:"id"`
	Title struct {
		English string `json:"english"`
		Romaji  string `json:"romaji"`
		Native  string `json:"native"`
	} `json:"title"`
}

// anilistTitle returns the best available title.
func (r *anilistSearchResult) anilistTitle() string {
	if r.Title.English != "" {
		return r.Title.English
	}
	if r.Title.Romaji != "" {
		return r.Title.Romaji
	}
	return r.Title.Native
}

// searchAniList queries the AniList GraphQL API to resolve text to AniList IDs.
func (c *Client) searchAniList(ctx context.Context, query string) ([]provider.Anime, error) {
	const anilistQuery = `query($search: String, $type: MediaType) { Page(page: 1, perPage: 10) { media(search: $search, type: $type) { id title { english romaji native } } } }`

	payload, err := json.Marshal(map[string]any{
		"query": anilistQuery,
		"variables": map[string]any{
			"search": query,
			"type":   "ANIME",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, common.AniListAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", redactedRequestError{err: err})
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anilist status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var envelope struct {
		Data struct {
			Page struct {
				Media []anilistSearchResult `json:"media"`
			} `json:"Page"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		msgs := make([]string, 0, len(envelope.Errors))
		for _, e := range envelope.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, fmt.Errorf("anilist returned errors: %s", strings.Join(msgs, "; "))
	}

	results := make([]provider.Anime, 0, len(envelope.Data.Page.Media))
	for _, m := range envelope.Data.Page.Media {
		title := m.anilistTitle()
		if title == "" {
			title = "AniList #" + strconv.Itoa(m.ID)
		}
		results = append(results, provider.Anime{
			ID:    strconv.Itoa(m.ID),
			Title: title,
		})
	}
	return results, nil
}

// Episodes returns provider-ordered episodes for a show identified by AniList ID.
func (c *Client) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	var resp episodesResponse
	if err := c.fetchJSON(ctx, "/anime/"+animeID+"/episodes", &resp); err != nil {
		return nil, fmt.Errorf("bettermelon episodes: %w", err)
	}

	episodes := make([]provider.Episode, 0, len(resp.Data.Episodes))
	for _, ep := range resp.Data.Episodes {
		number := ep.number()
		if number == "" {
			continue
		}
		sortKey, err := strconv.ParseFloat(number, 64)
		if err != nil {
			continue
		}
		title := ep.title()
		if title == "" {
			title = "Episode " + number
		}
		episodes = append(episodes, provider.Episode{
			ID:      encodeEpisodeID(episodeRef{AniListID: animeID, Number: number, Provider: c.provider}),
			Number:  number,
			SortKey: sortKey,
			Title:   title,
		})
	}
	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].SortKey < episodes[j].SortKey
	})
	return episodes, nil
}

// Streams resolves an episode ID into downloadable media streams.
// If the primary provider encoded in the episode ID fails, fallback providers are tried.
func (c *Client) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	ref, err := decodeEpisodeID(episodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid episode ID")
	}

	// Build ordered provider list: primary first, then remaining in order.
	providers := providerOrder(ref.Provider)

	var lastErr error
	for _, prov := range providers {
		var resp streamResponse
		if err := c.fetchStream(ctx, ref.AniListID, ref.Number, prov, &resp); err != nil {
			lastErr = err
			continue
		}
		streams := resp.streams(ctx, c)
		if len(streams) > 0 {
			return streams, nil
		}
		lastErr = fmt.Errorf("bettermelon streams: no m3u8 URL in response from provider %q", prov)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("bettermelon streams: %w", lastErr)
	}
	return nil, fmt.Errorf("bettermelon streams: no supported sources")
}

// fetchJSON performs a GET request against the Bettermelon API and decodes the JSON response.
// It retries up to 2 times on transient server errors (5xx).
func (c *Client) fetchJSON(ctx context.Context, path string, out any) error {
	endpoint := c.apiURL.JoinPath(path)

	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request: %w", redactedRequestError{err: err})
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("upstream status %d", resp.StatusCode)
			continue // retry on 5xx
		}

		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("upstream status %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
	return lastErr
}

// fetchStream fetches streaming data for a specific anime/episode/provider combination.
func (c *Client) fetchStream(ctx context.Context, animeID, episodeNum, prov string, out *streamResponse) error {
	path := fmt.Sprintf("/anime/%s/%s/%s", animeID, episodeNum, prov)
	return c.fetchJSON(ctx, path, out)
}

// ── Response types ──────────────────────────────────────────────────────

// episodesResponse models the Kitsu-format episode listing.
type episodesResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Episodes []episodeEntry `json:"episodes"`
	} `json:"data"`
}

type episodeEntry struct {
	ID         string `json:"id"`
	Attributes struct {
		Number         json.Number `json:"number"`
		CanonicalTitle string      `json:"canonicalTitle"`
		Length         int         `json:"length"`
		Thumbnail      struct {
			Original string `json:"original"`
		} `json:"thumbnail"`
	} `json:"attributes"`
}

func (e *episodeEntry) number() string {
	if e.Attributes.Number.String() != "" {
		return e.Attributes.Number.String()
	}
	return ""
}

func (e *episodeEntry) title() string {
	return e.Attributes.CanonicalTitle
}

// streamResponse models the Bettermelon stream endpoint response.
type streamResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Provider string `json:"provider"`
		Anime    struct {
			ID    int `json:"id"`
			Title struct {
				English string `json:"english"`
				Romaji  string `json:"romaji"`
				Native  string `json:"native"`
			} `json:"title"`
			Format      string `json:"format"`
			Status      string `json:"status"`
			BannerImage string `json:"bannerImage"`
			CoverImage  struct {
				Medium     string `json:"medium"`
				Large      string `json:"large"`
				ExtraLarge string `json:"extraLarge"`
			} `json:"coverImage"`
			Episodes int `json:"episodes"`
		} `json:"anime"`
		Episode struct {
			Details struct {
				ID         string `json:"id"`
				Attributes struct {
					Number         json.Number `json:"number"`
					CanonicalTitle string      `json:"canonicalTitle"`
				} `json:"attributes"`
			} `json:"details"`
			Sources struct {
				Type    string `json:"type"`
				Sources struct {
					File string `json:"file"`
				} `json:"sources"`
				Tracks []struct {
					File    string `json:"file"`
					Label   string `json:"label"`
					Kind    string `json:"kind"`
					Default bool   `json:"default"`
				} `json:"tracks"`
				Intro struct {
					Start float64 `json:"start"`
					End   float64 `json:"end"`
				} `json:"intro"`
				Outro struct {
					Start float64 `json:"start"`
					End   float64 `json:"end"`
				} `json:"outro"`
			} `json:"sources"`
		} `json:"episode"`
	} `json:"data"`
}

// animeTitle returns the best available anime title from the stream response.
func (r *streamResponse) animeTitle() string {
	t := r.Data.Anime.Title
	if t.English != "" {
		return t.English
	}
	if t.Romaji != "" {
		return t.Romaji
	}
	return t.Native
}

// streams converts the stream response into provider.Stream objects.
func (r *streamResponse) streams(_ context.Context, client *Client) []provider.Stream {
	return client.buildStreams(r.Data.Episode.Sources.Sources.File)
}

// buildStreams converts a CDN m3u8 URL into a downloadable stream.
// It downloads the m3u8 manifest and all segments to local temp files
// with .ts extensions, so ffmpeg can correctly identify them as MPEG-TS.
// The CDN disguises segments with fake extensions (.jpg, .html, .js, etc.)
// which causes ffmpeg to misidentify the format.
func (c *Client) buildStreams(fileURL string) []provider.Stream {
	if fileURL == "" {
		return nil
	}

	// Try to create a local rewritten m3u8 for ffmpeg.
	localURL := c.rewriteM3U8(fileURL)
	if localURL != "" {
		headers := make(http.Header)
		headers.Set("Referer", "https://bettermelon.ru/")
		headers.Set("User-Agent", c.userAgent)
		return []provider.Stream{{URL: localURL, Quality: "", Headers: headers}}
	}

	// Fallback: return the proxied URL directly.
	proxied := c.proxiedURL(fileURL)
	headers := make(http.Header)
	headers.Set("Referer", "https://bettermelon.ru/")
	headers.Set("User-Agent", c.userAgent)
	return []provider.Stream{{URL: proxied, Quality: "", Headers: headers}}
}

// proxiedURL rewrites a CDN URL through the Bettermelon proxy.
// If the URL is already on the proxy host, it is returned as-is to avoid double-proxying.
func (c *Client) proxiedURL(rawURL string) string {
	if c.proxyURL == nil {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	// Already on the proxy host — don't double-proxy.
	if parsed.Host == c.proxyURL.Host {
		return rawURL
	}
	proxy := *c.proxyURL
	q := proxy.Query()
	q.Set("url", parsed.String())
	proxy.RawQuery = q.Encode()
	proxy.Path = "/proxy"
	return proxy.String()
}

// rewriteM3U8 fetches the m3u8 manifest (and any sub-playlists) via the
// proxy, rewrites segment URLs to point through a local HTTP server that
// serves them with .ts extensions, and writes a local temp m3u8 file.
// Returns a file:// URL or empty string on error.
// All temp files are placed in a single temp directory so cleanupTempInput
// can remove them all at once.
func (c *Client) rewriteM3U8(fileURL string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The manifest is fetched via the proxy, so any relative URLs inside it
	// are relative to the proxy URL — not the original CDN URL.
	// We must pass the proxy URL as the base for URL resolution.
	proxyBase := c.proxiedURL(fileURL)

	manifest, err := c.fetchViaProxy(ctx, fileURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rewriteM3U8: fetchViaProxy failed: %v\n", err)
		return ""
	}

	// Create a temp directory for all m3u8 files (master + sub-playlists).
	tmpDir, err := os.MkdirTemp("", "bettermelon-m3u8")
	if err != nil {
		fmt.Fprintf(os.Stderr, "rewriteM3U8: MkdirTemp failed: %v\n", err)
		return ""
	}

	// Start a local HTTP server that proxies segment requests.
	// Segments are served with .ts extension so ffmpeg identifies them as
	// MPEG-TS, but the server fetches them from the CDN with their original
	// fake extension (.jpg, .html, etc.).
	segMap := &segmentProxyMap{urls: make(map[string]string)}
	srv := httptest.NewServer(segMap.handler(c.httpClient, c.userAgent, c.proxyURL))

	rewritten, err := c.rewritePlaylist(ctx, manifest, proxyBase, tmpDir, srv.URL, segMap)
	if err != nil {
		srv.Close()
		fmt.Fprintf(os.Stderr, "rewriteM3U8: rewritePlaylist failed: %v\n", err)
		_ = os.RemoveAll(tmpDir)
		return ""
	}

	// Write the master playlist.
	masterPath := tmpDir + "/master.m3u8"
	if err := os.WriteFile(masterPath, []byte(rewritten), 0o644); err != nil {
		srv.Close()
		fmt.Fprintf(os.Stderr, "rewriteM3U8: WriteFile failed: %v\n", err)
		_ = os.RemoveAll(tmpDir)
		return ""
	}

	// Register the server so it stays alive until CloseSegmentServers is called
	// after ffmpeg finishes.
	activeSrvMu.Lock()
	activeSrvs[tmpDir] = srv
	activeSrvMu.Unlock()

	return "file://" + masterPath
}

// rewritePlaylist processes an m3u8 playlist: for master playlists it
// rewrites sub-playlist URLs to absolute proxy URLs; for media playlists
// it rewrites segment URLs to point through a local HTTP server that
// serves them with .ts extensions. Sub-playlists are written to tmpDir.
func (c *Client) rewritePlaylist(ctx context.Context, body, sourceURL, tmpDir, localSrv string, segMap *segmentProxyMap) (string, error) {
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(body))
	subIdx := 0
	segIdx := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments/directives.
		if strings.HasPrefix(line, "#") {
			// Handle #EXT-X-KEY or #EXT-X-MAP that may contain URIs.
			if strings.Contains(line, "URI=") {
				line = c.rewriteURIsInDirective(line)
			}
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out.WriteByte('\n')
			continue
		}

		// Resolve relative URLs against the source playlist URL.
		absURL := c.resolveAgainst(trimmed, sourceURL)

		// If this is a sub-playlist reference (.m3u8), fetch and inline-rewrite it.
		// Check the resolved absolute URL for .m3u8 (not the raw line, which may
		// be a relative /proxy?url=... path containing .m3u8 in a query param).
		if strings.HasSuffix(absURL, ".m3u8") || strings.Contains(absURL, ".m3u8?") ||
			strings.Contains(absURL, ".m3u8&") {
			subManifest, err := c.fetchViaProxy(ctx, absURL)
			if err != nil {
				return "", fmt.Errorf("fetch sub-playlist %s: %w", trimmed, err)
			}
			subRewritten, err := c.rewritePlaylist(ctx, subManifest, absURL, tmpDir, localSrv, segMap)
			if err != nil {
				return "", fmt.Errorf("rewrite sub-playlist %s: %w", trimmed, err)
			}
			// Write the sub-playlist to a file in the temp directory.
			subPath := fmt.Sprintf("%s/sub-%d.m3u8", tmpDir, subIdx)
			subIdx++
			if err := os.WriteFile(subPath, []byte(subRewritten), 0o644); err != nil {
				return "", err
			}
			out.WriteString("file://" + subPath)
			out.WriteByte('\n')
			continue
		}

		// It's a segment URL — register it with the local segment proxy
		// so ffmpeg can fetch it with a .ts extension. The CDN uses fake
		// extensions (.jpg, .html) that ffmpeg misidentifies as mjpeg/html.
		segIdx++
		segKey := fmt.Sprintf("%d.ts", segIdx)
		segMap.Register(segKey, absURL)
		out.WriteString(localSrv + "/seg/" + segKey)
		out.WriteByte('\n')
	}
	return out.String(), nil
}

// rewriteURIsInDirective rewrites URI= values in HLS directives (like
// #EXT-X-KEY) to absolute proxy URLs.
func (c *Client) rewriteURIsInDirective(line string) string {
	uriStart := strings.Index(line, `URI="`)
	if uriStart < 0 {
		return line
	}
	uriStart += 5 // skip URI="
	uriEnd := strings.Index(line[uriStart:], `"`)
	if uriEnd < 0 {
		return line
	}
	origURI := line[uriStart : uriStart+uriEnd]
	proxied := c.proxiedURL(origURI)
	return line[:uriStart] + proxied + line[uriStart+uriEnd:]
}

// resolveAgainst resolves a potentially relative URL against a base URL.
func (c *Client) resolveAgainst(ref, base string) string {
	// If ref is already absolute, return as-is.
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	baseParsed, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refParsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseParsed.ResolveReference(refParsed).String()
}

// fetchViaProxy fetches a URL through the Bettermelon proxy.
func (c *Client) fetchViaProxy(ctx context.Context, rawURL string) (string, error) {
	proxied := c.proxiedURL(rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxied, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", "https://bettermelon.ru/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// activeServers tracks local HTTP servers created for segment proxying,
// keyed by temp directory path. They are closed by CloseSegmentServers.
var (
	activeSrvMu sync.Mutex
	activeSrvs  = make(map[string]*httptest.Server)
)

// CloseSegmentServers closes the local HTTP server(s) associated with the
// given stream URL's temp directory. Called by the service layer after
// ffmpeg finishes downloading.
func CloseSegmentServers(streamURL string) {
	if !strings.HasPrefix(streamURL, "file://") {
		return
	}
	path := streamURL[len("file://"):]
	dir := filepath.Dir(path)
	if !strings.Contains(filepath.Base(dir), "bettermelon-m3u8") {
		return
	}
	activeSrvMu.Lock()
	srv, ok := activeSrvs[dir]
	if ok {
		delete(activeSrvs, dir)
	}
	activeSrvMu.Unlock()
	if ok {
		srv.Close()
	}
}

// segmentProxyMap maps .ts segment keys (e.g. "1.ts") to their real CDN URLs.
// It also serves as an http.Handler that proxies segment requests, fetching
// the real URL through the Bettermelon proxy and streaming the response back
// to ffmpeg (which sees .ts extension and correctly identifies MPEG-TS).
type segmentProxyMap struct {
	mu   sync.RWMutex
	urls map[string]string // key -> real proxy URL
}

// Register adds a segment key -> real URL mapping.
func (m *segmentProxyMap) Register(key, realURL string) {
	m.mu.Lock()
	m.urls[key] = realURL
	m.mu.Unlock()
}

// handler returns an http.Handler that serves segments on-demand.
// When ffmpeg requests /seg/1.ts, the handler looks up the real URL
// and proxies the request through the Bettermelon proxy.
func (m *segmentProxyMap) handler(httpClient *http.Client, userAgent string, proxyURL *url.URL) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract segment key from path: /seg/1.ts → "1.ts"
		key := strings.TrimPrefix(r.URL.Path, "/seg/")
		if key == "" {
			http.NotFound(w, r)
			return
		}

		m.mu.RLock()
		realURL, ok := m.urls[key]
		m.mu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		// Build the proxy URL for this segment.
		proxied := proxiedURLFromURL(realURL, proxyURL)

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, proxied, nil)
		if err != nil {
			http.Error(w, "bad request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Accept", "*/*")
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Referer", "https://bettermelon.ru/")

		resp, err := httpClient.Do(req)
		if err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, "upstream error", resp.StatusCode)
			return
		}

		// Stream the segment data back to ffmpeg.
		w.Header().Set("Content-Type", "video/mp2t")
		if resp.ContentLength > 0 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
		}
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 50*1024*1024)) // 50MB max
	}
}

// proxiedURLFromURL builds a proxy URL from a real URL and proxy base.
// This is a standalone version of Client.proxiedURL that doesn't need a Client.
func proxiedURLFromURL(rawURL string, proxyURL *url.URL) string {
	if proxyURL == nil {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	if parsed.Host == proxyURL.Host {
		return rawURL
	}
	proxy := *proxyURL
	q := proxy.Query()
	q.Set("url", parsed.String())
	proxy.RawQuery = q.Encode()
	proxy.Path = "/proxy"
	return proxy.String()
}

// ── Episode ID encoding ─────────────────────────────────────────────────

func encodeEpisodeID(ref episodeRef) string {
	data, _ := json.Marshal(ref)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeEpisodeID(raw string) (episodeRef, error) {
	var ref episodeRef
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return ref, err
	}
	if err := json.Unmarshal(data, &ref); err != nil {
		return ref, err
	}
	if ref.AniListID == "" || ref.Number == "" || ref.Provider == "" {
		return ref, errors.New("incomplete episode ID")
	}
	return ref, nil
}

// providerOrder returns the provider list with the preferred provider first,
// followed by remaining providers in their standard order.
func providerOrder(preferred string) []string {
	seen := make(map[string]bool, len(availableProviders))
	result := make([]string, 0, len(availableProviders))

	// Try preferred provider first.
	for _, p := range availableProviders {
		if p == preferred && !seen[p] {
			result = append(result, p)
			seen[p] = true
		}
	}
	// Append remaining providers.
	for _, p := range availableProviders {
		if !seen[p] {
			result = append(result, p)
			seen[p] = true
		}
	}
	return result
}
