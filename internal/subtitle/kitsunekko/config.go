// Package kitsunekko implements a subtitle provider for kitsunekko.net.
//
// kitsunekko.net is a simple Apache directory listing site that hosts
// subtitle archives organized by language and anime title.
//
// Structure:
//   - /subtitles/              → English subtitle directories
//   - /subtitles/japanese/     → Japanese subtitle directories
//   - /subtitles/japanese/{Title}/ → .zip files for a specific anime
//
// There is no search API. "Search" lists all directories and fuzzy-matches
// the query against directory names.
package kitsunekko

import (
	"net/http"
	"time"

	"github.com/bibimoni/orphion/internal/common"
)

// Config holds configuration for the kitsunekko client.
type Config struct {
	// BaseURL is the kitsunekko.net root URL.
	BaseURL string

	// Languages lists the language paths to search.
	// Each entry maps to a subdirectory under /subtitles/.
	// e.g. "japanese" → /subtitles/japanese/, "" → /subtitles/
	Languages []string

	// UserAgent is the HTTP User-Agent header.
	UserAgent string

	// Timeout is the HTTP client timeout.
	Timeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   common.KitsunekkoSiteURL,
		Languages: []string{"japanese"},
		UserAgent: common.DefaultUserAgent,
		Timeout:   common.KitsunekkoTimeout,
	}
}

// HTTPClient returns an *http.Client configured from the Config.
func (c Config) HTTPClient() *http.Client {
	return &http.Client{
		Timeout: c.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}
