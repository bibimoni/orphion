// Package nyaa implements the built-in Nyaa.si torrent provider.
package nyaa

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/distiled/orphion/internal/provider"
)

// episodeRangeRe extracts episode ranges from torrent titles like "(01-11)".
var episodeRangeRe = regexp.MustCompile(`\((\d+)-(\d+)\)`)

// singleEpisodeRe extracts a single episode number like "E01" or "EP01" or " - 01".
var singleEpisodeRe = regexp.MustCompile(`(?:E|EP|ep)\s*(\d+)`)

// rssFeed represents the top-level RSS document from Nyaa.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

// rssChannel represents the RSS channel element.
type rssChannel struct {
	Items []rssItem `xml:"item"`
}

// rssItem represents a single Nyaa torrent entry in RSS format.
type rssItem struct {
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	GUID     string `xml:"guid"`
	InfoHash string `xml:"https://nyaa.si/xmlns/nyaa infoHash"`
	Seeders  string `xml:"https://nyaa.si/xmlns/nyaa seeders"`
	Size     string `xml:"https://nyaa.si/xmlns/nyaa size"`
}

// Client fetches and normalizes Nyaa.si data.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	category   string
	userAgent  string
	trackers   []string
}

// NewClient validates configuration and creates a Nyaa client.
func NewClient(cfg Config) (*Client, error) {
	baseURL, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	if cfg.Category == "" {
		cfg.Category = CategoryLiveActionSubbed
	}
	trackers := defaultTrackers
	return &Client{
		httpClient: cfg.HTTPClient,
		baseURL:    baseURL,
		category:   cfg.Category,
		userAgent:  cfg.UserAgent,
		trackers:   trackers,
	}, nil
}

// Search queries Nyaa.si for matching torrents via RSS.
func (c *Client) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	cat := c.category
	switch kind {
	case "anime":
		cat = CategoryAnimeSubbed
	case "drama":
		cat = c.category // default is already live action subbed
	}

	searchURL := c.baseURL.String()
	params := url.Values{}
	params.Set("page", "rss")
	params.Set("c", cat)
	params.Set("q", query)
	searchURL += "?" + params.Encode()

	data, err := c.fetch(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("nyaa search: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("nyaa search: parse RSS: %w", err)
	}

	seen := make(map[string]bool)
	var results []provider.Anime

	for _, item := range feed.Channel.Items {
		infoHash := item.InfoHash
		if infoHash == "" {
			// Try extracting from the guid URL (e.g. https://nyaa.si/view/12345)
			infoHash = extractHashFromGUID(item.GUID)
		}
		if infoHash == "" {
			continue
		}

		title := strings.TrimSpace(item.Title)
		if title == "" || seen[infoHash] {
			continue
		}
		seen[infoHash] = true

		results = append(results, provider.Anime{
			ID:    infoHash,
			Title: cleanTitle(title),
		})
	}

	return results, nil
}

// Episodes returns episodes for a given Nyaa torrent.
// Nyaa torrents are typically batch files (e.g. episodes 01-11).
// If the title contains an episode range, individual episodes are generated.
// Otherwise, a single episode representing the batch is returned.
func (c *Client) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	if animeID == "" {
		return nil, fmt.Errorf("nyaa episodes: empty anime ID")
	}

	// animeID is the infoHash. For Nyaa, we store the torrent title
	// alongside the hash in our Anime struct. Since we only have the hash
	// here, we return a single batch episode. The caller can use the
	// title from the search result to determine if this is a batch.
	episodes := []provider.Episode{
		{
			ID:      animeID,
			Number:  "1",
			SortKey: 1.0,
		},
	}

	return episodes, nil
}

// EpisodesFromTitle creates episode entries by parsing the torrent title
// for episode ranges (e.g. "(01-11)" or "E01").
func EpisodesFromTitle(infoHash, title string) []provider.Episode {
	if infoHash == "" {
		return nil
	}

	// Try episode range like "(01-11)"
	if matches := episodeRangeRe.FindStringSubmatch(title); len(matches) >= 3 {
		start, err1 := strconv.Atoi(matches[1])
		end, err2 := strconv.Atoi(matches[2])
		if err1 == nil && err2 == nil && end >= start && end-start < 1000 {
			var episodes []provider.Episode
			for i := start; i <= end; i++ {
				episodes = append(episodes, provider.Episode{
					ID:      fmt.Sprintf("%s:%d", infoHash, i),
					Number:  strconv.Itoa(i),
					SortKey: float64(i),
				})
			}
			return episodes
		}
	}

	// Try single episode like "E01"
	if matches := singleEpisodeRe.FindStringSubmatch(title); len(matches) >= 2 {
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return []provider.Episode{
				{
					ID:      fmt.Sprintf("%s:%d", infoHash, num),
					Number:  strconv.Itoa(num),
					SortKey: float64(num),
				},
			}
		}
	}

	// Fallback: single batch episode
	return []provider.Episode{
		{ID: infoHash, Number: "1", SortKey: 1.0},
	}
}

// Streams returns magnet URIs for a given episode.
// For Nyaa, the episodeID is either the infoHash directly or
// infoHash:episodeNumber (from EpisodesFromTitle).
func (c *Client) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	if episodeID == "" {
		return nil, fmt.Errorf("nyaa streams: empty episode ID")
	}

	// Strip episode number suffix if present (e.g. "HASH:3" -> "HASH")
	infoHash := episodeID
	if idx := strings.LastIndex(episodeID, ":"); idx != -1 {
		infoHash = episodeID[:idx]
	}

	if len(infoHash) < 32 {
		return nil, fmt.Errorf("nyaa streams: invalid info hash %q", infoHash)
	}

	magnet := buildMagnetURI(infoHash, c.trackers)

	return []provider.Stream{
		{
			URL:     magnet,
			Quality: "torrent",
			Headers: http.Header{},
		},
	}, nil
}

// fetch retrieves raw bytes from a URL.
func (c *Client) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/rss+xml,application/xml,text/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return data, nil
}

// buildMagnetURI constructs a magnet URI from an info hash and tracker list.
func buildMagnetURI(infoHash string, trackers []string) string {
	var sb strings.Builder
	sb.WriteString("magnet:?xt=urn:btih:")
	sb.WriteString(infoHash)
	for _, tr := range trackers {
		sb.WriteString("&tr=")
		sb.WriteString(url.QueryEscape(tr))
	}
	return sb.String()
}

// extractHashFromGUID tries to extract a meaningful ID from a GUID URL.
func extractHashFromGUID(guid string) string {
	if guid == "" {
		return ""
	}
	u, err := url.Parse(guid)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(u.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && parts[i] != "view" {
			return parts[i]
		}
	}
	return ""
}

// cleanTitle removes extra whitespace from a title.
func cleanTitle(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
