package subtitle

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// createTestZIP builds a ZIP archive in memory containing the given entries.
// Each entry is (name, content).
func createTestZIP(t *testing.T, entries []struct{ Name, Body string }) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.Name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", e.Name, err)
		}
		if _, err := f.Write([]byte(e.Body)); err != nil {
			t.Fatalf("write zip entry %q: %v", e.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

func TestExtractFromZIP(t *testing.T) {
	srtContent := "1\n00:00:01,000 --> 00:00:04,000\nHello world\n"
	assContent := "[Script Info]\nTitle: Test\n"
	nfoContent := "<?xml version=\"1.0\"?>\n<episodedetails></episodedetails>\n"

	zipData := createTestZIP(t, []struct{ Name, Body string }{
		{"subs/episode01.srt", srtContent},
		{"subs/episode02.ass", assContent},
		{"subs/info.nfo", nfoContent},
	})

	outDir := t.TempDir()
	got, err := ExtractFromZIP(zipData, outDir)
	if err != nil {
		t.Fatalf("ExtractFromZIP() error: %v", err)
	}

	// Should extract exactly 2 subtitle files (skip .nfo).
	if len(got) != 2 {
		t.Fatalf("len(extracted) = %d, want 2", len(got))
	}

	// Verify .srt content.
	srtPath := filepath.Join(outDir, "episode01.srt")
	b, err := os.ReadFile(srtPath)
	if err != nil {
		t.Fatalf("read %s: %v", srtPath, err)
	}
	if string(b) != srtContent {
		t.Errorf("episode01.srt content = %q, want %q", b, srtContent)
	}

	// Verify .ass content.
	assPath := filepath.Join(outDir, "episode02.ass")
	b, err = os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("read %s: %v", assPath, err)
	}
	if string(b) != assContent {
		t.Errorf("episode02.ass content = %q, want %q", b, assContent)
	}

	// Verify .nfo was NOT extracted.
	nfoPath := filepath.Join(outDir, "info.nfo")
	if _, err := os.Stat(nfoPath); !os.IsNotExist(err) {
		t.Error("info.nfo should not have been extracted")
	}
}

func TestExtractFromZIPFlattensSubdirectories(t *testing.T) {
	zipData := createTestZIP(t, []struct{ Name, Body string }{
		{"deep/nested/path/op.srt", "content"},
	})

	outDir := t.TempDir()
	got, err := ExtractFromZIP(zipData, outDir)
	if err != nil {
		t.Fatalf("ExtractFromZIP() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(extracted) = %d, want 1", len(got))
	}

	expected := filepath.Join(outDir, "op.srt")
	if got[0] != expected {
		t.Errorf("path = %q, want %q", got[0], expected)
	}
}

func TestExtractFromZIPSkipExisting(t *testing.T) {
	zipData := createTestZIP(t, []struct{ Name, Body string }{
		{"episode01.srt", "original"},
	})

	outDir := t.TempDir()

	// Pre-create the file.
	existing := filepath.Join(outDir, "episode01.srt")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ExtractFromZIP(zipData, outDir)
	if err != nil {
		t.Fatalf("ExtractFromZIP() error: %v", err)
	}

	// Should skip the existing file.
	if len(got) != 0 {
		t.Fatalf("len(extracted) = %d, want 0 (skipped existing)", len(got))
	}

	// Verify the original content is untouched.
	b, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "old" {
		t.Errorf("existing file overwritten: got %q, want %q", b, "old")
	}
}

func TestExtractFromZIPEmptyArchive(t *testing.T) {
	zipData := createTestZIP(t, nil)

	outDir := t.TempDir()
	got, err := ExtractFromZIP(zipData, outDir)
	if err != nil {
		t.Fatalf("ExtractFromZIP() error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("len(extracted) = %d, want 0", len(got))
	}
}

func TestExtractFromZIPInvalidData(t *testing.T) {
	outDir := t.TempDir()
	_, err := ExtractFromZIP([]byte("not a zip"), outDir)
	if err == nil {
		t.Error("expected error for invalid ZIP data")
	}
}

func TestIsSubtitleFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"episode.srt", true},
		{"episode.SRT", true},
		{"episode.Srt", true},
		{"episode.ass", true},
		{"episode.ssa", true},
		{"episode.sub", true},
		{"episode.vtt", true},
		{"episode.nfo", false},
		{"episode.txt", false},
		{"episode.mp4", false},
		{"episode", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubtitleFile(tt.name); got != tt.want {
				t.Errorf("isSubtitleFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
