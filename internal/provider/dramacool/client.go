// Package dramacool implements the built-in DramaCool provider.
package dramacool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/distiled/orphion/internal/provider"
)

// episodeNumberRe extracts the episode number from a DramaCool slug.
var episodeNumberRe = regexp.MustCompile(`episode-(\d+(?:\.\d+)?)`)

// m3u8Re extracts m3u8 stream URLs from a response body.
var m3u8Re = regexp.MustCompile(`https?://[^\s"'<>\\]+\.m3u8[^\s"'<>\\]*`)

// Client fetches and normalizes DramaCool data.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	apiURL     string // Xyra Stream API base URL (optional)
	apiKey     string // Xyra Stream API key (optional)
	userAgent  string
}

// NewClient validates configuration and creates a DramaCool client.
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
	return &Client{
		httpClient: cfg.HTTPClient,
		baseURL:    baseURL,
		apiURL:     cfg.APIURL,
		apiKey:     cfg.APIKey,
		userAgent:  cfg.UserAgent,
	}, nil
}

// Search queries DramaCool for matching dramas.
func (c *Client) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	// DramaCool uses WordPress, so ?s=query works for search
	searchURL := c.baseURL.String()
	params := url.Values{}
	params.Set("s", query)
	searchURL += "?" + params.Encode()

	doc, err := c.fetchHTML(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("dramacool search: %w", err)
	}

	var results []provider.Anime
	seen := make(map[string]bool)

	// WordPress search results: <h2><a href="...slug..." rel="bookmark">Title</a></h2>
	for _, h2 := range findNodes(doc, isElement("h2")) {
		a := findFirstElement(h2, isElement("a"))
		if a == nil {
			continue
		}
		href := attr(a, "href")
		title := strings.TrimSpace(textContent(a))
		if href == "" || title == "" {
			continue
		}

		id := slugFromURL(href)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		// Skip non-drama pages
		if strings.Contains(id, "category/") || strings.Contains(id, "tag/") ||
			strings.Contains(id, "/feed") || strings.Contains(id, "blog") {
			continue
		}

		results = append(results, provider.Anime{ID: id, Title: cleanTitle(title)})
	}

	// Fallback: ul.list-episode-item > li > a[href] + h3 (old layout)
	if len(results) == 0 {
		for _, ul := range findNodes(doc, hasClass("list-episode-item")) {
			for _, li := range findNodes(ul, isElement("li")) {
				a := findFirstElement(li, isElement("a"))
				if a == nil {
					continue
				}
				href := attr(a, "href")
				title := textContent(findFirstElement(li, isElement("h3")))
				id := slugFromURL(href)
				if id == "" || title == "" || seen[id] {
					continue
				}
				seen[id] = true
				results = append(results, provider.Anime{ID: id, Title: cleanTitle(title)})
			}
		}
	}

	// Fallback: ul.switch-block > li with h3.title and a[href]
	if len(results) == 0 {
		for _, ul := range findNodes(doc, hasClass("switch-block")) {
			for _, li := range findNodes(ul, isElement("li")) {
				a := findFirstElement(li, isElement("a"))
				if a == nil {
					continue
				}
				href := attr(a, "href")
				h3 := findFirstElement(li, hasClass("title"))
				if h3 == nil {
					h3 = findFirstElement(li, isElement("h3"))
				}
				title := textContent(h3)
				id := slugFromURL(href)
				if id == "" || title == "" || seen[id] {
					continue
				}
				seen[id] = true
				results = append(results, provider.Anime{ID: id, Title: cleanTitle(title)})
			}
		}
	}

	return results, nil
}

// Episodes returns episodes for a given drama.
func (c *Client) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	// Strip "drama-detail/" prefix if present.
	animeID = strings.TrimPrefix(animeID, "drama-detail/")

	detailURL := c.baseURL.JoinPath(animeID).String()

	doc, err := c.fetchHTML(ctx, detailURL)
	if err != nil {
		return nil, fmt.Errorf("dramacool episodes: %w", err)
	}

	var episodes []provider.Episode
	seen := make(map[string]bool)

	// Find episode list: div#episode-list > ... > ul.episode-list > li
	epList := findFirstElement(doc, hasClass("episode-list"))
	if epList == nil {
		return nil, fmt.Errorf("dramacool episodes: no episode list found")
	}

	for _, li := range findNodes(epList, isElement("li")) {
		a := findFirstElement(li, isElement("a"))
		if a == nil {
			continue
		}
		href := attr(a, "href")
		epSlug := slugFromURL(href)
		if epSlug == "" {
			continue
		}

		matches := episodeNumberRe.FindStringSubmatch(epSlug)
		if len(matches) < 2 {
			continue
		}
		number := matches[1]
		sortKey, err := strconv.ParseFloat(number, 64)
		if err != nil {
			continue
		}

		if seen[epSlug] {
			continue
		}
		seen[epSlug] = true

		episodes = append(episodes, provider.Episode{
			ID:      epSlug,
			Number:  number,
			SortKey: sortKey,
		})
	}

	// DramaCool lists newest first; reverse to get ascending order.
	for i, j := 0, len(episodes)-1; i < j; i, j = i+1, j-1 {
		episodes[i], episodes[j] = episodes[j], episodes[i]
	}

	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].SortKey < episodes[j].SortKey
	})

	return episodes, nil
}

// Streams resolves a DramaCool episode into downloadable media streams.
func (c *Client) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	// Try Xyra Stream API first (handles JS-rendered video servers)
	if c.apiURL != "" {
		streams, err := c.streamsViaAPI(ctx, episodeID)
		if err == nil && len(streams) > 0 {
			return streams, nil
		}
	}

	// Fallback: direct HTML scraping for server URLs
	watchURL := c.baseURL.JoinPath(episodeID).String()

	doc, err := c.fetchHTML(ctx, watchURL)
	if err != nil {
		return nil, fmt.Errorf("dramacool streams: %w", err)
	}

	// Find video servers: div.serverslist[data-server] or div[data-server]
	type serverInfo struct {
		name string
		url  string
	}
	var servers []serverInfo

	for _, div := range findNodes(doc, hasClass("serverslist")) {
		serverURL := attr(div, "data-server")
		if serverURL == "" {
			continue
		}
		if strings.HasPrefix(serverURL, "//") {
			serverURL = "https:" + serverURL
		}
		name := strings.TrimSpace(textContent(div))
		name = strings.TrimSuffix(name, "Choose this server")
		name = strings.TrimSpace(name)

		servers = append(servers, serverInfo{name: name, url: serverURL})
	}

	if len(servers) == 0 {
		// Fallback: look for iframe embeds
		for _, iframe := range findNodes(doc, isElement("iframe")) {
			src := attr(iframe, "src")
			if src == "" {
				continue
			}
			if strings.HasPrefix(src, "//") {
				src = "https:" + src
			}
			if !strings.HasPrefix(src, "http") {
				continue
			}
			servers = append(servers, serverInfo{name: "iframe", url: src})
		}
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("dramacool streams: no video servers found")
	}

	// Try the Standard server first, then others.
	sort.SliceStable(servers, func(i, j int) bool {
		return strings.Contains(strings.ToLower(servers[i].name), "standard")
	})

	headers := make(http.Header)
	headers.Set("Referer", c.baseURL.String())
	headers.Set("User-Agent", c.userAgent)

	for _, srv := range servers {
		// If the URL directly points to an m3u8, use it.
		if strings.Contains(strings.ToLower(srv.url), ".m3u8") {
			return []provider.Stream{{
				URL:     srv.url,
				Quality: "",
				Headers: headers,
			}}, nil
		}

		// Try fetching the server URL and looking for m3u8 streams.
		streams, err := c.resolveServerStreams(ctx, srv.url)
		if err != nil {
			continue
		}
		if len(streams) > 0 {
			return streams, nil
		}
	}

	return nil, fmt.Errorf("dramacool streams: no supported sources")
}

// resolveServerStreams fetches a server page and extracts m3u8 URLs.
func (c *Client) resolveServerStreams(ctx context.Context, serverURL string) ([]provider.Stream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create server request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Referer", c.baseURL.String()+"/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch server page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("server page status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read server page: %w", err)
	}

	body := string(data)
	matches := m3u8Re.FindAllString(body, -1)

	seen := make(map[string]bool)
	var streams []provider.Stream
	headers := make(http.Header)
	headers.Set("Referer", serverURL)
	headers.Set("User-Agent", c.userAgent)

	for _, rawURL := range matches {
		cleaned := strings.TrimRight(rawURL, "',;\")")
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		streams = append(streams, provider.Stream{
			URL:     cleaned,
			Quality: "",
			Headers: headers,
		})
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no m3u8 streams found in server page")
	}

	return streams, nil
}

// streamsViaAPI uses the Xyra Stream API to resolve episode streams.
func (c *Client) streamsViaAPI(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API URL: %w", err)
	}
	u = u.JoinPath("stream")
	q := u.Query()
	q.Set("episode_id", episodeID)
	if c.apiKey != "" {
		q.Set("api_key", c.apiKey)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read API response: %w", err)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    map[string]struct {
			Stream       string `json:"stream"`
			M3u8         bool   `json:"m3u8"`
			EmbeddedLink string `json:"embeded_link"` //nolint:misspell // API field is misspelled
			Skipped      bool   `json:"skipped"`
		} `json:"data"`
		HasM3u8 bool `json:"has_m3u8"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode API response: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Referer", c.baseURL.String()+"/")
	headers.Set("User-Agent", c.userAgent)

	var streams []provider.Stream
	for _, srv := range envelope.Data {
		if srv.Skipped {
			continue
		}
		if srv.Stream != "" && srv.M3u8 {
			streams = append(streams, provider.Stream{
				URL:     srv.Stream,
				Quality: "",
				Headers: headers,
			})
		}
	}

	return streams, nil
}

// fetchHTML fetches a URL and returns a parsed HTML document.
func (c *Client) fetchHTML(ctx context.Context, rawURL string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", c.baseURL.String()+"/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return doc, nil
}

// --- HTML parsing helpers (stdlib only) ---

// findNodes traverses the DOM tree and returns all nodes matching the predicate.
func findNodes(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if pred(node) {
			result = append(result, node)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return result
}

// findFirstElement returns the first descendant element matching the predicate.
func findFirstElement(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstElement(c, pred); found != nil {
			return found
		}
	}
	return nil
}

// isElement returns a predicate that matches a specific element type.
func isElement(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == tag
	}
}

// hasClass returns a predicate that matches nodes with a specific CSS class.
func hasClass(className string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		for _, a := range n.Attr {
			if a.Key == "class" {
				for _, c := range strings.Fields(a.Val) {
					if c == className {
						return true
					}
				}
			}
		}
		return false
	}
}

// attr returns the value of an HTML attribute.
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textContent returns all text content under a node.
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// slugFromURL extracts the drama/episode slug from a URL.
func slugFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	slug := parts[len(parts)-1]
	if slug == "" && len(parts) > 1 {
		slug = parts[len(parts)-2]
	}
	// Strip "drama-detail/" prefix.
	slug = strings.TrimPrefix(slug, "drama-detail/")
	return slug
}

// cleanTitle removes extra whitespace from a title.
func cleanTitle(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// isEmbedServer checks if a URL belongs to an embed server that may contain m3u8.
func isEmbedServer(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "plcool") ||
		strings.Contains(host, "asianload") ||
		strings.Contains(host, "gogoanime") ||
		strings.Contains(host, "gogoplay") ||
		strings.Contains(host, "streaming.php") ||
		strings.Contains(host, "embed")
}
