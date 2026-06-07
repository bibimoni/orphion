package subtitle

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/paths"
)

// DownloadAndExtract downloads a ZIP file from url and extracts subtitle files
// (.srt, .ass, .ssa, .sub, .vtt) into outputDir. Returns the list of extracted
// file paths. The userAgent is used for the HTTP request.
func DownloadAndExtract(ctx context.Context, url, userAgent, outputDir string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: common.DownloadHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, common.MaxZIPSize))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return ExtractFromZIP(body, outputDir)
}

// ExtractFromZIP extracts subtitle files from ZIP data into outputDir.
func ExtractFromZIP(data []byte, outputDir string) ([]string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	var extracted []string
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isSubtitleFile(f.Name) {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		content, err := io.ReadAll(io.LimitReader(rc, common.MaxSubtitleFileSize))
		_ = rc.Close()
		if err != nil {
			continue
		}

		// Use just the base filename, no subdirectory structure.
		filename := filepath.Base(f.Name)
		outPath := filepath.Join(outputDir, filename)

		// Path traversal guard: verify the resolved path is within outputDir.
		if !paths.IsSafe(outputDir, outPath) {
			continue
		}

		// Skip if file already exists.
		if _, err := os.Stat(outPath); err == nil {
			continue
		}

		if err := os.WriteFile(outPath, content, 0o644); err != nil {
			continue
		}

		extracted = append(extracted, outPath)
	}

	return extracted, nil
}

// isSubtitleFile checks if a filename has a subtitle extension.
func isSubtitleFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".srt", ".ass", ".ssa", ".sub", ".vtt":
		return true
	default:
		return false
	}
}
