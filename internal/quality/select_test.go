package quality

import "testing"

func TestSelect(t *testing.T) {
	tests := []struct {
		name      string
		preferred string
		streams   []Stream
		wantURL   string
		wantReas  Reason
	}{
		{
			"exact",
			"1080p",
			[]Stream{
				{URL: "u720", Quality: "720p"},
				{URL: "u1080", Quality: "1080p"},
				{URL: "u480", Quality: "480p"},
			},
			"u1080", ReasonExact,
		},
		{
			"lower_fallback",
			"1080p",
			[]Stream{
				{URL: "u720", Quality: "720p"},
				{URL: "u480", Quality: "480p"},
			},
			"u720", ReasonLower,
		},
		{
			"lowest_fallback",
			"1080p",
			[]Stream{
				{URL: "u2160", Quality: "2160p"},
				{URL: "u1440", Quality: "1440p"},
			},
			"u1440", ReasonLowest,
		},
		{
			"provider_fallback",
			"1080p",
			[]Stream{
				{URL: "u-no-label", Quality: ""},
			},
			"u-no-label", ReasonProvider,
		},
		{
			"1920x1080_format",
			"1080p",
			[]Stream{
				{URL: "u1080", Quality: "1920x1080"},
				{URL: "u720", Quality: "1280x720"},
			},
			"u1080", ReasonExact,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Select(tt.preferred, tt.streams)
			if r.Stream.URL != tt.wantURL {
				t.Fatalf("URL = %q, want %q", r.Stream.URL, tt.wantURL)
			}
			if r.Reason != tt.wantReas {
				t.Fatalf("Reason = %v, want %v", r.Reason, tt.wantReas)
			}
		})
	}
}

func TestParseQuality(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1080", 1080},
		{"1080p", 1080},
		{"720p", 720},
		{"480", 480},
		{"1920x1080", 1080},
		{"1280x720", 720},
		{"", -1},
		{"abc", -1},
	}
	for _, tt := range tests {
		got := parseQuality(tt.input)
		if got != tt.want {
			t.Errorf("parseQuality(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
