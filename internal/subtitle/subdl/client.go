package subdl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/subtitle"
)

var nextDataRe = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// Client fetches and parses SubDL data.
type Client struct {
	httpClient  *http.Client
	siteURL     string
	downloadURL string
	userAgent   string
}

// NewClient validates configuration and creates a SubDL client.
func NewClient(cfg Config) (*Client, error) {
	if _, err := parseEndpoint("site", cfg.SiteURL); err != nil {
		return nil, err
	}
	if _, err := parseEndpoint("download", cfg.DownloadURL); err != nil {
		return nil, err
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = common.DefaultUserAgent
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{
		httpClient:  cfg.HTTPClient,
		siteURL:     cfg.SiteURL,
		downloadURL: cfg.DownloadURL,
		userAgent:   cfg.UserAgent,
	}, nil
}

// Search searches SubDL for matching titles.
func (c *Client) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	pageURL := fmt.Sprintf("%s/search/%s", c.siteURL, pathEscape(query))
	data, err := c.fetchPageProps(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("subdl search: %w", err)
	}

	var props struct {
		List []struct {
			Type           string `json:"type"`
			SDID           string `json:"sd_id"`
			Name           string `json:"name"`
			OriginalName   string `json:"original_name"`
			Year           int    `json:"year"`
			Slug           string `json:"slug"`
			SubtitlesCount int    `json:"subtitles_count"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("subdl search: decode: %w", err)
	}

	results := make([]subtitle.Result, 0, len(props.List))
	for _, item := range props.List {
		if item.SDID == "" || item.Name == "" {
			continue
		}
		results = append(results, subtitle.Result{
			ID:       "subdl:" + item.SDID,
			Title:    item.Name,
			Type:     item.Type,
			Year:     item.Year,
			Slug:     item.Slug,
			SubCount: item.SubtitlesCount,
			Source:   "subdl",
		})
	}
	return results, nil
}

// Page fetches subtitle data (seasons + subtitles) for a show.
// seasonSlug can be empty (shows the default/home page) or a season like
// "first-season", "second-season", etc.
func (c *Client) Page(ctx context.Context, sdID, slug, seasonSlug string) (*subtitle.PageResult, error) {
	var pageURL string
	if seasonSlug != "" {
		pageURL = fmt.Sprintf("%s/subtitle/%s/%s/%s", c.siteURL, sdID, slug, seasonSlug)
	} else {
		pageURL = fmt.Sprintf("%s/subtitle/%s/%s", c.siteURL, sdID, slug)
	}

	data, err := c.fetchPageProps(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("subdl page: %w", err)
	}

	var props struct {
		MovieInfo struct {
			Type    string `json:"type"`
			SDID    int    `json:"sd_id"`
			Slug    string `json:"slug"`
			Name    string `json:"name"`
			Year    int    `json:"year"`
			Total   int    `json:"total_seasons"`
			Seasons []struct {
				Number string `json:"number"`
				Name   string `json:"name"`
			} `json:"seasons"`
		} `json:"movieInfo"`
		GroupedSubtitles json.RawMessage `json:"groupedSubtitles"`
	}
	if err := json.Unmarshal(data, &props); err != nil {
		return nil, fmt.Errorf("subdl page: decode: %w", err)
	}

	result := &subtitle.PageResult{}

	// Parse seasons.
	for _, s := range props.MovieInfo.Seasons {
		result.Seasons = append(result.Seasons, subtitle.Season{
			Slug: s.Number,
			Name: s.Name,
		})
	}

	// Parse subtitles from all languages.
	// SubDL returns groupedSubtitles as either {} (object) or [] (empty array).
	trimmed := bytes.TrimSpace(props.GroupedSubtitles)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var grouped map[string][]subdlSubtitle
		if err := json.Unmarshal(trimmed, &grouped); err == nil {
			for _, items := range grouped {
				for _, item := range items {
					result.Subtitles = append(result.Subtitles, subtitle.Subtitle{
						ID:         item.ID,
						Language:   item.Language,
						Quality:    item.Quality,
						Link:       item.Link,
						BucketLink: item.BucketLink,
						Author:     item.Author,
						Season:     item.Season,
						Episode:    item.Episode,
						Title:      item.Title,
						Downloads:  item.Downloads,
						Releases:   item.Releases,
						Source:     "subdl",
					})
				}
			}
		}
	}

	return result, nil
}

// DownloadURL returns the direct download URL for a subtitle entry.
func (c *Client) DownloadURL(sub subtitle.Subtitle) string {
	return fmt.Sprintf("%s/subtitle/%s", c.downloadURL, sub.Link)
}

// fetchPageProps fetches a SubDL page and extracts the pageProps JSON from
// the embedded __NEXT_DATA__ script tag.
func (c *Client) fetchPageProps(ctx context.Context, pageURL string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	matches := nextDataRe.FindSubmatch(body)
	if len(matches) < 2 {
		return nil, fmt.Errorf("no __NEXT_DATA__ found in page")
	}

	var envelope struct {
		Props struct {
			PageProps json.RawMessage `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(matches[1], &envelope); err != nil {
		return nil, fmt.Errorf("parse __NEXT_DATA__: %w", err)
	}

	return envelope.Props.PageProps, nil
}

// pathEscape escapes a search query for SubDL URL paths.
func pathEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "%20", "+")
}

// subdlSubtitle maps the SubDL JSON subtitle object.
type subdlSubtitle struct {
	ID         int      `json:"id"`
	Language   string   `json:"language"`
	Quality    string   `json:"quality"`
	Link       string   `json:"link"`
	BucketLink string   `json:"bucketLink"`
	Author     string   `json:"author"`
	Season     int      `json:"season"`
	Episode    int      `json:"episode"`
	Title      string   `json:"title"`
	Downloads  int      `json:"downloads"`
	Releases   []string `json:"releases"`
}
