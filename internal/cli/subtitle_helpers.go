package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/subtitle"
)

var subtitleEpisodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bS\d{1,2}E(\d{1,3})\b`),
	regexp.MustCompile(`(?i)\b(?:E|EP|EPISODE)[ ._-]?(\d{1,3})\b`),
}

// resultLabel builds a display label for a subtitle search result.
// It includes the source provider and type when available.
func resultLabel(r subtitle.Result) string {
	label := r.Title
	if r.Year > 0 {
		label = fmt.Sprintf("%s (%d)", label, r.Year)
	}
	if r.Type != "" && r.Type != "tv" {
		label = fmt.Sprintf("%s [%s]", label, r.Type)
	}
	if r.Source != "" {
		label = fmt.Sprintf("%s (%s)", label, r.Source)
	}
	return label
}

// matchResultLabel checks if a subtitle result matches a display label
// produced by resultLabel.
func matchResultLabel(r subtitle.Result, label string) bool {
	return resultLabel(r) == label
}

// subtitleFileLabel builds a display label for a subtitle file entry.
func subtitleFileLabel(sub subtitle.Subtitle) string {
	parts := []string{sub.Title}
	if sub.Quality != "" {
		parts = append(parts, sub.Quality)
	}
	if sub.Downloads > 0 {
		parts = append(parts, fmt.Sprintf("%d downloads", sub.Downloads))
	}
	if sub.Author != "" {
		parts = append(parts, sub.Author)
	}
	return strings.Join(parts, " | ")
}

// filterByLang filters subtitles to those matching the preferred language.
// Returns the filtered list (which may be empty) and a boolean indicating
// whether any subtitles matched the language.
func filterByLang(subs []subtitle.Subtitle, lang string) ([]subtitle.Subtitle, bool) {
	var filtered []subtitle.Subtitle
	for _, sub := range subs {
		if strings.EqualFold(sub.Language, lang) {
			filtered = append(filtered, sub)
		}
	}
	return filtered, len(filtered) > 0
}

// selectSubtitleResult searches for subtitles, auto-matches if possible,
// and falls back to manual selection. Returns the chosen result or nil
// if the user cancels or no results are found.
func selectSubtitleResult(ctx context.Context, service *app.Service, query string) (*subtitle.Result, error) {
	results, err := service.SearchSubtitles(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search subtitles: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	// Rank results by similarity and keep the top matches.
	// This is critical for providers like kitsunekko that return ALL
	// directories (thousands) without server-side filtering.
	ranked := subtitle.RankResults(query, results, common.RankDefaultMax, common.RankMinScore)

	// If results come from multiple providers, always show the selection
	// list so the user can choose which provider (and thus which subtitle
	// pool) to use. Auto-match is only safe when there's a single provider
	// because picking one provider means losing access to the other's subtitles.
	sources := uniqueSources(ranked)
	if len(sources) == 1 {
		// Single provider — auto-match is safe.
		matchIdx, matchResult := subtitle.BestMatch(query, ranked)
		if matchIdx >= 0 {
			return matchResult, nil
		}
	}

	if len(ranked) == 0 {
		return nil, nil
	}

	// Show manual selection from the ranked list.
	opts := make([]string, len(ranked)+1)
	opts[0] = backOption
	for i, r := range ranked {
		opts[i+1] = resultLabel(r)
	}
	selected, err := interactiveSelect(opts, "Select title")
	if err != nil {
		return nil, fmt.Errorf("title selection: %w", err)
	}
	if selected == backOption {
		return nil, nil
	}
	for i := range ranked {
		if matchResultLabel(ranked[i], selected) {
			return &ranked[i], nil
		}
	}
	return nil, nil
}

// uniqueSources returns the set of distinct Source values in the results.
func uniqueSources(results []subtitle.Result) map[string]bool {
	m := make(map[string]bool, len(results))
	for _, r := range results {
		if r.Source != "" {
			m[r.Source] = true
		}
	}
	return m
}

func matchSubtitlesToEpisodes(subs []subtitle.Subtitle, episodes []string) ([]subtitle.Subtitle, []string) {
	bestByEpisode := make(map[int]subtitle.Subtitle)
	for _, sub := range subs {
		episodeNumber := sub.Episode
		if episodeNumber <= 0 {
			episodeNumber = subtitleEpisodeFromTitle(sub.Title)
		}
		if episodeNumber <= 0 {
			continue
		}
		current, exists := bestByEpisode[episodeNumber]
		if !exists || sub.Downloads > current.Downloads {
			bestByEpisode[episodeNumber] = sub
		}
	}

	matched := make([]subtitle.Subtitle, 0, len(episodes))
	var missing []string
	for _, episode := range episodes {
		number, err := strconv.Atoi(strings.TrimSpace(episode))
		if err != nil || number <= 0 {
			missing = append(missing, episode)
			continue
		}
		sub, ok := bestByEpisode[number]
		if !ok {
			missing = append(missing, episode)
			continue
		}
		matched = append(matched, sub)
	}
	return matched, missing
}

func subtitleEpisodeFromTitle(title string) int {
	for _, pattern := range subtitleEpisodePatterns {
		match := pattern.FindStringSubmatch(title)
		if len(match) != 2 {
			continue
		}
		number, err := strconv.Atoi(match[1])
		if err == nil && number > 0 {
			return number
		}
	}
	return 0
}
