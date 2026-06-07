// Package subtitle defines the subtitle provider interface and types.
package subtitle

import "context"

// Result represents a subtitle search result (a show/movie with subtitles).
type Result struct {
	ID       string // e.g. "sd1300065"
	Title    string // e.g. "Naruto"
	Type     string // "tv" or "movie"
	Year     int
	Slug     string // URL slug, e.g. "naruto"
	SubCount int    // number of subtitles available
	Source   string // provider name, e.g. "subdl", "kitsunekko"
}

// Season represents a season within a show.
type Season struct {
	Slug string // e.g. "first-season"
	Name string // e.g. "Season 1"
}

// Subtitle represents a single subtitle entry.
type Subtitle struct {
	ID         int
	Language   string // e.g. "english"
	Quality    string // e.g. "webdl", "bluray", "other"
	Link       string // download filename, e.g. "3455495-8378310.zip"
	BucketLink string // alternative path, e.g. "3455495/8378310.zip"
	Author     string
	Season     int
	Episode    int // 0 = all episodes
	Title      string
	Downloads  int
	Releases   []string
	Source     string // provider name, e.g. "subdl", "kitsunekko"
}

// PageResult contains both seasons and subtitles from a subtitle page.
type PageResult struct {
	Seasons   []Season
	Subtitles []Subtitle
}

// Provider defines the subtitle catalog contract.
type Provider interface {
	Search(ctx context.Context, query string) ([]Result, error)
	Page(ctx context.Context, sdID, slug, seasonSlug string) (*PageResult, error)
	DownloadURL(sub Subtitle) string
}
