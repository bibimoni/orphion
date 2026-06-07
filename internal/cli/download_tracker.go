package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pterm/pterm"

	"github.com/distiled/orphion/internal/ffmpeg"
)

// episodeState tracks the display state of a single episode download.
type episodeState struct {
	episode  string
	progress ffmpeg.Progress
	done     bool
}

// downloadTracker renders a stable multi-line live display for concurrent
// downloads. Each episode occupies a fixed line position (insertion order).
// Completed episodes show ✓ and stay in place.
type downloadTracker struct {
	mu      sync.Mutex
	area    *pterm.AreaPrinter
	order   []string
	states  map[string]*episodeState
	heading string
}

// newDownloadTracker creates and starts a live area display.
func newDownloadTracker() *downloadTracker {
	area, _ := pterm.DefaultArea.WithRemoveWhenDone().Start()
	return &downloadTracker{
		area:   area,
		states: make(map[string]*episodeState),
	}
}

// update records a progress event for an episode and re-renders the display.
func (t *downloadTracker) update(episode string, progress ffmpeg.Progress) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.states[episode]; !exists {
		t.order = append(t.order, episode)
	}
	t.states[episode] = &episodeState{
		episode:  episode,
		progress: progress,
		done:     false,
	}
	t.render()
}

// markDone marks an episode as completed and re-renders.
func (t *downloadTracker) markDone(episode string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if st, ok := t.states[episode]; ok {
		st.done = true
	}
	t.render()
}

// stop ends the live display.
func (t *downloadTracker) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.area != nil {
		_ = t.area.Stop()
	}
}

// render redraws all episode lines in insertion order.
func (t *downloadTracker) render() {
	var lines []string
	if t.heading != "" {
		lines = append(lines, t.heading)
	}
	for _, key := range t.order {
		st := t.states[key]
		if st.done {
			lines = append(lines, fmt.Sprintf("  %s Episode %s", pterm.Green("✓"), st.episode))
		} else {
			lines = append(lines, formatProgressLine(st.episode, st.progress))
		}
	}
	if t.area != nil {
		t.area.Update(strings.Join(lines, "\n"))
	}
}
