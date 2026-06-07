// Package torrent provides a BitTorrent download client using magnet URIs.
package torrent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/anacrolix/torrent"
)

// Config holds torrent client configuration.
type Config struct {
	// DataDir is the directory for torrent data and metadata storage.
	// Defaults to os.TempDir() if empty.
	DataDir string
	// Timeout is the maximum time to wait for download completion.
	// 0 means no timeout (waits indefinitely).
	Timeout time.Duration
}

// Client wraps anacrolix/torrent for magnet URI downloads.
type Client struct {
	cfg    Config
	client *torrent.Client
}

// NewClient creates a new torrent client.
func NewClient(cfg Config) (*Client, error) {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "orphion-torrent")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("torrent: create data dir: %w", err)
	}

	clientCfg := torrent.NewDefaultClientConfig()
	clientCfg.DataDir = dataDir

	cl, err := torrent.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("torrent: init client: %w", err)
	}

	return &Client{
		cfg:    cfg,
		client: cl,
	}, nil
}

// Close shuts down the torrent client gracefully.
func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}
	errs := c.client.Close()
	var first error
	for _, e := range errs {
		if e != nil && first == nil {
			first = e
		}
	}
	return first
}

// Download downloads a torrent from a magnet URI to the given output directory.
// It returns the paths to the downloaded file(s).
func (c *Client) Download(ctx context.Context, magnetURI, outputDir string) ([]string, error) {
	t, err := c.client.AddMagnet(magnetURI)
	if err != nil {
		return nil, fmt.Errorf("torrent: add magnet: %w", err)
	}

	// Wait for torrent info (metadata) with a timeout.
	infoTimeout := 60 * time.Second
	if c.cfg.Timeout > 0 && c.cfg.Timeout < infoTimeout {
		infoTimeout = c.cfg.Timeout
	}

	select {
	case <-t.GotInfo():
		// Metadata received.
	case <-time.After(infoTimeout):
		t.Drop()
		return nil, fmt.Errorf("torrent: timed out waiting for metadata after %s", infoTimeout)
	case <-ctx.Done():
		t.Drop()
		return nil, fmt.Errorf("torrent: canceled waiting for metadata: %w", ctx.Err())
	}

	info := t.Info()
	if info == nil {
		t.Drop()
		return nil, fmt.Errorf("torrent: no info after metadata received")
	}

	// Download all files in the torrent.
	t.DownloadAll()

	fmt.Fprintf(os.Stderr, "  Downloading: %s (%d files, %s)\n",
		info.Name, len(info.Files), humanizeBytes(info.TotalLength()))

	// Progress ticker while waiting for completion.
	doneCh := t.Complete().On()
	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	downloadTimeout := c.cfg.Timeout
	if downloadTimeout <= 0 {
		downloadTimeout = 30 * time.Minute
	}
	deadline := time.Now().Add(downloadTimeout)

	for {
		select {
		case <-doneCh:
			// Download complete — copy files to output.
			return c.copyFiles(t, outputDir)
		case <-progressTicker.C:
			stats := t.Stats()
			total := t.Length()
			if total > 0 {
				pct := float64(stats.BytesRead.Int64()) / float64(total) * 100
				fmt.Fprintf(os.Stderr, "  Progress: %.1f%% (%s / %s) seeders=%d\n",
					pct,
					humanizeBytes(stats.BytesRead.Int64()),
					humanizeBytes(total),
					stats.ConnectedSeeders,
				)
			}
			if time.Now().After(deadline) {
				t.Drop()
				return nil, fmt.Errorf("torrent: download timed out after %s", downloadTimeout)
			}
		case <-ctx.Done():
			t.Drop()
			return nil, fmt.Errorf("torrent: canceled: %w", ctx.Err())
		}
	}
}

// copyFiles copies completed torrent files to the output directory.
func (c *Client) copyFiles(t *torrent.Torrent, outputDir string) ([]string, error) {
	files := t.Files()
	if len(files) == 0 {
		t.Drop()
		return nil, fmt.Errorf("torrent: no files in torrent")
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Drop()
		return nil, fmt.Errorf("torrent: create output dir: %w", err)
	}

	var paths []string
	for _, f := range files {
		dst := filepath.Join(outputDir, f.DisplayPath())
		if err := copyFile(f, dst); err != nil {
			t.Drop()
			return paths, fmt.Errorf("torrent: copy %s: %w", f.DisplayPath(), err)
		}
		paths = append(paths, dst)
	}

	t.Drop()
	return paths, nil
}

// copyFile reads a completed torrent file and writes it to dst.
func copyFile(f *torrent.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	reader := f.NewReader()
	defer func() { _ = reader.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// humanizeBytes converts bytes to a human-readable string.
func humanizeBytes(b int64) string {
	const (
		kiB = 1024
		miB = kiB * 1024
		giB = miB * 1024
	)
	switch {
	case b >= giB:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(giB))
	case b >= miB:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(miB))
	case b >= kiB:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(kiB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
