package subdl

import (
	"context"

	"github.com/bibimoni/orphion/internal/subtitle"
)

// Provider implements the subtitle.Provider interface for SubDL.
type Provider struct {
	client *Client
}

// NewProvider creates a SubDL subtitle provider.
func NewProvider(cfg Config) (*Provider, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{client: client}, nil
}

// Search searches SubDL for matching titles.
func (p *Provider) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	return p.client.Search(ctx, query)
}

// Page returns seasons and subtitles for a show.
func (p *Provider) Page(ctx context.Context, sdID, slug, seasonSlug string) (*subtitle.PageResult, error) {
	return p.client.Page(ctx, sdID, slug, seasonSlug)
}

// DownloadURL returns the direct download URL for a subtitle.
func (p *Provider) DownloadURL(sub subtitle.Subtitle) string {
	return p.client.DownloadURL(sub)
}

var _ subtitle.Provider = (*Provider)(nil)
