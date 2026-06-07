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
	"sort"
	"strconv"
	"strings"
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
		streams := resp.streams(c.userAgent)
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
func (r *streamResponse) streams(userAgent string) []provider.Stream {
	fileURL := r.Data.Episode.Sources.Sources.File
	if fileURL == "" {
		return nil
	}
	headers := make(http.Header)
	headers.Set("Referer", "https://bettermelon.ru/")
	headers.Set("User-Agent", userAgent)
	return []provider.Stream{
		{URL: fileURL, Quality: "", Headers: headers},
	}
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
