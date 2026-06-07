package quality

import (
	"strconv"
	"strings"
)

// Stream represents a downloadable quality variant.
type Stream struct {
	URL     string
	Quality string
}

// Reason indicates why a stream was selected.
type Reason int

const (
	ReasonExact Reason = iota
	ReasonLower
	ReasonLowest
	ReasonProvider
)

// Result holds the selected stream and the reason it was chosen.
type Result struct {
	Stream Stream
	Reason Reason
}

// Select selects the best stream matching the preferred quality.
func Select(preferred string, streams []Stream) Result {
	if len(streams) == 0 {
		return Result{}
	}
	target := parseQuality(preferred)

	var (
		best   Stream
		bestQ  float64 = -1
		reason Reason
	)

	// Find exact match first.
	for _, s := range streams {
		q := parseQuality(s.Quality)
		if q == target {
			return Result{Stream: s, Reason: ReasonExact}
		}
		if q > 0 && q < target {
			if q > bestQ {
				bestQ = q
				best = s
				reason = ReasonLower
			}
		}
	}

	if reason == ReasonLower {
		return Result{Stream: best, Reason: reason}
	}

	// No valid lower quality. Pick lowest available.
	var lowest Stream
	lowestQ := float64(-1)
	for _, s := range streams {
		q := parseQuality(s.Quality)
		if q > 0 && (lowestQ < 0 || q < lowestQ) {
			lowestQ = q
			lowest = s
		}
	}
	if lowestQ > 0 {
		return Result{Stream: lowest, Reason: ReasonLowest}
	}

	// Fall back to the provider's first stream.
	return Result{Stream: streams[0], Reason: ReasonProvider}
}

// parseQuality extracts a numeric height from a quality string.
// Accepts formats: "1080", "1080p", "1920x1080".
func parseQuality(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	s = strings.TrimSuffix(s, "p")
	if strings.Contains(s, "x") {
		parts := strings.Split(s, "x")
		if len(parts) == 2 {
			h, err := strconv.ParseFloat(parts[1], 64)
			if err == nil && h > 0 {
				return h
			}
		}
		return -1
	}
	h, err := strconv.ParseFloat(s, 64)
	if err != nil || h <= 0 {
		return -1
	}
	return h
}
