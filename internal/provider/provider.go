// Package provider defines the catalog provider interface.
package provider

import (
	"context"
	"net/http"
)

// Anime represents a search result.
type Anime struct {
	ID    string
	Title string
}

// Episode belongs to an anime title.
type Episode struct {
	ID      string
	Number  string
	SortKey float64
}

// Stream represents a downloadable quality variant.
type Stream struct {
	URL     string
	Quality string
	Headers http.Header
}

// Provider defines the content-catalog contract.
type Provider interface {
	Search(ctx context.Context, query, kind string) ([]Anime, error)
	Episodes(ctx context.Context, animeID string) ([]Episode, error)
	Streams(ctx context.Context, episodeID string) ([]Stream, error)
}
