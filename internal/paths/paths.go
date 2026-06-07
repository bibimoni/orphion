// Package paths generates output file paths for downloaded episodes.
package paths

import (
	"path/filepath"
	"strings"
)

// TitleToDir returns a sanitized directory name for an anime title.
func TitleToDir(title string) string {
	const invalid = `<>:"/\|?*` + "\x00"
	for _, c := range invalid {
		title = strings.ReplaceAll(title, string(c), " ")
	}
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, ". ")
	for _, ws := range []string{"..", "  "} {
		for strings.Contains(title, ws) {
			title = strings.ReplaceAll(title, ws, " ")
		}
	}
	if title == "" {
		return "unknown"
	}
	return title
}

// SanitizeLabel restricts episode label to numeric characters and dots.
func SanitizeLabel(label string) string {
	filtered := make([]byte, 0, len(label))
	for i := range len(label) {
		c := label[i]
		if (c >= '0' && c <= '9') || c == '.' {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return "0"
	}
	return string(filtered)
}

// EpisodeFilename returns the filename for a given episode number.
func EpisodeFilename(number string) string {
	safe := SanitizeLabel(number)
	return "Episode " + safe + ".mkv"
}

// PartialFilename returns the temporary partial filename.
func PartialFilename(number string) string {
	safe := SanitizeLabel(number)
	return "Episode " + safe + ".part.mkv"
}

// OutputLayout builds the full path, sanitizing episode labels.
func OutputLayout(base, title, number string) string {
	dir := filepath.Join(base, TitleToDir(title))
	safe := SanitizeLabel(number)
	return filepath.Join(dir, "Episode "+safe+".mkv")
}

// PartialPath builds the path for a partial download.
func PartialPath(base, title, number string) string {
	dir := filepath.Join(base, TitleToDir(title))
	safe := SanitizeLabel(number)
	return filepath.Join(dir, "Episode "+safe+".part.mkv")
}

// IsSafe checks that resolved is within base directory.
func IsSafe(base, resolved string) bool {
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}
