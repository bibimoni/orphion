package jimaku

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/subtitle"
)

// Client scrapes jimaku.cc pages for anime subtitle entries.
// The home page listing is cached after the first fetch so that repeated
// searches within the same session don't hit the network again.
type Client struct {
	cfg    Config
	client *http.Client

	mu    sync.Mutex
	cache []entryLink // cached home page entries
}

// entryLink represents a parsed entry link from the home page.
type entryLink struct {
	ID    string // numeric ID (e.g. "1315")
	Title string // display name
	Href  string // raw href attribute
}

// entryRe extracts href and display text from entry links on the home page.
var entryRe = regexp.MustCompile(`<a\s+href="/entry/(\d+)"[^>]*>([^<]+)</a>`)

// fileRe extracts href and display text from subtitle file links on entry pages.
// Matches links like /entry/1315/download/filename.srt
var fileRe = regexp.MustCompile(`<a\s+href="(/entry/\d+/download/[^"]+)"[^>]*>([^<]+)</a>`)

// NewClient creates a jimaku.cc client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("jimaku: base URL is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = cfg.defaultHTTPClient()
	}
	return &Client{
		cfg:    cfg,
		client: httpClient,
	}, nil
}

// Search lists anime entries matching the query from the jimaku.cc home page.
// The home page is fetched once and cached; subsequent searches use the cache.
func (c *Client) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	entries, err := c.fetchEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("jimaku search: %w", err)
	}

	// Pre-compute query tokens for client-side filtering.
	queryTokens := tokenizeQuery(query)

	var results []subtitle.Result
	seen := make(map[string]bool)

	for _, e := range entries {
		// Deduplicate by ID.
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true

		// Cheap pre-filter: skip entries that share no tokens with the query.
		cleaned := cleanTitle(e.Title)
		if len(queryTokens) > 0 && !hasTokenOverlap(cleaned, queryTokens) {
			continue
		}

		results = append(results, subtitle.Result{
			ID:     "jimaku:" + e.ID,
			Title:  cleanTitle(e.Title),
			Type:   "tv",
			Slug:   e.ID,
			Source: "jimaku",
		})
	}

	return results, nil
}

// Page lists the subtitle files for a specific anime entry.
func (c *Client) Page(ctx context.Context, id, slug, seasonSlug string) (*subtitle.PageResult, error) {
	// Use slug if available, otherwise fall back to id.
	cleanID := slug
	if cleanID == "" {
		cleanID = id
	}
	// Strip "jimaku:" prefix if present.
	cleanID = strings.TrimPrefix(cleanID, "jimaku:")

	pageURL := c.cfg.BaseURL + "/entry/" + cleanID
	body, err := c.fetchPage(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("jimaku page %s: %w", pageURL, err)
	}

	// Parse subtitle file links.
	matches := fileRe.FindAllStringSubmatch(body, -1)
	var subs []subtitle.Subtitle
	for i, m := range matches {
		if len(m) < 3 {
			continue
		}
		href := m[1]
		name := strings.TrimSpace(m[2])
		if name == "" {
			continue
		}

		lang := detectLanguage(name)

		subs = append(subs, subtitle.Subtitle{
			ID:       i + 1,
			Language: lang,
			Quality:  detectQuality(name),
			Link:     c.cfg.BaseURL + href,
			Title:    name,
			Source:   "jimaku",
		})
	}

	return &subtitle.PageResult{
		Subtitles: subs,
	}, nil
}

// DownloadURL returns the direct URL for a subtitle file.
// For jimaku, the Link field already contains the full URL.
func (c *Client) DownloadURL(sub subtitle.Subtitle) string {
	return sub.Link
}

// fetchEntries fetches and caches the home page entry listing.
func (c *Client) fetchEntries(ctx context.Context) ([]entryLink, error) {
	c.mu.Lock()
	if c.cache != nil {
		cached := c.cache
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	body, err := c.fetchPage(ctx, c.cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	matches := entryRe.FindAllStringSubmatch(body, -1)
	entries := make([]entryLink, 0, len(matches))
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		entries = append(entries, entryLink{
			ID:    m[1],
			Title: strings.TrimSpace(m[2]),
			Href:  "/entry/" + m[1],
		})
	}

	c.mu.Lock()
	c.cache = entries
	c.mu.Unlock()

	return entries, nil
}

// fetchPage fetches a URL and returns the response body.
func (c *Client) fetchPage(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", url, err)
	}

	return string(body), nil
}

// cleanTitle removes surrounding quotes from a title.
// jimaku.cc often wraps titles in double quotes or curly quotes.
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	if len(title) >= 2 && title[0] == '"' && title[len(title)-1] == '"' {
		title = title[1 : len(title)-1]
	}
	// Check for Unicode curly quotes (U+201C and U+201D).
	runes := []rune(title)
	if len(runes) >= 2 && runes[0] == '\u201C' && runes[len(runes)-1] == '\u201D' {
		title = string(runes[1 : len(runes)-1])
	}
	return title
}

// detectLanguage attempts to detect the subtitle language from the filename.
func detectLanguage(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, ".ja-en.") || strings.Contains(lower, ".ja."):
		return "japanese"
	case strings.Contains(lower, ".en-") || strings.Contains(lower, ".en."):
		return "english"
	case strings.Contains(lower, "japanese") || strings.Contains(lower, "ja[cc]"):
		return "japanese"
	case strings.Contains(lower, "english"):
		return "english"
	default:
		for _, r := range name {
			if (r >= 0x3040 && r <= 0x30FF) || (r >= 0x4E00 && r <= 0x9FFF) {
				return "japanese"
			}
		}
	}
	return "english"
}

// detectQuality attempts to detect the subtitle quality from the filename.
func detectQuality(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "bd") || strings.Contains(lower, "bluray"):
		return "bluray"
	case strings.Contains(lower, "webrip") || strings.Contains(lower, "web-dl") || strings.Contains(lower, "netflix"):
		return "webrip"
	default:
		return "other"
	}
}

// tokenizeQuery splits a query into lowercase tokens for matching.
func tokenizeQuery(query string) map[string]bool {
	normalized := strings.ToLower(query)
	tokens := strings.Fields(normalized)
	m := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		if len(t) >= 2 {
			m[t] = true
		}
	}
	return m
}

// hasTokenOverlap checks whether a title shares at least one token
// with the query tokens.
func hasTokenOverlap(title string, queryTokens map[string]bool) bool {
	titleTokens := tokenizeQuery(title)
	for t := range titleTokens {
		if queryTokens[t] {
			return true
		}
	}
	// Also check prefix matching for partial matches.
	for qt := range queryTokens {
		for tt := range titleTokens {
			if strings.HasPrefix(tt, qt) || strings.HasPrefix(qt, tt) {
				return true
			}
		}
	}
	return false
}
