// Package app implements the core application services for Orphion.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/pterm/pterm"

	"github.com/bibimoni/orphion/internal/common"
	"github.com/bibimoni/orphion/internal/download"
	"github.com/bibimoni/orphion/internal/episode"
	"github.com/bibimoni/orphion/internal/ffmpeg"
	"github.com/bibimoni/orphion/internal/paths"
	"github.com/bibimoni/orphion/internal/provider"
	"github.com/bibimoni/orphion/internal/quality"
	"github.com/bibimoni/orphion/internal/subtitle"
)

// Config holds application service configuration.
type Config struct {
	OutputDir    string
	Concurrency  int
	PreferredQty string
	Force        bool
	ProviderName string
	Providers    map[string]provider.Provider
	SubtitleLang string
	SubtitleSrc  subtitle.Provider
}

// ProgressCallback receives progress updates during a download.
type ProgressCallback func(episode string, progress ffmpeg.Progress)

// CompletedCallback is called when an episode download finishes successfully.
type CompletedCallback func(episode string)

// Service orchestrates content lookup and download.
type Service struct {
	provider     provider.Provider
	scheduler    *download.Scheduler
	runner       *ffmpeg.Runner
	config       Config
	progressCb   ProgressCallback
	completedCb  CompletedCallback
	providerName string
	providers    map[string]provider.Provider
	subtitleSrc  subtitle.Provider
}

// New creates a new app service.
func New(p provider.Provider, runner *ffmpeg.Runner, cfg Config) *Service {
	providers := make(map[string]provider.Provider, len(cfg.Providers))
	for name, provider := range cfg.Providers {
		providers[name] = provider
	}
	if cfg.ProviderName != "" && p != nil {
		providers[cfg.ProviderName] = p
	}
	return &Service{
		provider:     p,
		scheduler:    download.NewScheduler(cfg.Concurrency),
		runner:       runner,
		config:       cfg,
		providerName: cfg.ProviderName,
		providers:    providers,
		subtitleSrc:  cfg.SubtitleSrc,
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
	Canceled  int
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
//
// When a search returns multiple results, an exact title match (case- and
// punctuation-insensitive) is used to disambiguate. This lets users resolve
// shows like "Sentenced to Be a Hero" even when a sequel ("... Season 2") is
// returned alongside it. If no exact match exists, the ambiguity is reported.
func (s *Service) ResolveID(ctx context.Context, query, kind string) (string, error) {
	results, err := s.provider.Search(ctx, query, kind)
	if err != nil {
		return "", fmt.Errorf("resolve %s/%s: %w", kind, query, err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no results for %s %q", kind, query)
	}
	if len(results) > 1 {
		if id, ok := resolveByExactTitle(query, results); ok {
			return id, nil
		}
		return "", fmt.Errorf("ambiguous search for %s %q: %d results", kind, query, len(results))
	}
	return results[0].ID, nil
}

// resolveByExactTitle returns the ID of the single result whose normalized
// title matches the query exactly. Returns ok=false when there is no match,
// or when more than one result matches (ambiguous even after normalization).
func resolveByExactTitle(query string, results []provider.Anime) (string, bool) {
	want := normalizeTitle(query)
	if want == "" {
		return "", false
	}
	var match string
	for _, r := range results {
		if normalizeTitle(r.Title) != want {
			continue
		}
		if match != "" {
			return "", false // more than one exact match
		}
		match = r.ID
	}
	if match == "" {
		return "", false
	}
	return match, true
}

// normalizeTitle lowercases a title and strips all non-alphanumeric
// characters (replacing them with single spaces) so that differences in
// case, punctuation, and spacing do not prevent an exact match.
func normalizeTitle(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	prevSpace := false
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// SetProgressCallback sets the callback invoked with download progress updates.
func (s *Service) SetProgressCallback(fn ProgressCallback) {
	s.progressCb = fn
}

// SetCompletedCallback sets the callback invoked when an episode download succeeds.
func (s *Service) SetCompletedCallback(fn CompletedCallback) {
	s.completedCb = fn
}

// SetProvider switches the active content provider by registered name.
func (s *Service) SetProvider(name string) error {
	p, ok := s.providers[name]
	if !ok {
		return fmt.Errorf("provider %q not found", name)
	}
	s.provider = p
	s.providerName = name
	return nil
}

// ProviderName returns the active provider name, if known.
func (s *Service) ProviderName() string {
	return s.providerName
}

// ProviderNames returns the registered provider names in stable UI order.
func (s *Service) ProviderNames() []string {
	names := make([]string, 0, len(s.providers))
	if s.providerName != "" {
		if _, ok := s.providers[s.providerName]; ok {
			names = append(names, s.providerName)
		}
	}
	for _, name := range []string{"allanime", "catalog", "bettermelon"} {
		if name == s.providerName {
			continue
		}
		if _, ok := s.providers[name]; ok {
			names = append(names, name)
		}
	}
	for name := range s.providers {
		if name == s.providerName || name == "allanime" || name == "catalog" || name == "bettermelon" {
			continue
		}
		names = append(names, name)
	}
	return names
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
	selectedProviderEpisodes := make([]provider.Episode, len(selected))
	for i, ep := range selected {
		selectedProviderEpisodes[i] = provider.Episode{
			ID:      ep.ID,
			Number:  ep.Number,
			SortKey: ep.SortKey,
		}
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

	return s.downloadEpisodeList(ctx, selectedProviderEpisodes, title, missing)
}

// DownloadSelectedEpisodes downloads provider episodes selected directly by ID.
func (s *Service) DownloadSelectedEpisodes(ctx context.Context, selected []provider.Episode, title string) (DownloadResult, []download.Result, error) {
	if len(selected) == 0 {
		return DownloadResult{}, nil, fmt.Errorf("no episodes selected")
	}
	return s.downloadEpisodeList(ctx, selected, title, nil)
}

func (s *Service) downloadEpisodeList(ctx context.Context, selected []provider.Episode, title string, missing []string) (DownloadResult, []download.Result, error) {
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
			dr.Canceled++
		}
	}
	dr.Missing = missing
	return dr, results, nil
}

func (s *Service) executeJob(ctx context.Context, job download.Job) (string, error) {
	// Notify the UI that we're starting this episode.
	if s.progressCb != nil {
		s.progressCb(job.Episode, ffmpeg.Progress{Phase: "resolving"})
	}

	baseDir := paths.ExpandTilde(s.config.OutputDir)
	title := job.Title
	if title == "" {
		title = "Download"
	}

	outPath := paths.OutputLayout(baseDir, title, job.Episode)
	partPath := paths.PartialPath(baseDir, title, job.Episode)

	// Skip before resolving remote streams when the final file already exists.
	if !s.config.Force {
		if _, err := os.Stat(outPath); err == nil {
			return outPath, nil
		}
	}

	streams, err := s.provider.Streams(ctx, job.ID)
	if err != nil {
		return "", fmt.Errorf("get streams: %w", err)
	}
	if len(streams) == 0 {
		return "", fmt.Errorf("no streams for episode %s", job.Episode)
	}

	candidates := orderStreams(s.config.PreferredQty, streams)
	if len(candidates) == 0 {
		return "", fmt.Errorf("selected stream is unavailable")
	}
	selectedStream := candidates[0]

	if preparer, ok := s.provider.(provider.StreamPreparer); ok {
		var prepareErr error
		for _, candidate := range candidates {
			prepared, err := preparer.PrepareStream(ctx, candidate, func(done, total int) {
				if s.progressCb != nil {
					s.progressCb(job.Episode, ffmpeg.Progress{
						Phase:         "segments",
						SegmentsDone:  done,
						SegmentsTotal: total,
					})
				}
			})
			if err == nil {
				selectedStream = prepared
				prepareErr = nil
				break
			}
			cleanupTempInput(prepared.URL)
			prepareErr = err
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
		}
		if prepareErr != nil {
			return "", fmt.Errorf("prepare stream: %w", prepareErr)
		}
	}
	streamURL := selectedStream.URL
	selectedHeaders := selectedStream.Headers

	if err := os.MkdirAll(filepath.Dir(partPath), 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Guard against nil runner (ffmpeg not installed).
	if s.runner == nil {
		return "", fmt.Errorf("ffmpeg runner not available: install ffmpeg and ensure it is on your PATH")
	}

	// Build FFmpeg args for the selected stream.
	if s.progressCb != nil {
		args := s.runner.ProgressArgs(streamURL, partPath, selectedHeaders.Get("Referer"), selectedHeaders.Get("User-Agent"))
		progressFn := func(p ffmpeg.Progress) {
			s.progressCb(job.Episode, p)
		}
		if err := s.runner.ExecuteWithProgress(ctx, args, progressFn); err != nil {
			cleanupTempInput(streamURL)
			_ = os.Remove(partPath)
			return "", fmt.Errorf("ffmpeg: %w", err)
		}
	} else {
		args := s.runner.Args(streamURL, partPath, selectedHeaders.Get("Referer"), selectedHeaders.Get("User-Agent"))
		if err := s.runner.Execute(ctx, args); err != nil {
			cleanupTempInput(streamURL)
			_ = os.Remove(partPath)
			return "", fmt.Errorf("ffmpeg: %w", err)
		}
	}

	// Clean up temp input files (e.g. bettermelon-*.ts segment cache).
	cleanupTempInput(streamURL)

	// Atomic rename.
	if err := os.Rename(partPath, outPath); err != nil {
		_ = os.Remove(partPath)
		return "", fmt.Errorf("rename: %w", err)
	}

	// Notify the UI that this episode is done.
	if s.completedCb != nil {
		s.completedCb(job.Episode)
	}

	return outPath, nil
}

func orderStreams(preferred string, streams []provider.Stream) []provider.Stream {
	remaining := append([]provider.Stream(nil), streams...)
	ordered := make([]provider.Stream, 0, len(streams))

	for len(remaining) > 0 {
		qualityStreams := make([]quality.Stream, len(remaining))
		for i, stream := range remaining {
			qualityStreams[i] = quality.Stream{
				URL:       stream.URL,
				Quality:   stream.Quality,
				Bandwidth: stream.Bandwidth,
			}
		}
		selected := quality.Select(preferred, qualityStreams).Stream
		selectedIndex := -1
		for i, stream := range remaining {
			if stream.URL == selected.URL &&
				stream.Quality == selected.Quality &&
				stream.Bandwidth == selected.Bandwidth {
				selectedIndex = i
				break
			}
		}
		if selectedIndex < 0 {
			ordered = append(ordered, remaining...)
			break
		}
		ordered = append(ordered, remaining[selectedIndex])
		remaining = append(remaining[:selectedIndex], remaining[selectedIndex+1:]...)
	}
	return ordered
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
	return paths.ExpandTilde(s.config.OutputDir)
}

// SubtitleProvider returns the configured subtitle provider, or nil.
func (s *Service) SubtitleProvider() subtitle.Provider {
	return s.subtitleSrc
}

// SubtitleLang returns the preferred subtitle language.
func (s *Service) SubtitleLang() string {
	if s.config.SubtitleLang == "" {
		return common.DefaultSubtitleLang
	}
	return s.config.SubtitleLang
}

// SetSubtitleLang updates the preferred subtitle language.
func (s *Service) SetSubtitleLang(lang string) {
	s.config.SubtitleLang = lang
}

// SearchSubtitles searches for subtitle results matching the query.
func (s *Service) SearchSubtitles(ctx context.Context, query string) ([]subtitle.Result, error) {
	if s.subtitleSrc == nil {
		return nil, fmt.Errorf("subtitle provider not configured")
	}
	return s.subtitleSrc.Search(ctx, query)
}

// SubtitlePage returns seasons and subtitles for a given show.
// If the base page returns seasons but no subtitles, it automatically
// fetches the first non-specials season.
func (s *Service) SubtitlePage(ctx context.Context, sdID, slug, seasonSlug string) (*subtitle.PageResult, error) {
	if s.subtitleSrc == nil {
		return nil, fmt.Errorf("subtitle provider not configured")
	}

	page, err := s.subtitleSrc.Page(ctx, sdID, slug, seasonSlug)
	if err != nil {
		return nil, err
	}

	// If we got subtitles, return as-is.
	if len(page.Subtitles) > 0 {
		return page, nil
	}

	// If no subtitles and no explicit season was requested, try the first
	// non-specials season. Many shows on SubDL only have subtitles under
	// season-specific pages.
	if seasonSlug == "" && len(page.Seasons) > 0 {
		for _, season := range page.Seasons {
			// Skip "specials" seasons — users typically want the main show.
			if strings.Contains(strings.ToLower(season.Slug), "special") {
				continue
			}
			pterm.Debug.Printfln("Trying season %s...", season.Name)
			seasonPage, err := s.subtitleSrc.Page(ctx, sdID, slug, season.Slug)
			if err != nil {
				continue
			}
			if len(seasonPage.Subtitles) > 0 {
				// Merge seasons info from the base page.
				seasonPage.Seasons = page.Seasons
				return seasonPage, nil
			}
		}

		// If all non-special seasons were empty, try specials as last resort.
		for _, season := range page.Seasons {
			if !strings.Contains(strings.ToLower(season.Slug), "special") {
				continue
			}
			seasonPage, err := s.subtitleSrc.Page(ctx, sdID, slug, season.Slug)
			if err != nil {
				continue
			}
			if len(seasonPage.Subtitles) > 0 {
				seasonPage.Seasons = page.Seasons
				return seasonPage, nil
			}
		}
	}

	return page, nil
}

// DownloadSubtitle downloads and extracts subtitle files into outputDir.
func (s *Service) DownloadSubtitle(ctx context.Context, sub subtitle.Subtitle, outputDir string) ([]string, error) {
	if s.subtitleSrc == nil {
		return nil, fmt.Errorf("subtitle provider not configured")
	}
	url := s.subtitleSrc.DownloadURL(sub)
	return subtitle.DownloadAndExtract(ctx, url, common.DefaultUserAgent, outputDir)
}

func parseSortKey(s string) float64 {
	var val float64
	_, _ = fmt.Sscanf(s, "%f", &val)
	return val
}

// cleanupTempInput removes local temp files/directories used as ffmpeg input.
func cleanupTempInput(streamURL string) {
	if !strings.HasPrefix(streamURL, "file://") {
		return
	}

	path := streamURL[len("file://"):]
	// If the temp file is inside a bettermelon-m3u8 directory, remove the whole directory.
	dir := filepath.Dir(path)
	if strings.Contains(filepath.Base(dir), "bettermelon-m3u8") {
		_ = os.RemoveAll(dir)
		return
	}
	_ = os.Remove(path)
}
