// Package torrent provides a BitTorrent download client using magnet URIs.
package torrent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
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
	// OnProgress is called periodically with download progress.
	// Pct is 0-100, downloaded/total in bytes, speed in bytes/sec, peers and seeders count.
	OnProgress func(pct float64, downloaded, total int64, speed float64, peers, seeders int)
}

// Client wraps anacrolix/torrent for magnet URI downloads.
// The underlying anacrolix client is initialized lazily on first use
// to avoid UPnP/port-mapping noise at startup.
type Client struct {
	cfg    Config
	client *torrent.Client
	init   sync.Once
	initOk bool
}

// NewClient creates a torrent client with lazy initialization.
// The underlying anacrolix client is NOT created until the first
// call to Download, so UPnP/port-mapping warnings are deferred
// until a magnet URI actually needs to be downloaded.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// initClient lazily creates the underlying anacrolix/torrent client.
func (c *Client) initClient() error {
	var initErr error
	c.init.Do(func() {
		dataDir := c.cfg.DataDir
		if dataDir == "" {
			dataDir = filepath.Join(os.TempDir(), "orphion-torrent")
		}
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			initErr = fmt.Errorf("torrent: create data dir: %w", err)
			return
		}

		clientCfg := newAnacrolixConfig(dataDir)

		cl, err := torrent.NewClient(clientCfg)
		if err != nil {
			initErr = fmt.Errorf("torrent: init client: %w", err)
			return
		}

		c.client = cl
		c.initOk = true
	})
	return initErr
}

func newAnacrolixConfig(dataDir string) *torrent.ClientConfig {
	clientCfg := torrent.NewDefaultClientConfig()
	clientCfg.DataDir = dataDir
	clientCfg.NoDefaultPortForwarding = true
	return clientCfg
}

// Close shuts down the torrent client gracefully.
// No-op if the client was never initialized (lazy init).
func (c *Client) Close() error {
	if !c.initOk {
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

// SetOnProgress sets the progress callback for download updates.
func (c *Client) SetOnProgress(fn func(pct float64, downloaded, total int64, speed float64, peers, seeders int)) {
	c.cfg.OnProgress = fn
}

// Download downloads a torrent from a magnet URI to the given output directory.
// It returns the paths to the downloaded file(s).
func (c *Client) Download(ctx context.Context, magnetURI, outputDir string) ([]string, error) {
	if err := c.initClient(); err != nil {
		return nil, err
	}

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

	// Report torrent name via progress callback (pct < 0 signals "got metadata").
	if c.cfg.OnProgress != nil {
		c.cfg.OnProgress(-1, 0, info.TotalLength(), 0, 0, 0)
	}

	// Progress ticker while waiting for completion.
	doneCh := t.Complete().On()
	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	downloadTimeout := c.cfg.Timeout
	if downloadTimeout <= 0 {
		downloadTimeout = 30 * time.Minute
	}
	deadline := time.Now().Add(downloadTimeout)
	lastBytes := int64(0)
	lastTick := time.Now()

	for {
		select {
		case <-doneCh:
			// Download complete — copy files to output.
			return c.copyFiles(t, outputDir)
		case <-progressTicker.C:
			stats := t.Stats()
			total := t.Length()
			if total > 0 {
				downloaded := stats.BytesRead.Int64()
				pct := float64(downloaded) / float64(total) * 100
				now := time.Now()
				elapsed := now.Sub(lastTick).Seconds()
				speed := 0.0
				if elapsed > 0 {
					speed = float64(downloaded-lastBytes) / elapsed
				}
				lastBytes = downloaded
				lastTick = now
				if c.cfg.OnProgress != nil {
					c.cfg.OnProgress(pct, downloaded, total, speed, stats.ActivePeers, stats.ConnectedSeeders)
				}
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
