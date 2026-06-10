// Package jimaku implements a subtitle provider for jimaku.cc.
//
// jimaku.cc is a simple site that hosts anime subtitle files
// organized by entry.
//
// Structure:
//   - /                     → Home page listing all anime entries
//   - /entry/{id}           → Entry page listing subtitle files (.srt, .ass)
//   - /entry/{id}/download/{filename} → Direct subtitle file download
//
// There is no search API. "Search" fetches the home page entry list
// and filters by token overlap with the query.
package jimaku

import (
	"net/http"
	"time"

	"github.com/distiled/orphion/internal/common"
)

// Config holds configuration for the jimaku.cc client.
type Config struct {
	// BaseURL is the jimaku.cc root URL.
	BaseURL string

	// UserAgent is the HTTP User-Agent header.
	UserAgent string

	// Timeout is the HTTP client timeout.
	Timeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   common.JimakuSiteURL,
		UserAgent: common.DefaultUserAgent,
		Timeout:   common.JimakuTimeout,
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
