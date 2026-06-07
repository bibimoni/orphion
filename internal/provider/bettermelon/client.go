package bettermelon

import (
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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/provider"
)

// availableProviders is the list of upstream Bettermelon providers in fallback order.
var availableProviders = []string{"hianime", "animekai", "kickassanime", "anikoto"}

// uriPattern matches URI="..." inside HLS tags like #EXT-X-I-FRAME-STREAM-INF.
var uriPattern = regexp.MustCompile(`URI="[^"]*"`)

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
	httpClient *http.Client
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
func (c *Client) fetchJSON(ctx context.Context, path string, out any) error {
	endpoint := c.apiURL.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", redactedRequestError{err: err})
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

// buildStreams converts a CDN m3u8 URL into a downloadable stream.
// It downloads the HLS manifest, rewrites obfuscated segment URLs
// with a #/seg.ts hint for ffmpeg, and writes a local temp file.
func (c *Client) buildStreams(ctx context.Context, fileURL string) []provider.Stream {
	if fileURL == "" {
		return nil
	}
	localM3U8, err := c.rewriteManifest(ctx, fileURL)
	if err != nil {
		// Fallback: return the raw proxy URL (may fail with obfuscated extensions).
		proxied := c.proxiedURL(fileURL)
		headers := make(http.Header)
		headers.Set("Referer", "https://bettermelon.ru/")
		headers.Set("User-Agent", c.userAgent)
		return []provider.Stream{{URL: proxied, Quality: "", Headers: headers}}
	}
	headers := make(http.Header)
	headers.Set("Referer", "https://bettermelon.ru/")
	headers.Set("User-Agent", c.userAgent)
	return []provider.Stream{{URL: localM3U8, Quality: "", Headers: headers}}
}

// proxiedURL rewrites a CDN URL through the Bettermelon proxy.
func (c *Client) proxiedURL(rawURL string) string {
	if c.proxyURL == nil {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	proxy := *c.proxyURL
	q := proxy.Query()
	q.Set("url", parsed.String())
	proxy.RawQuery = q.Encode()
	proxy.Path = "/proxy"
	return proxy.String()
}

// rewriteManifest downloads the HLS manifest chain through the proxy,
// appends #/seg.ts to every segment URL so ffmpeg uses the MPEG-TS
// demuxer, and writes the result to a temporary file.
func (c *Client) rewriteManifest(ctx context.Context, masterURL string) (string, error) {
	proxiedMaster := c.proxiedURL(masterURL)
	masterContent, err := c.fetchRaw(ctx, proxiedMaster)
	if err != nil {
		return "", fmt.Errorf("fetch master m3u8: %w", err)
	}

	// Rewrite the master manifest: make sub-manifest URLs absolute.
	rewritten := c.rewriteManifestContent(string(masterContent), proxiedMaster)

	// If the master has no segment lines (only sub-manifest references),
	// also fetch and inline the sub-manifests.
	if !strings.Contains(rewritten, "#EXTINF:") {
		rewritten, err = c.inlineSubManifests(ctx, rewritten, proxiedMaster)
		if err != nil {
			return "", fmt.Errorf("inline sub-manifests: %w", err)
		}
	}

	// Add #/seg.ts fragment to segment URLs so ffmpeg detects MPEG-TS.
	rewritten = c.addSegmentFragment(rewritten)

	// Write to temp file.
	f, err := os.CreateTemp("", "bettermelon-*.m3u8")
	if err != nil {
		return "", fmt.Errorf("create temp m3u8: %w", err)
	}
	if _, err := f.WriteString(rewritten); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("write temp m3u8: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("close temp m3u8: %w", err)
	}
	return "file://" + f.Name(), nil
}

// rewriteManifestContent makes relative URLs absolute in an m3u8 manifest.
func (c *Client) rewriteManifestContent(content, baseURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return content
	}
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Rewrite lines that contain URLs (not tags, not empty).
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if resolved := resolveURL(base, trimmed); resolved != "" {
				lines = append(lines, resolved)
				continue
			}
		}
		// Rewrite URI= inside #EXT-X-I-FRAME-STREAM-INF or #EXT-X-MEDIA.
		if strings.Contains(trimmed, `URI="`) {
			lines = append(lines, c.rewriteURIAttribute(trimmed, base))
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// rewriteURIAttribute rewrites URI="..." inside a tag line.
func (c *Client) rewriteURIAttribute(line string, base *url.URL) string {
	return uriPattern.ReplaceAllStringFunc(line, func(match string) string {
		uriStart := strings.Index(match, `"`) + 1
		uriEnd := strings.LastIndex(match, `"`)
		if uriStart >= uriEnd {
			return match
		}
		uri := match[uriStart:uriEnd]
		if resolved := resolveURL(base, uri); resolved != "" {
			return match[:uriStart] + resolved + match[uriEnd:]
		}
		return match
	})
}

// addSegmentFragment appends #/seg.ts to segment URLs in the m3u8.
func (c *Client) addSegmentFragment(content string) string {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "/proxy?url=") {
			line = trimmed + "#/seg.ts"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// inlineSubManifests fetches sub-manifests referenced in the master playlist,
// rewrites them, and returns a merged playlist with all segments inlined.
func (c *Client) inlineSubManifests(ctx context.Context, masterContent, _ string) (string, error) {
	var result []string
	for _, line := range strings.Split(masterContent, "\n") {
		trimmed := strings.TrimSpace(line)
		// If this line is a URL (not a tag), fetch the sub-manifest.
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, ".m3u8") {
			subContent, err := c.fetchRaw(ctx, trimmed)
			if err != nil {
				return "", fmt.Errorf("fetch sub-manifest %s: %w", trimmed, err)
			}
			// Rewrite sub-manifest URLs to be absolute.
			rewrittenSub := c.rewriteManifestContent(string(subContent), trimmed)
			result = append(result, rewrittenSub)
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n"), nil
}

// fetchRaw performs a GET request and returns the response body.
func (c *Client) fetchRaw(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
}

// resolveURL resolves a possibly-relative URL against a base URL.
func resolveURL(base *url.URL, ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if u.Scheme == "" {
		return base.ResolveReference(u).String()
	}
	return ref
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
