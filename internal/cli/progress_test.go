package cli

import (
	"strings"
	"testing"

	"github.com/distiled/orphion/internal/ffmpeg"
)

func TestFormatProgressLineShowsTorrentMetadata(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Speed:      "metadata",
		TotalBytes: 9_777_767_936,
	})

	if !strings.Contains(got, "Episode 1") {
		t.Fatalf("progress line = %q, want episode", got)
	}
	if !strings.Contains(got, "metadata") {
		t.Fatalf("progress line = %q, want metadata state", got)
	}
	if !strings.Contains(got, "9.1 GiB") {
		t.Fatalf("progress line = %q, want total size", got)
	}
}

func TestFormatProgressLineShowsTorrentDownloadedAndTotal(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Bytes:      1024,
		TotalBytes: 9_777_767_936,
		Speed:      "512 B/s",
	})

	if !strings.Contains(got, "512 B/s") {
		t.Fatalf("progress line = %q, want speed", got)
	}
	if !strings.Contains(got, "1.0 KiB / 9.1 GiB") {
		t.Fatalf("progress line = %q, want downloaded and total", got)
	}
}

func TestFormatProgressLineShowsWaitingForPeers(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		TotalBytes: 304_900_000,
		Speed:      "0 B/s",
		Peers:      0,
		Seeders:    0,
	})

	if !strings.Contains(got, "waiting for peers") {
		t.Fatalf("progress line = %q, want waiting for peers", got)
	}
	if !strings.Contains(got, "0 peers") {
		t.Fatalf("progress line = %q, want peer count", got)
	}
}
