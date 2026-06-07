package subtitle

import "testing"

func TestProviderTypes(t *testing.T) {
	_ = Result{
		ID:       "sd1300065",
		Title:    "Naruto",
		Type:     "tv",
		Year:     2002,
		Slug:     "naruto",
		SubCount: 10,
	}
	_ = Season{
		Slug: "first-season",
		Name: "Season 1",
	}
	_ = Subtitle{
		ID:         3455495,
		Language:   "english",
		Quality:    "other",
		Link:       "3455495-8378310.zip",
		BucketLink: "3455495/8378310.zip",
		Author:     "mo92",
		Season:     1,
		Episode:    0,
		Title:      "Naruto S01",
		Downloads:  1466,
		Releases:   []string{"Naruto Season 1 x264 v3 JySzE"},
	}
	_ = PageResult{
		Seasons:   []Season{{Slug: "first-season", Name: "Season 1"}},
		Subtitles: []Subtitle{{ID: 1, Language: "english"}},
	}
}
