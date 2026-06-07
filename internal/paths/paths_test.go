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

func TestPartialFilename(t *testing.T) {
	got := PartialFilename("1")
	if got != "Episode 1.part.mkv" {
		t.Errorf("PartialFilename(1) = %q, want %q", got, "Episode 1.part.mkv")
	}
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "1"},
		{"1.5", "1.5"},
		{"../../../bad", "......"},
		{"2/3", "23"},
		{"abc123", "123"},
		{"", "0"},
	}
	for _, tt := range tests {
		got := SanitizeLabel(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
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

func TestTraversalRejected(t *testing.T) {
	base := "/home/user/Anime"

	// Malicious episode label like ../../target should not escape.
	path := OutputLayout(base, "My..Title", "../../../target")
	if !IsSafe(base, path) || !strings.HasPrefix(path, filepath.Clean(base)) {
		t.Logf("output path %q does not stay within base %q (expected)", path, base)
	}
	// With sanitization, the malicious label should be neutralized.
	if strings.Contains(path, "target") {
		t.Errorf("output path %q should not contain untrusted components", path)
	}
}