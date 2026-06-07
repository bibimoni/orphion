package provider

import (
	"net/http"
	"testing"
)

func TestProviderTypes(t *testing.T) {
	// Verify types compile and have expected fields.
	_ = Anime{
		ID:    "test-id",
		Title: "Test Title",
	}
	_ = Episode{
		ID:      "test-ep",
		Number:  "1",
		SortKey: 1.0,
	}
	_ = Stream{
		URL:     "https://example.com/stream.m3u8",
		Quality: "1080p",
		Headers: http.Header{"Referer": {"https://example.com"}},
	}
}