package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/distiled/orphion/internal/download"
	"github.com/distiled/orphion/internal/episode"
	"github.com/distiled/orphion/internal/provider"
	"github.com/distiled/orphion/internal/quality"
)

// Config holds application service configuration.
type Config struct {
	OutputDir    string
	Concurrency  int
	PreferredQty string
}

// Service orchestrates content lookup and download.
type Service struct {
	provider  provider.Provider
	scheduler *download.Scheduler
	config    Config
}

// New creates a new app service.
func New(p provider.Provider, cfg Config) *Service {
	return &Service{
		provider:  p,
		scheduler: download.NewScheduler(cfg.Concurrency),
		config:    cfg,
	}
}

// EpisodeJob represents a single episode to download.
type EpisodeJob struct {
	EpisodeNumber string
	Title         string
	URL           string
	Quality       string
}

// SearchResult holds the result of a search operation.
type SearchResult struct {
	Anime []provider.Anime
}

// DownloadResult holds aggregated download results.
type DownloadResult struct {
	Completed int
	Failed    int
	Skipped   int
	Cancelled int
}

// Search performs a content search.
func (s *Service) Search(ctx context.Context, query, kind string) (SearchResult, error) {
	results, err := s.provider.Search(ctx, query, kind)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search %s/%s: %w", kind, query, err)
	}
	return SearchResult{Anime: results}, nil
}

// ResolveID resolves a search query to a single anime ID, or returns
// an error if the query returns zero or multiple results.
func (s *Service) ResolveID(ctx context.Context, query, kind string) (string, error) {
	results, err := s.provider.Search(ctx, query, kind)
	if err != nil {
		return "", fmt.Errorf("resolve %s/%s: %w", kind, query, err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no results for %s %q", kind, query)
	}
	if len(results) > 1 {
		return "", fmt.Errorf("ambiguous search for %s %q: %d results", kind, query, len(results))
	}
	return results[0].ID, nil
}

// DownloadEpisodes downloads selected episodes for a given title ID.
func (s *Service) DownloadEpisodes(ctx context.Context, animeID, expr string) (DownloadResult, []download.Result, error) {
	eps, err := s.provider.Episodes(ctx, animeID)
	if err != nil {
		return DownloadResult{}, nil, fmt.Errorf("get episodes: %w", err)
	}

	p := episode.Parser{}
	req, err := p.Parse(expr)
	if err != nil {
		return DownloadResult{}, nil, fmt.Errorf("parse episodes: %w", err)
	}

	episodes := make([]episode.Episode, len(eps))
	for i, e := range eps {
		episodes[i] = episode.Episode{
			ID:      e.ID,
			Number:  e.Number,
			SortKey: e.SortKey,
		}
	}
	selected := episode.Resolve(req, episodes)
	if len(selected) == 0 {
		return DownloadResult{}, nil, fmt.Errorf("no episodes matching %q", expr)
	}

	var mu sync.Mutex
	jobs := make([]download.Job, len(selected))
	for i, ep := range selected {
		jobs[i] = download.Job{
			ID:      ep.ID,
			Episode: ep.Number,
		}
	}

	runner := download.RunnerFunc(func(ctx context.Context, job download.Job) error {
		// Get streams for the episode.
		streams, err := s.provider.Streams(ctx, job.ID)
		if err != nil {
			return err
		}
		if len(streams) == 0 {
			return fmt.Errorf("no streams for episode %s", job.Episode)
		}

		// Select quality.
		qualityStreams := make([]quality.Stream, len(streams))
		for j, s := range streams {
			qualityStreams[j] = quality.Stream{
				URL:     s.URL,
				Quality: s.Quality,
			}
		}
		result := quality.Select(s.config.PreferredQty, qualityStreams)

		mu.Lock()
		defer mu.Unlock()
		// In a full implementation, we would pass the stream URL and headers to the
		// FFmpeg runner. For now, we record the download job.
		fmt.Printf("Episode %s: %s %s\n", job.Episode, result.Stream.Quality, result.Stream.URL)

		return nil
	})

	results := s.scheduler.RunAll(ctx, jobs, runner)

	var dr DownloadResult
	for _, r := range results {
		switch r.Status {
		case download.StatusCompleted:
			dr.Completed++
		case download.StatusFailed:
			dr.Failed++
		case download.StatusCancelled:
			dr.Cancelled++
		}
	}
	return dr, results, nil
}

// GetEpisodes returns episode info for an anime.
func (s *Service) GetEpisodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	return s.provider.Episodes(ctx, animeID)
}
