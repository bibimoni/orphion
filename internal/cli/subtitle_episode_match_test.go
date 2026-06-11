package cli

import (
	"reflect"
	"testing"

	"github.com/bibimoni/orphion/internal/subtitle"
)

func TestMatchSubtitlesToEpisodesUsesEpisodeMetadataAndBestDownload(t *testing.T) {
	subs := []subtitle.Subtitle{
		{ID: 1, Episode: 1, Title: "Episode 1 low", Downloads: 10},
		{ID: 2, Episode: 1, Title: "Episode 1 best", Downloads: 50},
		{ID: 3, Episode: 2, Title: "Episode 2", Downloads: 20},
	}

	got, missing := matchSubtitlesToEpisodes(subs, []string{"1", "2"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	gotIDs := []int{got[0].ID, got[1].ID}
	if !reflect.DeepEqual(gotIDs, []int{2, 3}) {
		t.Fatalf("matched IDs = %v, want [2 3]", gotIDs)
	}
}

func TestMatchSubtitlesToEpisodesParsesEpisodeFromFilename(t *testing.T) {
	subs := []subtitle.Subtitle{
		{ID: 1, Title: "Show.S01E03.1080p.WEB-DL.ass", Downloads: 20},
		{ID: 2, Title: "Show - Episode 04.srt", Downloads: 10},
	}

	got, missing := matchSubtitlesToEpisodes(subs, []string{"3", "4"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	gotIDs := []int{got[0].ID, got[1].ID}
	if !reflect.DeepEqual(gotIDs, []int{1, 2}) {
		t.Fatalf("matched IDs = %v, want [1 2]", gotIDs)
	}
}

func TestMatchSubtitlesToEpisodesReportsMissing(t *testing.T) {
	subs := []subtitle.Subtitle{{ID: 1, Episode: 1, Title: "Episode 1"}}

	got, missing := matchSubtitlesToEpisodes(subs, []string{"1", "2", "SP"})
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("matched = %#v, want episode 1", got)
	}
	if !reflect.DeepEqual(missing, []string{"2", "SP"}) {
		t.Fatalf("missing = %v, want [2 SP]", missing)
	}
}
