package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTitleToDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Frieren: Beyond Journey's End", "Frieren Beyond Journey's End"},
		{"Test/Title", "Test Title"},
		{"Test..Title", "Test Title"},
		{"  Spaces  ", "Spaces"},
		{"EndsWithDot.", "EndsWithDot"},
		{"", "unknown"},
		{"..", "unknown"},
	}
	for _, tt := range tests {
		got := TitleToDir(tt.input)
		if got != tt.want {
			t.Errorf("TitleToDir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEpisodeFilename(t *testing.T) {
	got := EpisodeFilename("1")
	if got != "Episode 1.mkv" {
		t.Errorf("EpisodeFilename(1) = %q, want %q", got, "Episode 1.mkv")
	}
}

func TestSafePath(t *testing.T) {
	base := "/home/user/Anime"
	ok := IsSafe(base, "/home/user/Anime/Title/Episode 1.mkv")
	if !ok {
		t.Error("safe path rejected")
	}
	bad := IsSafe(base, "/home/user/Malicious/../Title/file")
	if bad {
		t.Error("path traversal was not rejected")
	}
}

func TestContainment(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "Anime")
	os.MkdirAll(base, 0o755)

	path := OutputLayout(base, "Test", "1")
	if !strings.HasPrefix(path, filepath.Clean(base)) {
		t.Fatalf("output escapes base: %q not under %q", path, base)
	}

	partPath := PartialPath(base, "Test", "1")
	if !strings.HasPrefix(partPath, filepath.Clean(base)) {
		t.Fatalf("partial path escapes base: %q", partPath)
	}
}
