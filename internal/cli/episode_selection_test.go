package cli

import (
	"strings"
	"testing"

	"github.com/distiled/orphion/internal/provider"
)

func TestEpisodeOptionUsesSourceMetadata(t *testing.T) {
	got := episodeOption(provider.Episode{
		Number:  "1",
		Title:   "逃げるは恥だが役に立つ 第一話/NIGERUHA.HAJIDAGA.YAKUNITATSU.Ep01.mp4",
		Size:    "304.9 MiB",
		Seeders: 1,
	})

	for _, want := range []string{"Ep 1", "304.9 MiB", "seeders=1", "第一話"} {
		if !strings.Contains(got, want) {
			t.Fatalf("episode option = %q, want %q", got, want)
		}
	}
}

func TestEpisodeOptionFallsBackToEpisodeNumber(t *testing.T) {
	got := episodeOption(provider.Episode{Number: "12"})
	if got != "Ep 12" {
		t.Fatalf("episode option = %q, want Ep 12", got)
	}
}
