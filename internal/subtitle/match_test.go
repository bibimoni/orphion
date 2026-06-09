package subtitle

import "testing"

func TestBestMatch(t *testing.T) {
	results := []Result{
		{ID: "sd1", Title: "Steins;Gate", Type: "tv", SubCount: 50},
		{ID: "sd2", Title: "Steins;Gate 0", Type: "tv", SubCount: 30},
		{ID: "sd3", Title: "Steins;Gate: Egoistic Poriomania", Type: "tv", SubCount: 5},
		{ID: "sd4", Title: "Gate", Type: "movie", SubCount: 10},
	}

	tests := []struct {
		query    string
		wantID   string
		minScore float64
	}{
		{"Steins;Gate", "sd1", 0.9},
		{"steins gate", "sd1", 0.9},
		{"Steins Gate 0", "sd2", 0.8},
		{"SteinsGate", "sd1", 0.7},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			idx, result := BestMatch(tt.query, results)
			if idx < 0 {
				t.Fatalf("BestMatch(%q) = no match, want %s", tt.query, tt.wantID)
			}
			if result.ID != tt.wantID {
				t.Errorf("BestMatch(%q) = %s (%s), want %s", tt.query, result.ID, result.Title, tt.wantID)
			}
		})
	}
}

func TestBestMatchEmpty(t *testing.T) {
	idx, result := BestMatch("test", nil)
	if idx != -1 || result != nil {
		t.Errorf("BestMatch with nil should return (-1, nil), got (%d, %v)", idx, result)
	}

	idx, result = BestMatch("test", []Result{})
	if idx != -1 || result != nil {
		t.Errorf("BestMatch with empty slice should return (-1, nil), got (%d, %v)", idx, result)
	}
}

func TestBestMatchNoGoodMatch(t *testing.T) {
	results := []Result{
		{ID: "sd1", Title: "Naruto", Type: "tv", SubCount: 100},
		{ID: "sd2", Title: "One Piece", Type: "tv", SubCount: 200},
	}

	idx, result := BestMatch("Breaking Bad", results)
	if idx != -1 || result != nil {
		t.Errorf("BestMatch with unrelated query should return (-1, nil), got (%d, %v)", idx, result)
	}
}

func TestBestMatchTVBonus(t *testing.T) {
	results := []Result{
		{ID: "sd1", Title: "Death Note", Type: "movie", SubCount: 20},
		{ID: "sd2", Title: "Death Note", Type: "tv", SubCount: 50},
	}

	idx, result := BestMatch("Death Note", results)
	if idx < 0 {
		t.Fatal("expected a match")
	}
	if result.Type != "tv" {
		t.Errorf("expected tv type to be preferred, got %s", result.Type)
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Steins;Gate", "steins gate"},
		{"Re:Zero", "re zero"},
		{"Kono Subarashii Sekai ni Shukufuku wo!", "kono subarashii sekai ni shukufuku wo"},
		{"  extra   spaces  ", "extra spaces"},
		{"Bleach: Thousand-Year Blood War", "bleach thousand year blood war"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenOverlap(t *testing.T) {
	tests := []struct {
		query   string
		result  string
		wantMin float64
	}{
		{"steins gate", "steins gate", 0.99},
		{"steins gate", "steins gate 0", 0.5},
		{"steins gate", "naruto", 0.0},
	}

	for _, tt := range tests {
		score := tokenOverlap(tokenize(tt.query), tokenize(tt.result))
		if score < tt.wantMin-0.01 {
			t.Errorf("tokenOverlap(%q, %q) = %.3f, want >= %.3f", tt.query, tt.result, score, tt.wantMin)
		}
	}
}

func TestBigramSimilarity(t *testing.T) {
	// Identical strings should score 1.0.
	if score := bigramSimilarity("abc", "abc"); score != 1.0 {
		t.Errorf("bigramSimilarity identical = %.3f, want 1.0", score)
	}
	// Completely different strings should score 0.
	if score := bigramSimilarity("ab", "cd"); score != 0.0 {
		t.Errorf("bigramSimilarity different = %.3f, want 0.0", score)
	}
	// Partial overlap.
	score := bigramSimilarity("steinsgate", "steinsgate0")
	if score <= 0.5 {
		t.Errorf("bigramSimilarity partial = %.3f, want > 0.5", score)
	}
}

func TestBestMatchRealWorld(t *testing.T) {
	results := []Result{
		{ID: "sd1300262", Title: "Steins;Gate", Type: "tv", SubCount: 100},
		{ID: "sd1304005", Title: "Steins;Gate 0", Type: "tv", SubCount: 50},
		{ID: "sd1654461", Title: "Steins;Gate: Egoistic Poriomania", Type: "tv", SubCount: 5},
		{ID: "sd26643", Title: "Steins;Gate: Oukoubakko no Poriomania", Type: "tv", SubCount: 3},
	}

	// "Steins;Gate" should match the first result, not the sequel.
	idx, result := BestMatch("Steins;Gate", results)
	if idx < 0 {
		t.Fatal("expected a match")
	}
	if result.ID != "sd1300262" {
		t.Errorf("BestMatch(Steins;Gate) = %s (%s), want sd1300262 (Steins;Gate)", result.ID, result.Title)
	}

	// "Steins;Gate 0" should match the sequel.
	idx, result = BestMatch("Steins;Gate 0", results)
	if idx < 0 {
		t.Fatal("expected a match for Steins;Gate 0")
	}
	if result.ID != "sd1304005" {
		t.Errorf("BestMatch(Steins;Gate 0) = %s (%s), want sd1304005 (Steins;Gate 0)", result.ID, result.Title)
	}
}

func TestFolderMatch(t *testing.T) {
	folders := []string{
		"Steins Gate",
		"Steins Gate 0",
		"Naruto",
		"Attack on Titan",
		"Death Note",
	}

	// Query should rank matching folders first.
	ranked := FolderMatch("Steins Gate", folders)
	if len(ranked) == 0 {
		t.Fatal("FolderMatch returned no results")
	}
	// First result should be the best match.
	if ranked[0] != "Steins Gate" {
		t.Errorf("FolderMatch first = %q, want %q", ranked[0], "Steins Gate")
	}
	// Non-matching folders should be excluded.
	for _, f := range ranked {
		if f == "Naruto" {
			t.Error("FolderMatch should not include unrelated folders")
		}
	}
}

func TestFolderMatchEmpty(t *testing.T) {
	if ranked := FolderMatch("test", nil); ranked != nil {
		t.Errorf("FolderMatch(nil) = %v, want nil", ranked)
	}
	if ranked := FolderMatch("test", []string{}); ranked != nil {
		t.Errorf("FolderMatch(empty) = %v, want nil", ranked)
	}
}

func TestFolderMatchNoMatch(t *testing.T) {
	folders := []string{"Naruto", "Bleach"}
	ranked := FolderMatch("Breaking Bad", folders)
	if len(ranked) != 0 {
		t.Errorf("FolderMatch with unrelated query = %v, want empty", ranked)
	}
}

func TestBestMatchTypo(t *testing.T) {
	// Simulate the real Kitsunekko scenario: the directory for the original
	// Steins;Gate is misspelled as "Stein;Gate" (missing 's').
	results := []Result{
		{ID: "typo", Title: "Stein;Gate", Type: "tv", SubCount: 10},
		{ID: "sequel", Title: "Steins;Gate 0", Type: "tv", SubCount: 30},
		{ID: "unrelated", Title: "Naruto", Type: "tv", SubCount: 100},
	}

	// "Steins;Gate" should match the typo'd original over the sequel.
	idx, result := BestMatch("Steins;Gate", results)
	if idx < 0 {
		t.Fatal("BestMatch should find a match for the typo'd title")
	}
	if result.ID != "typo" {
		t.Errorf("BestMatch(Steins;Gate) = %s (%s), want typo (Stein;Gate)", result.ID, result.Title)
	}
}

func TestEditDistanceWithin(t *testing.T) {
	tests := []struct {
		a, b     string
		maxDist  int
		expected bool
	}{
		{"stein", "steins", 1, true}, // single insertion
		{"steins", "stein", 1, true}, // single deletion
		{"gate", "gate", 1, true},    // identical
		{"abc", "xyz", 1, false},     // completely different
		{"a", "abc", 1, false},       // two insertions
		{"a", "abc", 2, true},        // two insertions within limit
	}

	for _, tt := range tests {
		got := editDistanceWithin(tt.a, tt.b, tt.maxDist)
		if got != tt.expected {
			t.Errorf("editDistanceWithin(%q, %q, %d) = %v, want %v", tt.a, tt.b, tt.maxDist, got, tt.expected)
		}
	}
}

func TestRankResults(t *testing.T) {
	results := []Result{
		{ID: "1", Title: "Steins;Gate", Type: "tv", SubCount: 50},
		{ID: "2", Title: "Steins;Gate 0", Type: "tv", SubCount: 30},
		{ID: "3", Title: "Naruto", Type: "tv", SubCount: 100},
		{ID: "4", Title: "Gate", Type: "movie", SubCount: 10},
		{ID: "5", Title: "Death Note", Type: "tv", SubCount: 80},
	}

	// Query matching some results.
	ranked := RankResults("Steins Gate", results, 10, 0.2)
	if len(ranked) == 0 {
		t.Fatal("RankResults returned no results")
	}
	// First result should be the best match.
	if ranked[0].Title != "Steins;Gate" {
		t.Errorf("RankResults first = %q, want %q", ranked[0].Title, "Steins;Gate")
	}
	// Non-matching results should be excluded.
	for _, r := range ranked {
		if r.Title == "Naruto" || r.Title == "Death Note" {
			t.Errorf("RankResults should not include unrelated: %q", r.Title)
		}
	}

	// Test maxResults limit.
	ranked = RankResults("Steins Gate", results, 1, 0.0)
	if len(ranked) != 1 {
		t.Errorf("RankResults with maxResults=1 returned %d, want 1", len(ranked))
	}

	// Test minScore filtering.
	ranked = RankResults("xyz", results, 10, 0.9)
	if len(ranked) != 0 {
		t.Errorf("RankResults with high minScore = %d results, want 0", len(ranked))
	}

	// Test empty input.
	ranked = RankResults("test", nil, 10, 0.2)
	if ranked != nil {
		t.Errorf("RankResults(nil) = %v, want nil", ranked)
	}
}
