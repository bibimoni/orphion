package cli

import (
	"testing"

	"github.com/bibimoni/orphion/internal/ffmpeg"
	"github.com/bibimoni/orphion/internal/provider"
	"github.com/bibimoni/orphion/internal/subtitle"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes  int64
		expect string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1572864, "1.5 MiB"},
		{1073741824, "1.0 GiB"},
		{1610612736, "1.5 GiB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.expect {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.expect)
		}
	}
}

func TestOutputDirFor(t *testing.T) {
	tests := []struct {
		path   string
		expect string
	}{
		{"/home/user/Anime/Title/ep.mkv", "/home/user/Anime/Title"},
		{"ep.mkv", "ep.mkv"},
		{"/tmp/test", "/tmp"},
	}
	for _, tt := range tests {
		got := outputDirFor(tt.path)
		if got != tt.expect {
			t.Errorf("outputDirFor(%q) = %q, want %q", tt.path, got, tt.expect)
		}
	}
}

func TestFormatProgressLinePhaseResolving(t *testing.T) {
	got := formatProgressLine("5", ffmpeg.Progress{Phase: "resolving"})
	if !contains(got, "Episode 5") {
		t.Errorf("progress line = %q, want episode number", got)
	}
	if !contains(got, "resolving stream") {
		t.Errorf("progress line = %q, want resolving stream", got)
	}
}

func TestFormatProgressLinePhaseConnecting(t *testing.T) {
	got := formatProgressLine("2", ffmpeg.Progress{})
	if !contains(got, "connecting") {
		t.Errorf("progress line = %q, want connecting", got)
	}
}

func TestFormatProgressLineWithSpeedOnly(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{Speed: "5.0x"})
	if !contains(got, "5.0x") {
		t.Errorf("progress line = %q, want speed", got)
	}
	if contains(got, "0 B") {
		t.Errorf("progress line = %q, should not show 0 B", got)
	}
}

func TestFormatProgressLineWithBytesOnly(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{Bytes: 2048})
	if !contains(got, "2.0 KiB") {
		t.Errorf("progress line = %q, want byte count", got)
	}
}

func TestFormatProgressLineWithSpeedAndTotal(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Bytes:      1048576,
		TotalBytes: 10485760,
		Speed:      "3.0x",
	})
	if !contains(got, "3.0x") {
		t.Errorf("progress line = %q, want speed", got)
	}
	if !contains(got, "/") {
		t.Errorf("progress line = %q, want size/total format", got)
	}
}

func TestCleanUserInput(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"  hello world  ", "hello world"},
		{"alt+something", "something"},
		{"ctrl+x", "x"},
		{"ESC escape", "escape"},
		{"normal input", "normal input"},
		{"", ""},
	}
	for _, tt := range tests {
		got := cleanUserInput(tt.input)
		if got != tt.expect {
			t.Errorf("cleanUserInput(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestResultLabel(t *testing.T) {
	tests := []struct {
		result subtitle.Result
		expect string
	}{
		{subtitle.Result{Title: "Naruto", Year: 2002, Type: "tv", Source: "subdl"}, "Naruto (2002) (subdl)"},
		{subtitle.Result{Title: "Spirited Away", Year: 2001, Type: "movie", Source: "jimaku"}, "Spirited Away (2001) [movie] (jimaku)"},
		{subtitle.Result{Title: "Test", Type: "tv", Source: "kitsunekko"}, "Test (kitsunekko)"},
		{subtitle.Result{Title: "Minimal"}, "Minimal"},
	}
	for _, tt := range tests {
		got := resultLabel(tt.result)
		if got != tt.expect {
			t.Errorf("resultLabel(%+v) = %q, want %q", tt.result, got, tt.expect)
		}
	}
}

func TestSubtitleFileLabel(t *testing.T) {
	tests := []struct {
		sub    subtitle.Subtitle
		expect string
	}{
		{subtitle.Subtitle{Title: "S01E01", Quality: "bluray", Downloads: 100, Author: "user1"}, "S01E01 | bluray | 100 downloads | user1"},
		{subtitle.Subtitle{Title: "S01E02", Quality: "webdl"}, "S01E02 | webdl"},
		{subtitle.Subtitle{Title: "S01E03"}, "S01E03"},
	}
	for _, tt := range tests {
		got := subtitleFileLabel(tt.sub)
		if got != tt.expect {
			t.Errorf("subtitleFileLabel(%+v) = %q, want %q", tt.sub, got, tt.expect)
		}
	}
}

func TestFilterByLang(t *testing.T) {
	subs := []subtitle.Subtitle{
		{Language: "english", Title: "en sub"},
		{Language: "japanese", Title: "ja sub"},
		{Language: "english", Title: "en sub 2"},
	}

	filtered, hasMatch := filterByLang(subs, "english")
	if !hasMatch {
		t.Error("filterByLang should find english match")
	}
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}

	// Case-insensitive matching.
	filteredCI, hasMatchCI := filterByLang(subs, "ENGLISH")
	if !hasMatchCI {
		t.Error("filterByLang should be case-insensitive")
	}
	if len(filteredCI) != 2 {
		t.Fatalf("len(filteredCI) = %d, want 2", len(filteredCI))
	}

	// No match.
	_, noMatch := filterByLang(subs, "french")
	if noMatch {
		t.Error("filterByLang should not find french match")
	}
}

func TestUniqueSources(t *testing.T) {
	results := []subtitle.Result{
		{Source: "subdl"},
		{Source: "jimaku"},
		{Source: "subdl"},
		{Source: ""},
	}
	sources := uniqueSources(results)
	if len(sources) != 2 {
		t.Fatalf("len(uniqueSources) = %d, want 2", len(sources))
	}
	if !sources["subdl"] || !sources["jimaku"] {
		t.Errorf("uniqueSources = %v, want subdl and jimaku", sources)
	}
}

func TestEpisodeOption(t *testing.T) {
	ep := provider.Episode{ID: "ep-1", Number: "1", Title: "Pilot", Size: "500MB"}
	got := episodeOption(ep)
	if !contains(got, "Ep 1") {
		t.Errorf("episodeOption() = %q, want episode number", got)
	}
	if !contains(got, "Pilot") {
		t.Errorf("episodeOption() = %q, want title", got)
	}
	if !contains(got, "500MB") {
		t.Errorf("episodeOption() = %q, want size", got)
	}
}

func TestEpisodeOptionMinimal(t *testing.T) {
	ep := provider.Episode{ID: "ep-2", Number: "12"}
	got := episodeOption(ep)
	if !contains(got, "Ep 12") {
		t.Errorf("episodeOption() = %q, want episode number only", got)
	}
}

func TestMatchResultLabel(t *testing.T) {
	r := subtitle.Result{Title: "Naruto", Year: 2002, Type: "tv", Source: "subdl"}
	label := resultLabel(r)
	if !matchResultLabel(r, label) {
		t.Errorf("matchResultLabel(%+v, %q) = false, want true", r, label)
	}
	if matchResultLabel(r, "wrong label") {
		t.Error("matchResultLabel should return false for wrong label")
	}
}

func TestSubtitleEpisodeFromTitle(t *testing.T) {
	tests := []struct {
		title  string
		expect int
	}{
		{"S01E05", 5},
		{"S02E12", 12},
		{"E03", 3},
		{"EP7", 7},
		{"Episode 10", 10},
		{"S01E01 BD", 1},
		{"No Episode Here", 0},
		{"e5", 5},
	}
	for _, tt := range tests {
		got := subtitleEpisodeFromTitle(tt.title)
		if got != tt.expect {
			t.Errorf("subtitleEpisodeFromTitle(%q) = %d, want %d", tt.title, got, tt.expect)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
