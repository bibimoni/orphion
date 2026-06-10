package jimaku

import (
	"context"

	"github.com/distiled/orphion/internal/subtitle"
)

// Provider implements the subtitle.Provider interface for jimaku.cc.
type Provider struct {
	client *Client
}

// NewProvider creates a jimaku.cc subtitle provider.
func NewProvider(cfg Config) (*Provider, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{client: client}, nil
}

// Search searches jimaku.cc for matching anime titles.
func (p *Provider) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	return p.client.Search(ctx, query)
}

// Page returns subtitle files for a specific anime entry.
func (p *Provider) Page(ctx context.Context, id, slug, seasonSlug string) (*subtitle.PageResult, error) {
	return p.client.Page(ctx, id, slug, seasonSlug)
}

// DownloadURL returns the direct download URL for a subtitle file.
func (p *Provider) DownloadURL(sub subtitle.Subtitle) string {
	return p.client.DownloadURL(sub)
}

var _ subtitle.Provider = (*Provider)(nil)
