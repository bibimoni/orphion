package episode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Episode represents a single episode from a provider.
type Episode struct {
	ID      string
	Number  string
	SortKey float64
}

// Request holds a parsed episode expression.
type Request struct {
	All  bool
	Seqs []Seq
}

// Seq represents a range or single episode number.
type Seq struct {
	Start string
	End   string
}

// Parser parses episode expressions.
type Parser struct{}

// Parse parses an episode expression string.
func (p *Parser) Parse(expr string) (*Request, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty episode expression")
	}
	if strings.ToLower(expr) == "all" {
		return &Request{All: true}, nil
	}

	var seqs []Seq
	parts := strings.Split(expr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty episode token in %q", expr)
		}
		var s Seq
		dashIdx := strings.Index(part, "-")
		if dashIdx >= 0 {
			s.Start = strings.TrimSpace(part[:dashIdx])
			s.End = strings.TrimSpace(part[dashIdx+1:])
			if s.Start == "" || s.End == "" {
				return nil, fmt.Errorf("invalid range %q in %q", part, expr)
			}
			if !validEpisodeNum(s.Start) || !validEpisodeNum(s.End) {
				return nil, fmt.Errorf("invalid range %q in %q", part, expr)
			}
			if parseSortKey(s.Start) >= parseSortKey(s.End) {
				return nil, fmt.Errorf("range %q must be increasing", part)
			}
		} else {
			s.Start = part
			s.End = part
			if !validEpisodeNum(s.Start) {
				return nil, fmt.Errorf("invalid episode number %q", part)
			}
		}
		seqs = append(seqs, s)
	}
	return &Request{Seqs: seqs}, nil
}

func validEpisodeNum(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == '.' {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Resolve selects episodes matching the request. Episodes are returned
// in the order they appear in the provided list, without duplicates.
func Resolve(req *Request, episodes []Episode) []Episode {
	if req.All {
		return episodes
	}

	// Build a set of matching sort keys from our ranges.
	var out []Episode
	for _, ep := range episodes {
		if matches(req, ep) {
			out = append(out, ep)
		}
	}
	return out
}

func matches(req *Request, ep Episode) bool {
	key := ep.SortKey
	if key == 0 {
		key = parseSortKey(ep.Number)
	}
	for _, seq := range req.Seqs {
		start := parseSortKey(seq.Start)
		end := parseSortKey(seq.End)
		if key >= start && key <= end {
			return true
		}
	}
	return false
}

func parseSortKey(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return -1
	}
	return f
}

// AllNumbers returns a sorted list of all episode numbers referenced by
// this request and the available episodes.
func AllNumbers(req *Request, episodes []Episode) []string {
	if req.All {
		var nums []string
		for _, ep := range episodes {
			nums = append(nums, ep.Number)
		}
		return nums
	}

	seen := make(map[string]bool)
	var nums []string
	for _, seq := range req.Seqs {
		for _, ep := range episodes {
			key := ep.SortKey
			if key == 0 {
				key = parseSortKey(ep.Number)
			}
			start := parseSortKey(seq.Start)
			end := parseSortKey(seq.End)
			if key >= start && key <= end {
				if !seen[ep.Number] {
					seen[ep.Number] = true
					nums = append(nums, ep.Number)
				}
			}
		}
	}
	sort.Strings(nums)
	return nums
}