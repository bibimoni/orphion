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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/distiled/orphion/internal/provider"
)

// episodeRangeRe extracts episode ranges from torrent titles like "(01-11)".
var episodeRangeRe = regexp.MustCompile(`\((\d+)-(\d+)\)`)

// singleEpisodeRe extracts a single episode number like "E01", "EP01", "Ep01", "ep01".
var singleEpisodeRe = regexp.MustCompile(`(?i)(?:E|EP|ep)\s*(\d+)`)

// japaneseEpRe extracts episode numbers from Japanese patterns like "第一話", "第2話", "最終話".
var japaneseEpRe = regexp.MustCompile(`第(\d+)話`)

// bracketEpsRe extracts episode numbers from brackets like "[01]", "[02]", "[11]".
var bracketEpsRe = regexp.MustCompile(`\[(\d{1,3})\]`)

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

// torrentEntry is a parsed Nyaa torrent with normalized metadata.
type torrentEntry struct {
	infoHash string
	title    string
	seeders  int
	size     string
}

// showCache stores grouped search results keyed by the virtual show ID.
type showCache struct {
	mu      sync.RWMutex
	shows   map[string]*showGroup // showID → group
	entries map[string]string     // episodeID → showID (reverse lookup)
}

// showGroup represents a virtual "show" — a group of related torrents.
type showGroup struct {
	title    string
	torrents []torrentEntry
}

func newShowCache() *showCache {
	return &showCache{
		shows:   make(map[string]*showGroup),
		entries: make(map[string]string),
	}
}

func (c *showCache) Put(groups map[string]*showGroup) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shows = groups
	c.entries = make(map[string]string)
	for showID, group := range groups {
		for _, t := range group.torrents {
			c.entries[t.infoHash] = showID
		}
	}
}

func (c *showCache) GetShow(showID string) (*showGroup, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	g, ok := c.shows[showID]
	return g, ok
}

func (c *showCache) ShowIDForEpisode(episodeID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Strip episode suffix if present
	infoHash := episodeID
	if idx := strings.LastIndex(episodeID, ":"); idx != -1 {
		infoHash = episodeID[:idx]
	}
	id, ok := c.entries[infoHash]
	return id, ok
}

// Client fetches and normalizes Nyaa.si data.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	category   string
	userAgent  string
	trackers   []string
	cache      *showCache
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
		cache:      newShowCache(),
	}, nil
}

// Search queries Nyaa.si for matching torrents via RSS and groups them
// into virtual "shows" by normalized title. Each group becomes one
// provider.Anime result, so ResolveID can find a unique match.
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

	// Parse RSS items into torrent entries.
	seen := make(map[string]bool)
	var entries []torrentEntry
	for _, item := range feed.Channel.Items {
		infoHash := item.InfoHash
		if infoHash == "" {
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

		seeders, _ := strconv.Atoi(item.Seeders)
		entries = append(entries, torrentEntry{
			infoHash: infoHash,
			title:    title,
			seeders:  seeders,
			size:     item.Size,
		})
	}

	// Group by normalized show title.
	groups := groupByShow(entries)

	// Store in cache for later Episodes/Streams lookups.
	c.cache.Put(groups)

	// Convert groups to provider.Anime results, sorted by torrent count desc.
	var results []provider.Anime
	for showID, group := range groups {
		results = append(results, provider.Anime{
			ID:    showID,
			Title: group.title,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		gi, _ := c.cache.GetShow(results[i].ID)
		gj, _ := c.cache.GetShow(results[j].ID)
		return len(gi.torrents) > len(gj.torrents)
	})

	return results, nil
}

// Episodes returns episodes for a given show (grouped from Nyaa search results).
// Each torrent becomes one or more episodes depending on the title parsing.
func (c *Client) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	if animeID == "" {
		return nil, fmt.Errorf("nyaa episodes: empty anime ID")
	}

	group, ok := c.cache.GetShow(animeID)
	if !ok {
		return nil, fmt.Errorf("nyaa episodes: unknown show %q (run search first)", animeID)
	}

	var episodes []provider.Episode
	epNum := 1
	for _, t := range group.torrents {
		parsed := EpisodesFromTitle(t.infoHash, t.title)
		if len(parsed) == 1 && parsed[0].Number == "1" {
			// Couldn't extract episode number — assign sequential.
			parsed[0].Number = strconv.Itoa(epNum)
			parsed[0].SortKey = float64(epNum)
			epNum++
		}
		episodes = append(episodes, parsed...)
	}

	// If no episodes extracted, return a single batch episode.
	if len(episodes) == 0 {
		episodes = []provider.Episode{
			{ID: animeID, Number: "1", SortKey: 1.0},
		}
	}

	return episodes, nil
}

// Streams returns magnet URIs for a given episode.
// The episodeID is either the infoHash directly or infoHash:episodeNumber.
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

// groupByShow clusters torrent entries by normalized title into virtual shows.
// It uses CJK prefix extraction for Japanese/Chinese titles and falls back
// to cleaned title comparison for English titles.
func groupByShow(entries []torrentEntry) map[string]*showGroup {
	groups := make(map[string]*showGroup) // normalKey → group

	for _, entry := range entries {
		key := showGroupKey(entry.title)

		if g, ok := groups[key]; ok {
			g.torrents = append(g.torrents, entry)
		} else {
			showTitle := extractShowTitle(entry.title)
			groups[key] = &showGroup{
				title:    showTitle,
				torrents: []torrentEntry{entry},
			}
		}
	}

	// Re-key groups by first torrent's infoHash (stable ID for provider.Anime).
	result := make(map[string]*showGroup, len(groups))
	for _, g := range groups {
		if len(g.torrents) > 0 {
			result[g.torrents[0].infoHash] = g
		}
	}
	return result
}

// showGroupKey produces a canonical grouping key for a torrent title.
// For CJK titles, it extracts the leading CJK character run.
// For English titles, it strips tags/ep numbers and lowercases.
func showGroupKey(title string) string {
	// Strip [] tags (sub group, quality) but keep 【】 content (often the title).
	cleaned := bracketTagRe.ReplaceAllString(title, "")

	// Extract content from 【】 brackets (Japanese title brackets).
	// If present, prefer the 【】 content as the title.
	if matches := cjkBracketRe.FindStringSubmatch(cleaned); len(matches) >= 2 {
		cjkContent := matches[1]
		if prefix := extractCJKPrefix(cjkContent); len(prefix) >= 2 {
			return prefix
		}
	}

	// Try CJK prefix from the cleaned title.
	if prefix := extractCJKPrefix(cleaned); len(prefix) >= 2 {
		return prefix
	}

	// Fallback: normalize for English titles.
	return normalizeEnglishTitle(cleaned)
}

// extractCJKPrefix returns the leading run of CJK + kana characters,
// stopping at the first non-CJK/non-kana character or delimiter.
func extractCJKPrefix(title string) string {
	var buf []rune
	for _, r := range title {
		if isCJK(r) {
			buf = append(buf, r)
		} else if len(buf) > 0 {
			break
		}
	}
	return string(buf)
}

// isCJK reports whether a rune is a CJK ideograph or Japanese kana.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		r == 'ー' // prolonged sound mark
}

// bracketTagRe matches [tag] patterns like [SubGroup], [720p], [Batch].
// Note: 【】 (CJK brackets) often contain the show title itself, so we only
// strip [] (square) brackets which typically hold group/quality tags.
var bracketTagRe = regexp.MustCompile(`\[.*?\]`)

// cjkBracketRe matches 【...】 patterns. Used separately because these
// often contain the actual show title rather than tags.
var cjkBracketRe = regexp.MustCompile(`【(.*?)】`)

// normalizeEnglishTitle strips episode numbers, resolution tags, etc.
func normalizeEnglishTitle(title string) string {
	s := title
	s = episodeRangeRe.ReplaceAllString(s, "")
	s = singleEpisodeRe.ReplaceAllString(s, "")
	s = regexp.MustCompile(`\.(mp4|mkv|avi|ts)$`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\b(480|720|1080|2160)[ipx]\b`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\b(HDTV|HDTVrip|WEB|WEBrip|BluRay|BDRip|BDR|RAW|Subbed|Hardsub)\b`).ReplaceAllString(s, "")
	s = strings.TrimRight(s, " -_/")
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// extractShowTitle returns a human-readable show title from a torrent title.
func extractShowTitle(title string) string {
	// Strip [] tags first.
	cleaned := bracketTagRe.ReplaceAllString(title, "")

	// If 【】 brackets are present, try their content.
	if matches := cjkBracketRe.FindStringSubmatch(cleaned); len(matches) >= 2 {
		cjkContent := matches[1]
		if prefix := extractCJKPrefix(cjkContent); len(prefix) >= 2 {
			return prefix
		}
	}

	// Try CJK prefix from the cleaned title.
	if prefix := extractCJKPrefix(cleaned); len(prefix) >= 2 {
		return prefix
	}
	// Try to extract before "/" separator (common: Japanese / Romaji).
	parts := strings.SplitN(cleaned, "/", 2)
	if len(parts) == 2 {
		candidate := strings.TrimSpace(parts[0])
		if candidate != "" {
			return candidate
		}
	}
	// Fallback: clean up full title.
	s := bracketTagRe.ReplaceAllString(title, "")
	s = regexp.MustCompile(`\s*[-/]\s*(E|EP|ep|第).*$`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s*[(\(].*$`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
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

	// Try single episode like "E01", "Ep01", "EP01"
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

	// Try Japanese episode marker like "第一話", "第2話"
	if matches := japaneseEpRe.FindStringSubmatch(title); len(matches) >= 2 {
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

	// Try bracketed episode number like "[01]"
	if matches := bracketEpsRe.FindAllStringSubmatch(title, -1); len(matches) > 0 {
		// Use the last bracketed number (usually the episode).
		last := matches[len(matches)-1]
		if num, err := strconv.Atoi(last[1]); err == nil && num > 0 && num < 1000 {
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
