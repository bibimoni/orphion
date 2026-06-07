package cli

import (
	"strings"
	"testing"

	"github.com/distiled/orphion/internal/ffmpeg"
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

func TestFormatProgressLineShowsSpeedAndSize(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Bytes:      1024,
		TotalBytes: 9_777_767_936,
		Speed:      "2.5x",
	})

	if !strings.Contains(got, "2.5x") {
		t.Fatalf("progress line = %q, want speed", got)
	}
	if !strings.Contains(got, "1.0 KiB / 9.1 GiB") {
		t.Fatalf("progress line = %q, want downloaded and total", got)
	}
}
