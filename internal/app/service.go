package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/distiled/orphion/internal/download"
	"github.com/distiled/orphion/internal/episode"
	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/paths"
	"github.com/distiled/orphion/internal/provider"
	"github.com/distiled/orphion/internal/quality"
)

// Config holds application service configuration.
type Config struct {
	OutputDir    string
	Concurrency  int
	PreferredQty string
	Force        bool
}

// ProgressCallback receives progress updates during a download.
type ProgressCallback func(episode string, progress ffmpeg.Progress)

// Service orchestrates content lookup and download.
type Service struct {
	provider   provider.Provider
	scheduler  *download.Scheduler
	runner     *ffmpeg.Runner
	config     Config
	progressCb ProgressCallback
}

// New creates a new app service.
func New(p provider.Provider, runner *ffmpeg.Runner, cfg Config) *Service {
	return &Service{
		provider:  p,
		scheduler: download.NewScheduler(cfg.Concurrency),
		runner:    runner,
		config:    cfg,
	}
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
	Missing   []string
	Outputs   []string         // Output file paths for completed downloads
	Errors    map[string]error // Episode number → error for failed downloads
}

// Search performs a content search.
func (s *Service) Search(ctx context.Context, query, kind string) (SearchResult, error) {
	results, err := s.provider.Search(ctx, query, kind)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search %s/%s: %w", kind, query, err)
	}
	return SearchResult{Anime: results}, nil
}

// ResolveID resolves a search query to a single anime ID.
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

// SetProgressCallback sets the callback invoked with download progress updates.
func (s *Service) SetProgressCallback(fn ProgressCallback) {
	s.progressCb = fn
}

// DownloadEpisodes downloads selected episodes for a given title ID.
func (s *Service) DownloadEpisodes(ctx context.Context, animeID, expr, title string) (DownloadResult, []download.Result, error) {
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

	// Detect missing requested episode numbers.
	reqNums := make(map[string]bool)
	for _, seq := range req.Seqs {
		for n := parseSortKey(seq.Start); n <= parseSortKey(seq.End); n++ {
			reqNums[fmt.Sprintf("%.2f", n)] = true
		}
	}
	var missing []string
	for num := range reqNums {
		found := false
		for _, ep := range selected {
			if fmt.Sprintf("%.2f", ep.SortKey) == num {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, num)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "warning: missing episodes: %v\n", missing)
	}

	jobs := make([]download.Job, len(selected))
	for i, ep := range selected {
		jobs[i] = download.Job{
			ID:      ep.ID,
			Episode: ep.Number,
			Title:   title,
		}
	}

	// Track output paths across concurrent downloads.
	var outputMu sync.Mutex
	outputPaths := make(map[string]string)

	runner := download.RunnerFunc(func(ctx context.Context, job download.Job) error {
		outPath, err := s.executeJob(ctx, job)
		if err != nil {
			return err
		}
		if outPath != "" {
			outputMu.Lock()
			outputPaths[job.ID] = outPath
			outputMu.Unlock()
		}
		return nil
	})

	results := s.scheduler.RunAll(ctx, jobs, runner)

	// Populate output paths into results.
	for i := range results {
		if p, ok := outputPaths[results[i].JobID]; ok {
			results[i].OutputPath = p
		}
	}

	var dr DownloadResult
	dr.Errors = make(map[string]error)
	for i, r := range results {
		epNum := jobs[i].Episode
		switch r.Status {
		case download.StatusCompleted:
			dr.Completed++
			if r.OutputPath != "" {
				dr.Outputs = append(dr.Outputs, r.OutputPath)
			}
		case download.StatusFailed:
			dr.Failed++
			if r.Err != nil {
				dr.Errors[epNum] = r.Err
			}
		case download.StatusCancelled:
			dr.Cancelled++
		}
	}
	dr.Missing = missing
	return dr, results, nil
}

func (s *Service) executeJob(ctx context.Context, job download.Job) (string, error) {
	// Notify the UI that we're starting this episode.
	if s.progressCb != nil {
		s.progressCb(job.Episode, ffmpeg.Progress{})
	}

	streams, err := s.provider.Streams(ctx, job.ID)
	if err != nil {
		return "", fmt.Errorf("get streams: %w", err)
	}
	if len(streams) == 0 {
		return "", fmt.Errorf("no streams for episode %s", job.Episode)
	}

	qualityStreams := make([]quality.Stream, len(streams))
	for j, st := range streams {
		qualityStreams[j] = quality.Stream{
			URL:     st.URL,
			Quality: st.Quality,
		}
	}
	result := quality.Select(s.config.PreferredQty, qualityStreams)

	// Find matching provider stream for URL and headers.
	var streamURL string
	var selectedHeaders http.Header
	for _, st := range streams {
		if st.Quality == result.Stream.Quality {
			streamURL = st.URL
			selectedHeaders = st.Headers
			break
		}
	}

	baseDir := expandTilde(s.config.OutputDir)
	title := job.Title
	if title == "" {
		title = "Download"
	}

	outPath := paths.OutputLayout(baseDir, title, job.Episode)
	partPath := paths.PartialPath(baseDir, title, job.Episode)

	// Skip if final file already exists, unless forced.
	if !s.config.Force {
		if _, err := os.Stat(outPath); err == nil {
			return outPath, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(partPath), 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Build FFmpeg args for the selected stream.
	if s.progressCb != nil {
		args := s.runner.ProgressArgs(streamURL, partPath, selectedHeaders.Get("Referer"), selectedHeaders.Get("User-Agent"))
		progressFn := func(p ffmpeg.Progress) {
			s.progressCb(job.Episode, p)
		}
		if err := s.runner.ExecuteWithProgress(ctx, args, progressFn); err != nil {
			_ = os.Remove(partPath)
			return "", fmt.Errorf("ffmpeg: %w", err)
		}
	} else {
		args := s.runner.Args(streamURL, partPath, selectedHeaders.Get("Referer"), selectedHeaders.Get("User-Agent"))
		if err := s.runner.Execute(ctx, args); err != nil {
			_ = os.Remove(partPath)
			return "", fmt.Errorf("ffmpeg: %w", err)
		}
	}

	// Atomic rename.
	if err := os.Rename(partPath, outPath); err != nil {
		_ = os.Remove(partPath)
		return "", fmt.Errorf("rename: %w", err)
	}

	return outPath, nil
}

// SetOutputDir updates the output directory.
func (s *Service) SetOutputDir(dir string) {
	s.config.OutputDir = dir
}

// SetPreferredQuality updates the preferred quality.
func (s *Service) SetPreferredQuality(q string) {
	s.config.PreferredQty = q
}

// Config returns a copy of the current service config.
func (s *Service) Config() Config {
	return s.config
}

// SetConcurrency updates the download concurrency.
func (s *Service) SetConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	s.config.Concurrency = n
	s.scheduler = download.NewScheduler(n)
}

// GetEpisodes returns episode info for an anime.
func (s *Service) GetEpisodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	return s.provider.Episodes(ctx, animeID)
}

// SetForce enables or disables force overwrite of existing files.
func (s *Service) SetForce(v bool) {
	s.config.Force = v
}

// OutputDir returns the configured output directory.
func (s *Service) OutputDir() string {
	return expandTilde(s.config.OutputDir)
}

func parseSortKey(s string) float64 {
	var val float64
	fmt.Sscanf(s, "%f", &val)
	return val
}

func expandTilde(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
