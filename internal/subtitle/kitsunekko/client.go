package kitsunekko

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/bibimoni/orphion/internal/common"

	"github.com/bibimoni/orphion/internal/subtitle"
)

// Client scrapes the kitsunekko.net Apache directory listings.
// Directory listings are cached after the first fetch so that repeated
// searches within the same session don't hit the network again.
type Client struct {
	cfg    Config
	client *http.Client

	mu    sync.Mutex
	cache map[string][]dirEntry // URL → parsed entries
}

// NewClient creates a kitsunekko client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("kitsunekko: base URL is required")
	}
	return &Client{
		cfg:    cfg,
		client: cfg.HTTPClient(),
		cache:  make(map[string][]dirEntry),
	}, nil
}

// dirEntry represents a parsed directory or file link from an Apache listing.
type dirEntry struct {
	Name string // display name (unescaped)
	Href string // raw href attribute (URL-encoded)
}

// linkRe extracts href and display text from <a href="...">...</a> tags.
var linkRe = regexp.MustCompile(`<a\s+href="([^"]+)"[^>]*>([^<]+)</a>`)

// langFetch is the result of fetching one language directory.
type langFetch struct {
	lang    string
	entries []dirEntry
}

// Search lists anime directories matching the query. Language directories
// are fetched in parallel. Results are pre-filtered by token overlap
// with the query so that irrelevant directories (thousands) are discarded
// cheaply before the caller's expensive ranking step.
func (c *Client) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	// Fetch all language directories in parallel.
	var wg sync.WaitGroup
	ch := make(chan langFetch, len(c.cfg.Languages))

	for _, lang := range c.cfg.Languages {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()
			url := c.langURL(lang)
			entries, err := c.fetchDirEntries(ctx, url)
			if err != nil {
				return // skip failing language dirs
			}
			ch <- langFetch{lang: lang, entries: entries}
		}(lang)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// Pre-compute query tokens for cheap client-side filtering.
	queryTokens := dirNameTokens(query)

	var results []subtitle.Result
	seen := make(map[string]bool) // deduplicate by normalized directory name

	for fetch := range ch {
		for _, e := range fetch.entries {
			// Only directories (end with /).
			if !strings.HasSuffix(e.Href, "/") {
				continue
			}
			// Skip parent directory link.
			name := strings.TrimSuffix(e.Name, "/")
			if name == "Parent Directory" || name == "" {
				continue
			}
			normalKey := normalizeDirName(name)
			if seen[normalKey] {
				continue
			}
			// Cheap pre-filter: skip directories that share no tokens
			// with the query. This eliminates thousands of irrelevant
			// entries before the expensive ranking step.
			if len(queryTokens) > 0 && !hasTokenOverlap(name, queryTokens) {
				continue
			}
			seen[normalKey] = true

			// Build a slug from the raw href (keeps URL-encoding intact
			// so that Page can reconstruct the correct URL later).
			slug := strings.TrimSuffix(e.Href, "/")
			// Strip ./ prefix that some Apache listings include.
			slug = strings.TrimPrefix(slug, "./")

			results = append(results, subtitle.Result{
				ID:     "kitsunekko:" + langPrefix(fetch.lang) + slug,
				Title:  name,
				Type:   "tv",
				Slug:   langPrefix(fetch.lang) + slug,
				Source: "kitsunekko",
			})
		}
	}

	return results, nil
}

// Page lists the subtitle files in a specific anime directory.
func (c *Client) Page(ctx context.Context, id, slug, seasonSlug string) (*subtitle.PageResult, error) {
	// Determine which language path to use based on the slug.
	// The slug may be prefixed with the language (e.g., "ja:Steins_Gashi").
	lang, cleanSlug := parseSlugLang(slug)

	url := c.animeURL(lang, cleanSlug)
	entries, err := c.fetchDirEntries(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("kitsunekko: fetch page %s: %w", url, err)
	}

	var subs []subtitle.Subtitle
	for i, e := range entries {
		// Only archive files.
		href := strings.TrimPrefix(e.Href, "./")
		if !strings.HasSuffix(href, ".zip") && !strings.HasSuffix(href, ".rar") && !strings.HasSuffix(href, ".7z") {
			continue
		}
		// Skip parent directory.
		name := e.Name
		if name == "Parent Directory" || name == "" {
			continue
		}

		fileURL := url + href
		subs = append(subs, subtitle.Subtitle{
			ID:       i + 1,
			Language: langLabel(lang),
			Quality:  "webrip",
			Link:     fileURL,
			Title:    name,
			Source:   "kitsunekko",
		})
	}

	return &subtitle.PageResult{
		Subtitles: subs,
	}, nil
}

// DownloadURL returns the direct URL for a subtitle file.
// For kitsunekko, the Link field already contains the full URL.
func (c *Client) DownloadURL(sub subtitle.Subtitle) string {
	return sub.Link
}

// fetchDirEntries fetches and parses an Apache directory listing page.
// Results are cached by URL so that repeated searches within the same
// session don't hit the network again.
func (c *Client) fetchDirEntries(ctx context.Context, url string) ([]dirEntry, error) {
	c.mu.Lock()
	if entries, ok := c.cache[url]; ok {
		c.mu.Unlock()
		return entries, nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	entries := parseDirListing(string(body))

	c.mu.Lock()
	c.cache[url] = entries
	c.mu.Unlock()

	return entries, nil
}

// parseDirListing extracts directory entries from an Apache index page.
func parseDirListing(html string) []dirEntry {
	matches := linkRe.FindAllStringSubmatch(html, -1)
	var entries []dirEntry
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		href := m[1]
		name := strings.TrimSpace(m[2])
		if name == "" || name == "." || name == ".." {
			continue
		}
		entries = append(entries, dirEntry{Name: name, Href: href})
	}
	return entries
}

// langURL builds the URL for a language's root directory listing.
// Empty string means the default (English) path.
// Short codes (ja, en) are expanded to full directory names (japanese, english).
func (c *Client) langURL(lang string) string {
	dir := langDirName(lang)
	if dir == "" {
		return c.cfg.BaseURL + "/subtitles/"
	}
	return c.cfg.BaseURL + "/subtitles/" + dir + "/"
}

// langDirName maps a language code to the kitsunekko directory name.
func langDirName(lang string) string {
	switch lang {
	case "ja", "japanese":
		return "japanese"
	case "en", "english", "":
		return ""
	case "ko", "korean":
		return "korean"
	case "zh", "chinese":
		return "chinese"
	default:
		return lang
	}
}

// animeURL builds the URL for a specific anime directory.
func (c *Client) animeURL(lang, slug string) string {
	base := c.langURL(lang)
	// Ensure slug ends with /.
	if !strings.HasSuffix(slug, "/") {
		slug += "/"
	}
	return base + slug
}

// langPrefix returns the language prefix for slug construction.
// e.g. "japanese" → "ja:", "" → ""
func langPrefix(lang string) string {
	switch lang {
	case "japanese":
		return "ja:"
	case "english", "":
		return ""
	case "korean":
		return "ko:"
	case "chinese":
		return "zh:"
	default:
		return lang + ":"
	}
}

// langLabel returns a human-readable label for a language path segment.
func langLabel(lang string) string {
	switch lang {
	case "japanese", "ja":
		return "japanese"
	case "", "english", "en":
		return "english"
	default:
		return lang
	}
}

// parseSlugLang extracts the language prefix from a slug like "ja:Steins_Gate".
// Returns (lang, cleanSlug). If no prefix, returns ("", slug).
// Supports short codes (ja, en) and full names (japanese, english, korean).
func parseSlugLang(slug string) (string, string) {
	parts := strings.SplitN(slug, ":", 2)
	if len(parts) == 2 && len(parts[0]) >= 2 && len(parts[0]) <= 10 {
		// Valid language prefix: "ja", "en", "japanese", "english", "korean", etc.
		// Reject single-char prefixes (likely a Windows drive letter like "C:").
		return parts[0], parts[1]
	}
	return "", slug
}

// normalizeDirName produces a canonical key for deduplicating directory
// names that differ only in underscores vs spaces or URL encoding.
// e.g. "Dagashi_Kashi" and "Dagashi Kashi" both become "dagashi kashi".
// Special characters (semicolons, colons, etc.) are treated as separators
// so that "Steins;Gate" and "Stein;Gate" produce overlapping tokens.
func normalizeDirName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "_", " ")
	// Replace common punctuation with spaces so they act as separators.
	// This ensures "Steins;Gate" becomes "steins gate" (two tokens)
	// rather than "steins;gate" (one token), which matches the
	// normalizeTitle logic used by the ranking algorithm.
	for _, ch := range []string{";", ":", "-", ".", ",", "!", "?", "'", "\"", "(", ")"} {
		s = strings.ReplaceAll(s, ch, " ")
	}
	// Collapse multiple spaces.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// dirNameTokens splits a directory name into lowercase tokens for cheap
// matching. Underscores and special characters are treated as separators.
func dirNameTokens(name string) map[string]bool {
	normalized := normalizeDirName(name)
	tokens := strings.Fields(normalized)
	m := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		if len(t) >= 2 { // skip single-char tokens
			m[t] = true
		}
	}
	return m
}

// hasTokenOverlap checks whether a directory name shares at least one
// token with the query tokens. This is a cheap pre-filter that eliminates
// thousands of clearly-unrelated directories before expensive ranking.
func hasTokenOverlap(name string, queryTokens map[string]bool) bool {
	dirTokens := dirNameTokens(name)
	for t := range dirTokens {
		if queryTokens[t] {
			return true
		}
	}
	// Also check if any query token is a prefix of a directory token
	// (e.g. query "dagash" should match "dagashi").
	for qt := range queryTokens {
		for dt := range dirTokens {
			if strings.HasPrefix(dt, qt) || strings.HasPrefix(qt, dt) {
				return true
			}
		}
	}
	return false
}
