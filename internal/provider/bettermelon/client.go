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
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bibimoni/orphion/internal/common"
	"github.com/bibimoni/orphion/internal/provider"
)

// availableProviders is the list of upstream Bettermelon providers in fallback order.
var availableProviders = []string{"hianime", "animekai", "kickassanime", "anikoto"}

const (
	maxSegmentSize = 64 << 20
)

var resolutionPattern = regexp.MustCompile(`(?i)(?:^|,)RESOLUTION=\d+x(\d+)(?:,|$)`)

// urlRedactionRe matches URLs embedded in Go http error messages.
// Typical format: Get "https://example.test/path?signed=secret": reason
var urlRedactionRe = regexp.MustCompile(`"https?://[^"]+"`)

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
	if e.err != nil {
		msg := e.err.Error()
		// Strip any URLs that may be embedded in the underlying error
		// (e.g. "Get \"https://...\": connection refused").
		msg = urlRedactionRe.ReplaceAllString(msg, "<redacted>")
		return fmt.Sprintf("request failed: %s", msg)
	}
	return "request failed"
}

func (e redactedRequestError) Unwrap() error {
	return e.err
}

// Client fetches and normalizes Bettermelon data.
type Client struct {
	httpClient     *http.Client // for API calls (short timeout)
	apiURL         *url.URL
	proxyURL       *url.URL
	userAgent      string
	provider       string
	segmentWorkers int
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
	if cfg.SegmentWorkers < 1 {
		cfg.SegmentWorkers = common.DefaultSegmentWorkers
	}
	if cfg.SegmentWorkers > common.MaxSegmentWorkers {
		cfg.SegmentWorkers = common.MaxSegmentWorkers
	}
	return &Client{
		httpClient:     cfg.HTTPClient,
		apiURL:         apiURL,
		proxyURL:       proxyURL,
		userAgent:      cfg.UserAgent,
		provider:       cfg.Provider,
		segmentWorkers: cfg.SegmentWorkers,
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
func (r *streamResponse) streams(ctx context.Context, client *Client) []provider.Stream {
	return client.buildStreams(ctx, r.Data.Episode.Sources.Sources.File)
}

// buildStreams fetches only the master manifest and exposes its quality
// variants. The selected media playlist is prepared later by PrepareStream,
// avoiding eager requests and downloads for qualities the user did not choose.
func (c *Client) buildStreams(ctx context.Context, fileURL string) []provider.Stream {
	if fileURL == "" {
		return nil
	}

	headers := make(http.Header)
	headers.Set("Referer", "https://bettermelon.ru/")
	headers.Set("User-Agent", c.userAgent)

	manifest, err := c.fetchViaProxy(ctx, fileURL)
	if err == nil {
		variants := c.masterVariants(manifest, c.proxiedURL(fileURL), headers)
		if len(variants) > 0 {
			return variants
		}
	}

	// A media playlist has no variants. Keep its original URL so the selected
	// stream can be prepared through the proxy after quality selection.
	return []provider.Stream{{URL: fileURL, Quality: "", Headers: headers}}
}

func (c *Client) masterVariants(manifest, sourceURL string, headers http.Header) []provider.Stream {
	scanner := bufio.NewScanner(strings.NewReader(manifest))
	pendingQuality := ""
	var variants []provider.Stream

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			pendingQuality = qualityFromStreamInfo(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || pendingQuality == "" {
			continue
		}
		variants = append(variants, provider.Stream{
			URL:     c.resolveAgainst(line, sourceURL),
			Quality: pendingQuality,
			Headers: headers.Clone(),
		})
		pendingQuality = ""
	}

	return variants
}

func qualityFromStreamInfo(attributes string) string {
	match := resolutionPattern.FindStringSubmatch(attributes)
	if len(match) != 2 {
		return ""
	}
	return match[1] + "p"
}

// PrepareStream downloads the selected media playlist's resources in
// parallel, rewrites them to local files, and returns a local playlist.
func (c *Client) PrepareStream(ctx context.Context, stream provider.Stream, progress provider.SegmentProgressFunc) (provider.Stream, error) {
	playlist, err := c.fetchViaProxy(ctx, stream.URL)
	if err != nil {
		return provider.Stream{}, fmt.Errorf("fetch media playlist: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "bettermelon-m3u8")
	if err != nil {
		return provider.Stream{}, fmt.Errorf("create stream cache: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	rewritten, resources, err := c.localizeMediaPlaylist(playlist, c.proxiedURL(stream.URL), tmpDir)
	if err != nil {
		cleanup()
		return provider.Stream{}, err
	}
	if len(resources) == 0 {
		cleanup()
		return provider.Stream{}, fmt.Errorf("media playlist contains no downloadable resources")
	}

	if err := c.downloadResources(ctx, resources, progress); err != nil {
		cleanup()
		return provider.Stream{}, err
	}

	playlistPath := filepath.Join(tmpDir, "media.m3u8")
	if err := os.WriteFile(playlistPath, []byte(rewritten), 0o644); err != nil {
		cleanup()
		return provider.Stream{}, fmt.Errorf("write local playlist: %w", err)
	}

	stream.URL = "file://" + playlistPath
	return stream, nil
}

type localResource struct {
	url  string
	path string
}

func (c *Client) localizeMediaPlaylist(body, sourceURL, tmpDir string) (string, []localResource, error) {
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(body))
	var resources []localResource
	segmentIndex := 0
	auxIndex := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out.WriteByte('\n')
		case strings.HasPrefix(trimmed, "#"):
			uri, start, end, ok := directiveURI(line)
			if !ok {
				out.WriteString(line)
				out.WriteByte('\n')
				continue
			}
			auxIndex++
			name := fmt.Sprintf("resource-%05d.bin", auxIndex)
			resources = append(resources, localResource{
				url:  c.resolveAgainst(uri, sourceURL),
				path: filepath.Join(tmpDir, name),
			})
			out.WriteString(line[:start])
			out.WriteString(name)
			out.WriteString(line[end:])
			out.WriteByte('\n')
		default:
			segmentIndex++
			name := fmt.Sprintf("segment-%05d.ts", segmentIndex)
			resources = append(resources, localResource{
				url:  c.resolveAgainst(trimmed, sourceURL),
				path: filepath.Join(tmpDir, name),
			})
			out.WriteString(name)
			out.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("parse media playlist: %w", err)
	}
	return out.String(), resources, nil
}

func directiveURI(line string) (uri string, start, end int, ok bool) {
	const marker = `URI="`
	markerStart := strings.Index(line, marker)
	if markerStart < 0 {
		return "", 0, 0, false
	}
	start = markerStart + len(marker)
	quote := strings.IndexByte(line[start:], '"')
	if quote < 0 {
		return "", 0, 0, false
	}
	end = start + quote
	return line[start:end], start, end, true
}

func (c *Client) downloadResources(ctx context.Context, resources []localResource, progress provider.SegmentProgressFunc) error {
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan localResource)
	var wg sync.WaitGroup
	var completed int
	var progressMu sync.Mutex
	var firstErr error
	var errOnce sync.Once

	workerCount := min(c.segmentWorkers, len(resources))
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for resource := range jobs {
				if err := c.downloadResource(workCtx, resource); err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("%s: %w", filepath.Base(resource.path), err)
						cancel()
					})
					continue
				}
				progressMu.Lock()
				completed++
				if progress != nil {
					progress(completed, len(resources))
				}
				progressMu.Unlock()
			}
		}()
	}

enqueue:
	for _, resource := range resources {
		select {
		case jobs <- resource:
		case <-workCtx.Done():
			break enqueue
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return fmt.Errorf("download stream resource: %w", firstErr)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (c *Client) downloadResource(ctx context.Context, resource localResource) error {
	var lastErr error
	for attempt := range 6 {
		if attempt > 0 {
			delay := 200 * time.Millisecond * time.Duration(1<<(attempt-1))
			if delay > 2*time.Second {
				delay = 2 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		retry, err := c.downloadResourceOnce(ctx, resource)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry {
			return err
		}
	}
	return lastErr
}

func (c *Client) downloadResourceOnce(ctx context.Context, resource localResource) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.proxiedURL(resource.url), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", "https://bettermelon.ru/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return retry, fmt.Errorf("status %d", resp.StatusCode)
	}

	file, err := os.Create(resource.path)
	if err != nil {
		return false, err
	}
	limited := &io.LimitedReader{R: resp.Body, N: maxSegmentSize + 1}
	written, copyErr := io.Copy(file, limited)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(resource.path)
		return true, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(resource.path)
		return true, closeErr
	}
	if written > maxSegmentSize {
		_ = os.Remove(resource.path)
		return false, fmt.Errorf("resource exceeds %d bytes", maxSegmentSize)
	}
	return false, nil
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
