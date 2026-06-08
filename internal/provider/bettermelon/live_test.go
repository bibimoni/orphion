package bettermelon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveProviderFrieren(t *testing.T) {
	if os.Getenv("ORPHION_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("set ORPHION_LIVE_PROVIDER_TEST=1 to contact the live provider")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "154587", "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("live search returned no results")
	}
	if results[0].Title == "" {
		t.Fatal("live search returned empty title")
	}

	episodes, err := client.Episodes(ctx, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) == 0 {
		t.Fatal("live episode lookup returned no episodes")
	}

	streams, err := client.Streams(ctx, episodes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) == 0 {
		t.Fatal("live stream lookup returned no streams")
	}
	t.Logf("stream URL (redacted): length=%d, quality=%q", len(streams[0].URL), streams[0].Quality)
}

// TestLiveDownload verifies the full end-to-end pipeline:
// search → episodes → streams → local m3u8 rewrite → ffmpeg download.
// It downloads 5 seconds of video and checks the output file exists and is non-empty.
func TestLiveDownload(t *testing.T) {
	if os.Getenv("ORPHION_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("set ORPHION_LIVE_PROVIDER_TEST=1 to run live download test")
	}

	// Check ffmpeg is available.
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found in PATH")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	// 1. Search for Frieren (AniList ID 154587).
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "154587", "anime")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no search results")
	}
	t.Logf("found: %q (id=%s)", results[0].Title, results[0].ID)

	// 2. Get episodes.
	episodes, err := client.Episodes(ctx, results[0].ID)
	if err != nil {
		t.Fatalf("episodes: %v", err)
	}
	if len(episodes) == 0 {
		t.Fatal("no episodes")
	}
	t.Logf("episode 1: id=%q number=%q", episodes[0].ID, episodes[0].Number)

	// 3. Get streams.
	streams, err := client.Streams(ctx, episodes[0].ID)
	if err != nil {
		t.Fatalf("streams: %v", err)
	}
	if len(streams) == 0 {
		t.Fatal("no streams")
	}
	stream := streams[0]
	t.Logf("stream: file://=%v headers=%v", strings.HasPrefix(stream.URL, "file://"), stream.Headers)

	// 4. Download 5 seconds with ffmpeg.
	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "test.ts")

	args := []string{
		"-nostdin", "-hide_banner", "-loglevel", "warning",
	}
	if !strings.HasPrefix(stream.URL, "file://") {
		args = append(args, "-headers",
			"Referer: "+stream.Headers.Get("Referer")+"\r\nUser-Agent: "+stream.Headers.Get("User-Agent")+"\r\n")
	}
	args = append(args,
		"-allowed_extensions", "ALL",
		"-allowed_segment_extensions", "3gp,aac,avi,ac3,eac3,flac,mkv,m3u8,m4a,m4s,m4v,mpg,mov,mp2,mp3,mp4,mpeg,mpegts,ogg,ogv,oga,ts,vob,vtt,wav,webvtt,cmfv,cmfa,ec3,fmp4,html,jpg,jpeg,js,css,txt,png,webp,gif,svg,ico,json,xml",
		"-extension_picky", "0",
	)
	if strings.HasPrefix(stream.URL, "file://") {
		args = append(args, "-protocol_whitelist", "file,https,http,crypto,data,tcp,tls")
	}
	args = append(args,
		"-i", stream.URL,
		"-map", "0:v?",
		"-map", "0:a?",
		"-c", "copy",
		"-t", "5",
		"-y", outFile,
	)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up temp m3u8 files.
		cleanupStreamTemp(stream.URL)
		t.Fatalf("ffmpeg: %v\n%s", err, string(out))
	}
	defer cleanupStreamTemp(stream.URL)

	// 5. Verify output file exists and is non-empty.
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
	t.Logf("download OK: %d bytes in 5s", info.Size())
}

// cleanupStreamTemp removes temp m3u8 files created by buildStreams
// and closes any associated local segment proxy servers.
func cleanupStreamTemp(streamURL string) {
	if !strings.HasPrefix(streamURL, "file://") {
		return
	}
	CloseSegmentServers(streamURL)
	path := streamURL[len("file://"):]
	// If inside a bettermelon-m3u8 temp directory, remove the whole directory.
	dir := filepath.Dir(path)
	if strings.Contains(filepath.Base(dir), "bettermelon-m3u8") {
		_ = os.RemoveAll(dir)
		return
	}
	_ = os.Remove(path)
}

// TestSmokeAPI verifies that the Bettermelon search and episodes APIs are
// reachable. It does NOT test stream downloads (which 403 from CI IPs).
// Triggered by ORPHION_SMOKE_ANIME_ID env var (set by CI).
func TestSmokeAPI(t *testing.T) {
	animeID := os.Getenv("ORPHION_SMOKE_ANIME_ID")
	if animeID == "" {
		t.Skip("set ORPHION_SMOKE_ANIME_ID to run smoke test")
	}

	client, err := NewClient(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	episodes, err := client.Episodes(ctx, animeID)
	if err != nil {
		t.Fatalf("episodes API unreachable: %v", err)
	}
	if len(episodes) == 0 {
		t.Fatal("episodes API returned zero episodes")
	}
	t.Logf("OK: %d episodes for anime ID %s", len(episodes), animeID)
}
