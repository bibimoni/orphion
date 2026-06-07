package paths

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TitleToDir returns a sanitized directory name for an anime title.
// Removes path separators, control characters, and trailing dots/spaces.
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

// EpisodeFilename returns the filename for a given episode number.
func EpisodeFilename(number string) string {
	return fmt.Sprintf("Episode %s.mkv", number)
}

// PartialFilename returns the temporary partial filename for a given episode.
func PartialFilename(number string) string {
	return fmt.Sprintf("Episode %s.part.mkv", number)
}

// OutputLayout returns the full path for the final episode file.
func OutputLayout(base, title, number string) string {
	dir := filepath.Join(base, TitleToDir(title))
	return filepath.Join(dir, EpisodeFilename(number))
}

// PartialPath returns the path for an in-progress download file.
func PartialPath(base, title, number string) string {
	dir := filepath.Join(base, TitleToDir(title))
	return filepath.Join(dir, PartialFilename(number))
}

// IsSafe checks that a resolved path is within the base directory.
func IsSafe(base, resolved string) bool {
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}