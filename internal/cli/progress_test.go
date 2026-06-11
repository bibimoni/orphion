package cli

import (
	"strings"
	"testing"

	"github.com/bibimoni/orphion/internal/ffmpeg"
)

func TestFormatProgressLineShowsConnecting(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{})
	if !strings.Contains(got, "Episode 1") {
		t.Fatalf("progress line = %q, want episode", got)
	}
	if !strings.Contains(got, "connecting") {
		t.Fatalf("progress line = %q, want connecting state", got)
	}
}

func TestFormatProgressLineShowsResolvingStream(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{Phase: "resolving"})
	if !strings.Contains(got, "resolving stream") {
		t.Fatalf("progress line = %q, want resolving stream phase", got)
	}
}

func TestFormatProgressLineShowsSpeedAndSize(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Bytes:      1024,
		TotalBytes: 9_777_767_936,
		Speed:      "2.5x",
	})

	if !strings.Contains(got, "2.5x") {
		t.Fatalf("progress line = %q, want speed", got)
	}
	if strings.Contains(got, "2.5x/s") {
		t.Fatalf("progress line = %q, playback speed must not be shown as bytes per second", got)
	}
	if !strings.Contains(got, "1.0 KiB / 9.1 GiB") {
		t.Fatalf("progress line = %q, want downloaded and total", got)
	}
}

func TestFormatProgressLineShowsSegmentProgress(t *testing.T) {
	got := formatProgressLine("3", ffmpeg.Progress{
		Phase:         "segments",
		SegmentsDone:  12,
		SegmentsTotal: 48,
	})

	if !strings.Contains(got, "Episode 3") {
		t.Fatalf("progress line = %q, want episode", got)
	}
	if !strings.Contains(got, "12/48") {
		t.Fatalf("progress line = %q, want segment count", got)
	}
	if !strings.Contains(got, "25%") {
		t.Fatalf("progress line = %q, want 25%%", got)
	}
	if !strings.Contains(got, "segments") {
		t.Fatalf("progress line = %q, want 'segments'", got)
	}
}

func TestFormatProgressLineShowsSegmentProgressComplete(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Phase:         "segments",
		SegmentsDone:  48,
		SegmentsTotal: 48,
	})

	if !strings.Contains(got, "48/48") {
		t.Fatalf("progress line = %q, want 48/48", got)
	}
	if !strings.Contains(got, "100%") {
		t.Fatalf("progress line = %q, want 100%%", got)
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		done   int
		total  int
		width  int
		expect string
	}{
		{0, 10, 10, "[░░░░░░░░░░]"},
		{5, 10, 10, "[█████░░░░░]"},
		{10, 10, 10, "[██████████]"},
		{3, 12, 6, "[█░░░░░]"},
	}
	for _, tt := range tests {
		got := progressBar(tt.done, tt.total, tt.width)
		// Strip ANSI color codes for comparison.
		clean := stripANSI(got)
		if clean != tt.expect {
			t.Errorf("progressBar(%d, %d, %d) = %q, want %q", tt.done, tt.total, tt.width, clean, tt.expect)
		}
	}
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var out strings.Builder
	escaping := false
	for _, r := range s {
		if r == '\x1b' {
			escaping = true
			continue
		}
		if escaping {
			if r == 'm' {
				escaping = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
