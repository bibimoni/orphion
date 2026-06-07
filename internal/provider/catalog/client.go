package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/distiled/orphion/internal/provider"
)

// Client fetches content from the catalog API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// Config holds catalog configuration.
type Config struct {
	BaseURL string
}

// DefaultBaseURL is the upstream catalog endpoint.
const DefaultBaseURL = "https://allanime.delivery"

// NewClient creates a Catalog API client.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    cfg.BaseURL,
	}
}

// Search queries the catalog for anime matching a query and kind.
func (c *Client) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/search", nil)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	req.URL.RawQuery = url.Values{
		"q":    {query},
		"kind": {kind},
	}.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search %q: %w", query, err)
	}
	defer resp.Body.Close()

	var results []searchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	var out []provider.Anime
	for _, r := range results {
		out = append(out, provider.Anime{
			ID:    r.ID,
			Title: r.Title,
		})
	}
	return out, nil
}

type searchResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Episodes returns the episode list for an anime title.
func (c *Client) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/episodes", nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = url.Values{"id": {animeID}}.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get episodes: %w", err)
	}
	defer resp.Body.Close()

	var eps []struct {
		ID     string  `json:"id"`
		Number float64 `json:"number"`
		Label  string  `json:"label"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eps); err != nil {
		return nil, fmt.Errorf("decode episodes: %w", err)
	}

	var out []provider.Episode
	for _, e := range eps {
		out = append(out, provider.Episode{
			ID:      e.ID,
			Number:  e.Label,
			SortKey: e.Number,
		})
	}
	return out, nil
}

// Streams returns available quality streams for an episode.
func (c *Client) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/streams", nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = url.Values{"id": {episodeID}}.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get streams: %w", err)
	}
	defer resp.Body.Close()

	var raw []struct {
		URL     string `json:"url"`
		Quality string `json:"quality"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode streams: %w", err)
	}

	var out []provider.Stream
	for _, s := range raw {
		out = append(out, provider.Stream{
			URL:     safeURL(s.URL),
			Quality: s.Quality,
			Headers: c.defaultHeaders(),
		})
	}
	return out, nil
}

func (c *Client) defaultHeaders() http.Header {
	h := http.Header{}
	h.Set("User-Agent", "orphion/1.0")
	return h
}

func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.String()
}

// IsSan logs a sanitized version of the upstream host.
func IsSan(host string) string {
	return strings.Replace(host, ".", "-", -1)
}