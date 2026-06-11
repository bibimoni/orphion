package allanime

import (
	"context"

	"github.com/bibimoni/orphion/internal/provider"
)

// Provider implements the normalized provider contract.
type Provider struct {
	client *Client
}

// NewProvider creates an AllAnime provider.
func NewProvider(cfg Config) (*Provider, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{client: client}, nil
}

func (p *Provider) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	return p.client.Search(ctx, query, kind)
}

func (p *Provider) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	return p.client.Episodes(ctx, animeID)
}

func (p *Provider) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	return p.client.Streams(ctx, episodeID)
}

var _ provider.Provider = (*Provider)(nil)
