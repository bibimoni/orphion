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
	Title   string
	Size    string
}

// Stream represents a downloadable quality variant.
type Stream struct {
	URL       string
	AudioURL  string
	Quality   string
	Bandwidth int64 // bits per second from HLS BANDWIDTH attribute (0 = unknown)
	Headers   http.Header
}

// Provider defines the content-catalog contract.
type Provider interface {
	Search(ctx context.Context, query, kind string) ([]Anime, error)
	Episodes(ctx context.Context, animeID string) ([]Episode, error)
	Streams(ctx context.Context, episodeID string) ([]Stream, error)
}

// SegmentProgressFunc reports preparation progress for segmented streams.
type SegmentProgressFunc func(done, total int)

// StreamPreparer is implemented by providers that need to materialize or
// rewrite a selected stream before FFmpeg can consume it.
type StreamPreparer interface {
	PrepareStream(ctx context.Context, stream Stream, progress SegmentProgressFunc) (Stream, error)
}
