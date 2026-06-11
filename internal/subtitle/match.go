package subtitle

import (
	"sort"
	"strings"
	"unicode"

	"github.com/distiled/orphion/internal/common"
)

// BestMatch finds the Result whose title best matches the query.
// It returns the index and the result. Returns (-1, nil) if no results
// or no match exceeds the minimum threshold.
func BestMatch(query string, results []Result) (int, *Result) {
	if len(results) == 0 {
		return -1, nil
	}

	normQuery := normalizeTitle(query)
	queryTokens := tokenize(normQuery)

	bestIdx := -1
	bestScore := 0.0

	for i, r := range results {
		normResult := normalizeTitle(r.Title)
		score := titleSimilarity(normQuery, normResult, queryTokens)
		if score < common.MatchMinScore {
			continue
		}

		// Bonus for matching type (tv shows are more likely what anime users want).
		if r.Type == "tv" && score > 0 {
			score += common.MatchTVBonus
		}

		// Bonus for having more subtitles available.
		if r.SubCount > 0 && score > 0 {
			score += common.MatchSubCountBonus
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return -1, nil
	}

	return bestIdx, &results[bestIdx]
}

// RankResults ranks subtitle Results against a query string.
// It returns the results sorted by descending similarity to the query,
// filtered to at most maxResults entries with similarity >= minScore.
// This is the Result-based counterpart of FolderMatch and should be used
// when the search provider returns a large unfiltered list (e.g. kitsunekko).
func RankResults(query string, results []Result, maxResults int, minScore float64) []Result {
	if len(results) == 0 {
		return nil
	}
	if maxResults <= 0 {
		maxResults = common.RankDefaultMax
	}

	normQuery := normalizeTitle(query)
	queryTokens := tokenize(normQuery)

	type scored struct {
		idx   int
		score float64
	}

	var ranked []scored
	for i, r := range results {
		normR := normalizeTitle(r.Title)
		score := titleSimilarity(normQuery, normR, queryTokens)
		if score < minScore {
			continue
		}

		// Bonus for matching type (tv shows are more likely what anime users want).
		if r.Type == "tv" && score > 0 {
			score += common.MatchTVBonus
		}
		// Bonus for having more subtitles available.
		if r.SubCount > 0 && score > 0 {
			score += common.MatchSubCountBonus
		}

		ranked = append(ranked, scored{idx: i, score: score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	n := len(ranked)
	if n > maxResults {
		n = maxResults
	}

	out := make([]Result, n)
	for i, r := range ranked[:n] {
		out[i] = results[r.idx]
	}
	return out
}

// FolderMatch ranks folder names against a query string.
// It returns the folder names sorted by descending similarity to the query.
// Names with very low similarity are excluded (below FolderMatchMinScore).
func FolderMatch(query string, folders []string) []string {
	if len(folders) == 0 {
		return nil
	}

	normQuery := normalizeTitle(query)
	queryTokens := tokenize(normQuery)

	type scored struct {
		name  string
		score float64
	}

	var ranked []scored
	for _, f := range folders {
		normF := normalizeTitle(f)
		score := titleSimilarity(normQuery, normF, queryTokens)
		if score > common.FolderMatchMinScore {
			ranked = append(ranked, scored{name: f, score: score})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	out := make([]string, len(ranked))
	for i, r := range ranked {
		out[i] = r.name
	}
	return out
}

// titleSimilarity computes a similarity score between two normalized titles.
// It combines token overlap with whole-string similarity.
func titleSimilarity(normQuery, normResult string, queryTokens []string) float64 {
	// Exact match gets highest score.
	if normQuery == normResult {
		return 1.0
	}

	// Treat spacing-only differences as near-exact matches, such as
	// "SteinsGate" and "Steins Gate".
	if strings.ReplaceAll(normQuery, " ", "") == strings.ReplaceAll(normResult, " ", "") {
		return 0.98
	}

	// Check if one is a substring of the other.
	if strings.Contains(normResult, normQuery) || strings.Contains(normQuery, normResult) {
		longer := float64(max(len(normQuery), len(normResult)))
		shorter := float64(min(len(normQuery), len(normResult)))
		return shorter / longer
	}

	// Token-based overlap score.
	queryTokens = meaningfulTitleTokens(queryTokens)
	resultTokens := meaningfulTitleTokens(tokenize(normResult))
	tokenScore := tokenOverlap(queryTokens, resultTokens)

	// Character-level n-gram similarity for partial matches.
	charScore := bigramSimilarity(normQuery, normResult)

	// For multi-word titles, one shared word is too weak to establish a
	// semantic match. This rejects results such as "Hero Bank" for
	// "Sentenced to Be a Hero" while preserving typos across two title words.
	if len(queryTokens) >= 2 && tokenMatchCount(queryTokens, resultTokens) < 2 {
		return common.CharWeight * charScore
	}

	// Weight token overlap more heavily (it captures semantic similarity better).
	return common.TokenWeight*tokenScore + common.CharWeight*charScore
}

var titleStopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "in": true,
	"is": true, "no": true, "of": true, "on": true, "or": true,
	"the": true, "to": true, "with": true,
}

func meaningfulTitleTokens(tokens []string) []string {
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !titleStopWords[token] {
			filtered = append(filtered, token)
		}
	}
	if len(filtered) == 0 {
		return tokens
	}
	return filtered
}

func tokenMatchCount(query, result []string) int {
	resultSet := make(map[string]bool, len(result))
	for _, token := range result {
		resultSet[token] = true
	}

	count := 0
	for _, queryToken := range query {
		if resultSet[queryToken] {
			count++
			continue
		}
		for _, resultToken := range result {
			if fuzzyTokenMatch(queryToken, resultToken) {
				count++
				break
			}
		}
	}
	return count
}

// tokenOverlap computes the Jaccard-like overlap between two token sets,
// but rewards directionality (query tokens found in result).
// Tokens that differ by a single edit (insertion/deletion/substitution)
// count as partial matches, so "stein" partially matches "steins".
func tokenOverlap(query, result []string) float64 {
	if len(query) == 0 {
		return 0
	}

	resultSet := make(map[string]bool, len(result))
	for _, t := range result {
		resultSet[t] = true
	}

	matched := 0.0
	for _, t := range query {
		if resultSet[t] {
			matched += 1.0
		} else {
			// Fuzzy: check if any result token is within edit distance 1.
			// This catches near-matches like "stein" ↔ "steins".
			for _, r := range result {
				if fuzzyTokenMatch(t, r) {
					matched += common.FuzzyTokenCredit // partial credit for fuzzy match
					break
				}
			}
		}
	}

	// Use query-anchored ratio: what fraction of query tokens appear in result.
	// This penalizes results that are supersets but rewards exact token matches.
	recall := matched / float64(len(query))

	// Also compute precision: what fraction of result tokens appear in query.
	querySet := make(map[string]bool, len(query))
	for _, t := range query {
		querySet[t] = true
	}
	precMatched := 0.0
	for _, t := range result {
		if querySet[t] {
			precMatched += 1.0
		} else {
			// Fuzzy precision match.
			for _, q := range query {
				if fuzzyTokenMatch(q, t) {
					precMatched += common.FuzzyTokenCredit
					break
				}
			}
		}
	}
	precision := precMatched / float64(max(len(result), 1))

	// F1-like harmonic mean.
	if recall+precision == 0 {
		return 0
	}
	return 2 * recall * precision / (recall + precision)
}

func fuzzyTokenMatch(a, b string) bool {
	// Short tokens are mostly articles, particles, and episode numbers.
	// Treating one edit as fuzzy equivalence makes unrelated pairs such as
	// "to"/"no" and "a"/"3" look like meaningful title matches.
	if len(a) < 4 || len(b) < 4 {
		return false
	}
	return editDistanceWithin(a, b, common.FuzzyEditDistance)
}

// editDistanceWithin returns true if the edit distance between a and b
// is at most maxDist. Uses an early-termination variant of the
// Levenshtein algorithm that stops once the distance exceeds maxDist.
func editDistanceWithin(a, b string, maxDist int) bool {
	la, lb := len(a), len(b)
	// Quick reject: length difference alone exceeds maxDist.
	if abs(la-lb) > maxDist {
		return false
	}
	// Swap so a is the shorter string.
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}
	// Single-row DP.
	prev := make([]int, la+1)
	for i := range prev {
		prev[i] = i
	}
	for j := 1; j <= lb; j++ {
		curr := make([]int, la+1)
		curr[0] = j
		minVal := curr[0]
		for i := 1; i <= la; i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,      // deletion
				curr[i-1]+1,    // insertion
				prev[i-1]+cost, // substitution
			)
			if curr[i] < minVal {
				minVal = curr[i]
			}
		}
		// Early termination: if the minimum value in this row exceeds maxDist,
		// the final distance will too.
		if minVal > maxDist {
			return false
		}
		prev = curr
	}
	return prev[la] <= maxDist
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// bigramSimilarity computes dice coefficient on character bigrams.
func bigramSimilarity(a, b string) float64 {
	if len(a) < 2 || len(b) < 2 {
		return 0
	}

	aGrams := bigrams(a)
	bGrams := make(map[string]bool, len(b)-1)
	for i := 0; i < len(b)-1; i++ {
		bGrams[b[i:i+2]] = true
	}

	overlap := 0
	for g := range aGrams {
		if bGrams[g] {
			overlap++
		}
	}

	total := len(aGrams) + len(bGrams)
	if total == 0 {
		return 0
	}
	return 2.0 * float64(overlap) / float64(total)
}

// bigrams returns the set of character bigrams in s.
func bigrams(s string) map[string]bool {
	result := make(map[string]bool, len(s)-1)
	for i := 0; i < len(s)-1; i++ {
		result[s[i:i+2]] = true
	}
	return result
}

// normalizeTitle lowercases and strips all non-alphanumeric characters
// except spaces, then collapses whitespace.
func normalizeTitle(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	prevSpace := false
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		} else if !prevSpace {
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// tokenize splits a normalized title into unique words.
func tokenize(norm string) []string {
	parts := strings.Fields(norm)
	// Deduplicate while preserving order.
	seen := make(map[string]bool, len(parts))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}
