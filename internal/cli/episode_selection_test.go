package cli

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"

	"github.com/bibimoni/orphion/internal/provider"
)

func TestEpisodeOptionUsesSourceMetadata(t *testing.T) {
	got := episodeOption(provider.Episode{
		Number: "1",
		Title:  "逃げるは恥だが役に立つ 第一話/NIGERUHA.HAJIDAGA.YAKUNITATSU.Ep01.mp4",
		Size:   "304.9 MiB",
	})

	for _, want := range []string{"Ep 1", "304.9 MiB", "第一話"} {
		if !strings.Contains(got, want) {
			t.Fatalf("episode option = %q, want %q", got, want)
		}
	}
}

func TestEpisodeOptionFitsTerminalWidth(t *testing.T) {
	oldWidth := pterm.GetTerminalWidth()
	oldHeight := pterm.GetTerminalHeight()
	pterm.SetForcedTerminalSize(72, 20)
	t.Cleanup(func() { pterm.SetForcedTerminalSize(oldWidth, oldHeight) })

	got := episodeOption(provider.Episode{
		Number: "1",
		Title:  "逃げるは恥だが役に立つ 第一話/NIGERUHA.HAJIDAGA.YAKUNITATSU.Ep01.Chi_Jap.HDTVrip.852X480-ZhuixinFan.mp4",
		Size:   "304.9 MiB",
	})

	// Leave room for pterm's selector and checkbox prefix.
	if width := runewidth.StringWidth(got); width > 64 {
		t.Fatalf("episode option width = %d, want <= 64: %q", width, got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("episode option = %q, want truncation marker", got)
	}
}

func TestEpisodeOptionFallsBackToEpisodeNumber(t *testing.T) {
	got := episodeOption(provider.Episode{Number: "12"})
	if got != "Ep 12" {
		t.Fatalf("episode option = %q, want Ep 12", got)
	}
}

func TestSelectInteractiveEpisodesUsesEpisodeLanguageWithoutExtraColon(t *testing.T) {
	originalSelect := interactiveMultiSelect
	t.Cleanup(func() { interactiveMultiSelect = originalSelect })
	interactiveMultiSelect = func(options []string, defaultText string) ([]string, error) {
		if defaultText != "Select episodes (Enter toggles, Tab confirms)" {
			t.Fatalf("default text = %q", defaultText)
		}
		return []string{options[1]}, nil
	}

	_, err := selectInteractiveEpisodes([]provider.Episode{{ID: "ep-1", Number: "1"}})
	if err != nil {
		t.Fatal(err)
	}
}
