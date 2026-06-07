// Package subdl implements the SubDL subtitle provider.
package subdl

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/distiled/orphion/internal/common"
)

// Config contains SubDL provider configuration.
type Config struct {
	SiteURL     string
	DownloadURL string
	UserAgent   string
	Timeout     time.Duration
	HTTPClient  *http.Client
}

// DefaultConfig returns production SubDL configuration.
func DefaultConfig() Config {
	return Config{
		SiteURL:     common.SubDLSiteURL,
		DownloadURL: common.SubDLDownloadURL,
		UserAgent:   common.DefaultUserAgent,
		Timeout:     common.DefaultHTTPTimeout,
	}
}

func parseEndpoint(name, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid %s URL", name)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid %s URL scheme", name)
	}
	return u, nil
}
