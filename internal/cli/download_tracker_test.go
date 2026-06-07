package cli

import (
	"strings"
	"testing"

	"github.com/distiled/orphion/internal/ffmpeg"
)

func TestDownloadTrackerUpdateAndDone(t *testing.T) {
	tr := &downloadTracker{
		states: make(map[string]*episodeState),
	}

	tr.update("1", ffmpeg.Progress{Speed: "5.0x", Bytes: 1024, TotalBytes: 2048})
	tr.update("2", ffmpeg.Progress{})
	tr.markDone("1")

	if len(tr.order) != 2 {
		t.Fatalf("order length = %d, want 2", len(tr.order))
	}
	if tr.order[0] != "1" || tr.order[1] != "2" {
		t.Fatalf("order = %v, want [1 2]", tr.order)
	}
	if !tr.states["1"].done {
		t.Error("episode 1 should be done")
	}
	if tr.states["2"].done {
		t.Error("episode 2 should not be done")
	}
}

func TestDownloadTrackerRenderDoneLine(t *testing.T) {
	tr := &downloadTracker{
		states: make(map[string]*episodeState),
	}
	tr.update("3", ffmpeg.Progress{})
	tr.markDone("3")

	// Render the done line — should contain ✓.
	var lines []string
	for _, key := range tr.order {
		st := tr.states[key]
		if st.done {
			lines = append(lines, formatProgressLine(st.episode, st.progress))
		}
	}
	rendered := lines[0]
	if !strings.Contains(rendered, "Episode 3") {
		t.Fatalf("done line = %q, want episode 3", rendered)
	}
}

func TestDownloadTrackerStableOrder(t *testing.T) {
	tr := &downloadTracker{
		states: make(map[string]*episodeState),
	}

	// Episodes appear in insertion order, even if updates come out of order.
	tr.update("3", ffmpeg.Progress{})
	tr.update("1", ffmpeg.Progress{})
	tr.update("2", ffmpeg.Progress{})
	tr.update("3", ffmpeg.Progress{Speed: "10x"}) // re-update

	if len(tr.order) != 3 {
		t.Fatalf("order length = %d, want 3", len(tr.order))
	}
	if tr.order[0] != "3" || tr.order[1] != "1" || tr.order[2] != "2" {
		t.Fatalf("order = %v, want [3 1 2]", tr.order)
	}
}
