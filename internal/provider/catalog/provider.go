package catalog

import (
	"context"

	"github.com/distiled/orphion/internal/provider"
)

// Provider wraps the catalog client to satisfy the provider.Provider interface.
type Provider struct {
	client *Client
}

// NewProvider creates a provider backed by the catalog client.
func NewProvider(cfg Config) *Provider {
	client := NewClient(cfg)
	return &Provider{client: client}
}

// Search searches for anime titles.
func (p *Provider) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	return p.client.Search(ctx, query, kind)
}

// Episodes returns episodes for an anime.
func (p *Provider) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	return p.client.Episodes(ctx, animeID)
}

// Streams returns quality streams for an episode.
func (p *Provider) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	// TODO: Remove catalog prefix from IDs.
	return p.client.Streams(ctx, episodeID)
}

// Ensure Provider satisfies the provider.Provider interface.
var _ provider.Provider = (*Provider)(nil)

// LiveTest requires opt-in to run against the real upstream.
func LiveTest(ctx context.Context) error {
	return nil
}