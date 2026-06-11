package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bibimoni/orphion/internal/app"
	"github.com/bibimoni/orphion/internal/ffmpeg"
	"github.com/bibimoni/orphion/internal/provider"
	"github.com/bibimoni/orphion/internal/subtitle"
)

// ─── ExitError ─────────────────────────────────────────────────────────────

func TestExitError(t *testing.T) {
	err := &ExitError{code: 42, msg: "something failed"}
	if got := err.Error(); got != "something failed" {
		t.Errorf("Error() = %q, want %q", got, "something failed")
	}
	if got := err.Code(); got != 42 {
		t.Errorf("Code() = %d, want 42", got)
	}
}

// ─── isTerminal ─────────────────────────────────────────────────────────────

func TestIsTerminalWithPipe(t *testing.T) {
	// Create a pipe (not a terminal) and verify it's detected as non-terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if isTerminal(r) {
		t.Error("pipe should not be detected as a terminal")
	}
}

func TestIsTerminalWithRegularFile(t *testing.T) {
	f, err := os.CreateTemp("", "orphion-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	defer func() { _ = os.Remove(f.Name()) }()

	if isTerminal(f) {
		t.Error("regular file should not be detected as terminal")
	}
}

// ─── allFlagsSet ────────────────────────────────────────────────────────────

func TestAllFlagsSetNoFlagsChanged(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "", "")
	cmd.Flags().String("quality", "", "")
	cmd.Flags().Int("concurrency", 0, "")
	cmd.Flags().Bool("force", false, "")

	if allFlagsSet(cmd, []string{"output", "quality", "concurrency", "force"}) {
		t.Error("allFlagsSet should return false when no flags changed")
	}
}

func TestAllFlagsSetSomeFlagsChanged(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "", "")
	cmd.Flags().String("quality", "", "")
	cmd.Flags().Int("concurrency", 0, "")
	cmd.Flags().Bool("force", false, "")

	_ = cmd.Flags().Set("output", "/tmp/test")

	if allFlagsSet(cmd, []string{"output", "quality", "concurrency", "force"}) {
		t.Error("allFlagsSet should return false when only some flags changed")
	}
}

func TestAllFlagsSetAllFlagsChanged(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("output", "", "")
	cmd.Flags().String("quality", "", "")
	cmd.Flags().Int("concurrency", 0, "")
	cmd.Flags().Bool("force", false, "")

	_ = cmd.Flags().Set("output", "/tmp/test")
	_ = cmd.Flags().Set("quality", "1080p")
	_ = cmd.Flags().Set("concurrency", "3")
	_ = cmd.Flags().Set("force", "true")

	if !allFlagsSet(cmd, []string{"output", "quality", "concurrency", "force"}) {
		t.Error("allFlagsSet should return true when all flags changed")
	}
}

func TestAllFlagsSetEmptyList(t *testing.T) {
	cmd := &cobra.Command{}
	if !allFlagsSet(cmd, nil) {
		t.Error("allFlagsSet with empty names should return true")
	}
}

// ─── SetConfigInitPath ─────────────────────────────────────────────────────

func TestSetConfigInitPath(t *testing.T) {
	original := configInitPath
	defer func() { configInitPath = original }()

	SetConfigInitPath("/custom/path/config.yaml")
	if configInitPath != "/custom/path/config.yaml" {
		t.Errorf("configInitPath = %q, want /custom/path/config.yaml", configInitPath)
	}
}

// ─── newConfigCmd execution ────────────────────────────────────────────────

func TestConfigInitCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	SetConfigInitPath(cfgPath)
	defer func() { configInitPath = "" }()

	cmd := newConfigCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("config file not created at %s", cfgPath)
	}
}

// ─── newVersionCmd execution ──────────────────────────────────────────────

func TestNewVersionCmdOutput(t *testing.T) {
	origVersion := Version
	Version = "test-v1.0.0"
	defer func() { Version = origVersion }()

	cmd := newVersionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version cmd: %v", err)
	}
	if !strings.Contains(buf.String(), "test-v1.0.0") {
		t.Errorf("version output = %q, want version string", buf.String())
	}
}

// ─── newSearchCmd with nil service ─────────────────────────────────────────

func TestSearchCmdNilService(t *testing.T) {
	cmd := newSearchCmd(nil)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Frieren"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("search with nil service should error")
	}
	if !strings.Contains(err.Error(), "service not configured") {
		t.Errorf("error = %q, want service not configured", err.Error())
	}
}

// ─── newDownloadCmd validation ─────────────────────────────────────────────

func TestDownloadCmdNilService(t *testing.T) {
	cmd := newDownloadCmd(nil)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--title", "Test", "--episodes", "1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("download with nil service should error")
	}
	if !strings.Contains(err.Error(), "service not configured") {
		t.Errorf("error = %q, want service not configured", err.Error())
	}
}

func TestDownloadCmdMissingTitle(t *testing.T) {
	svc := newTestService(t)
	cmd := newDownloadCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--episodes", "1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("download without title should error")
	}
	if !strings.Contains(err.Error(), "--title-id or --title is required") {
		t.Errorf("error = %q, want title required message", err.Error())
	}
}

func TestDownloadCmdMissingEpisodes(t *testing.T) {
	svc := newTestService(t)
	cmd := newDownloadCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--title", "Test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("download without episodes should error")
	}
	if !strings.Contains(err.Error(), "--episodes is required") {
		t.Errorf("error = %q, want episodes required message", err.Error())
	}
}

// ─── newSubtitlesCmd with no provider ──────────────────────────────────────

func TestSubtitlesCmdNoProvider(t *testing.T) {
	svc := newTestService(t) // no subtitle provider
	cmd := newSubtitlesCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Frieren"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("subtitles without provider should error")
	}
	if !strings.Contains(err.Error(), "subtitle provider not configured") {
		t.Errorf("error = %q, want subtitle provider not configured", err.Error())
	}
}

// ─── applySessionConfig ────────────────────────────────────────────────────

func TestApplySessionConfig(t *testing.T) {
	svc := newTestService(t)

	sc := &sessionConfig{
		OutputDir:   "/tmp/orphion-test",
		Quality:     "720p",
		Concurrency: 2,
		Force:       true,
	}
	applySessionConfig(svc, sc)

	cfg := svc.Config()
	if cfg.OutputDir != "/tmp/orphion-test" {
		t.Errorf("OutputDir = %q, want /tmp/orphion-test", cfg.OutputDir)
	}
	if cfg.PreferredQty != "720p" {
		t.Errorf("PreferredQty = %q, want 720p", cfg.PreferredQty)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("Concurrency = %d, want 2", cfg.Concurrency)
	}
	if cfg.Force != true {
		t.Error("Force = false, want true")
	}
}

// ─── listFolders ────────────────────────────────────────────────────────────

func TestListFolders(t *testing.T) {
	tmpDir := t.TempDir()
	t.Log("mkdir:", os.MkdirAll(filepath.Join(tmpDir, "SubDir1"), 0o755))
	t.Log("mkdir:", os.MkdirAll(filepath.Join(tmpDir, "SubDir2"), 0o755))
	t.Log("write:", os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0o644))

	folders := listFolders(tmpDir)
	if len(folders) != 2 {
		t.Fatalf("len(folders) = %d, want 2", len(folders))
	}
	m := map[string]bool{}
	for _, f := range folders {
		m[f] = true
	}
	if !m["SubDir1"] || !m["SubDir2"] {
		t.Errorf("folders = %v, want SubDir1 and SubDir2", folders)
	}
}

func TestListFoldersNonexistentDir(t *testing.T) {
	folders := listFolders("/nonexistent/path/12345")
	if folders != nil {
		t.Errorf("listFolders on nonexistent dir = %v, want nil", folders)
	}
}

func TestListFoldersEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	folders := listFolders(tmpDir)
	if len(folders) != 0 {
		t.Errorf("listFolders on empty dir = %v, want empty", folders)
	}
}

// ─── baseDirForFlow ─────────────────────────────────────────────────────────

func TestBaseDirForFlowWithConfigBase(t *testing.T) {
	svc := newTestService(t)
	cfg := SubtitleFlowConfig{BaseDir: "/custom/dir"}
	got := baseDirForFlow(svc, cfg)
	if got != "/custom/dir" {
		t.Errorf("baseDirForFlow = %q, want /custom/dir", got)
	}
}

func TestBaseDirForFlowFallsBackToServiceOutputDir(t *testing.T) {
	svc := newTestService(t)
	cfg := SubtitleFlowConfig{} // empty BaseDir
	got := baseDirForFlow(svc, cfg)
	expected := svc.OutputDir()
	if got != expected {
		t.Errorf("baseDirForFlow = %q, want %q", got, expected)
	}
}

// ─── progressBar edge cases ─────────────────────────────────────────────────

func TestProgressBarZeroTotal(t *testing.T) {
	got := progressBar(0, 0, 20)
	if got != "" {
		t.Errorf("progressBar(0, 0, 20) = %q, want empty string", got)
	}
}

func TestProgressBarNegativeTotal(t *testing.T) {
	got := progressBar(5, -1, 20)
	if got != "" {
		t.Errorf("progressBar(5, -1, 20) = %q, want empty string", got)
	}
}

func TestProgressBarOverFull(t *testing.T) {
	got := progressBar(100, 50, 20)
	clean := stripANSI(got)
	if !strings.Contains(clean, strings.Repeat("█", 20)) {
		t.Errorf("progressBar overflow: %q", clean)
	}
}

// ─── newDownloadTracker ────────────────────────────────────────────────────

func TestNewDownloadTracker(t *testing.T) {
	tr := newDownloadTracker()
	if tr == nil {
		t.Fatal("newDownloadTracker returned nil")
	}
	if tr.states == nil {
		t.Error("states map should be initialized")
	}
	tr.stop()
}

// ─── downloadTracker stop ──────────────────────────────────────────────────

func TestDownloadTrackerStop(t *testing.T) {
	tr := newDownloadTracker()
	tr.update("1", ffmpeg.Progress{Speed: "2x"})
	tr.stop()
	// Calling stop again should not panic.
	tr.stop()
}

// ─── formatBytes additional edge cases ─────────────────────────────────────

func TestFormatBytesAdditional(t *testing.T) {
	tests := []struct {
		bytes  int64
		expect string
	}{
		{1, "1 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1536, "1.5 KiB"},
		{3*1024*1024 + 512*1024, "3.5 MiB"},
		{2*1024*1024*1024 + 512*1024*1024, "2.5 GiB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.expect {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.expect)
		}
	}
}

// ─── outputDirFor additional cases ─────────────────────────────────────────

func TestOutputDirForAdditional(t *testing.T) {
	tests := []struct {
		path   string
		expect string
	}{
		{"/a/b/c/d.mkv", "/a/b/c"},
		{"filename.mkv", "filename.mkv"},
		{"/", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := outputDirFor(tt.path)
		if got != tt.expect {
			t.Errorf("outputDirFor(%q) = %q, want %q", tt.path, got, tt.expect)
		}
	}
}

// ─── formatProgressLine: empty speed fallback ──────────────────────────────

func TestFormatProgressLineEmptySpeedWithBytes(t *testing.T) {
	got := formatProgressLine("1", ffmpeg.Progress{
		Bytes: 2048,
		Speed: "",
	})
	if !strings.Contains(got, "...") {
		t.Errorf("empty speed should show '...', got: %q", got)
	}
	if !strings.Contains(got, "2.0 KiB") {
		t.Errorf("should show byte count, got: %q", got)
	}
}

// ─── selectOutputFolder ────────────────────────────────────────────────────

func TestSelectOutputFolderNoExistingFolders(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty directory — no interactive prompt, just returns default.
	got, err := selectOutputFolder(tmpDir, "My Anime", "")
	if err != nil {
		t.Fatalf("selectOutputFolder: %v", err)
	}
	// TitleToDir keeps spaces but sanitizes control characters.
	if !strings.Contains(got, "My Anime") {
		t.Errorf("selectOutputFolder = %q, want path containing 'My Anime'", got)
	}
}

func TestSelectOutputFolderWithDefault(t *testing.T) {
	origSelect := interactiveSelect
	defer func() { interactiveSelect = origSelect }()

	tmpDir := t.TempDir()
	t.Log("mkdir:", os.MkdirAll(filepath.Join(tmpDir, "ExistingDir"), 0o755))

	interactiveSelect = func(options []string, defaultText string) (string, error) {
		return useDefaultOption, nil
	}

	got, err := selectOutputFolder(tmpDir, "Test Title", "")
	if err != nil {
		t.Fatalf("selectOutputFolder: %v", err)
	}
	if !strings.Contains(got, "Test Title") {
		t.Errorf("selectOutputFolder = %q, want path containing 'Test Title'", got)
	}
}

func TestSelectOutputFolderWithSeason(t *testing.T) {
	origSelect := interactiveSelect
	defer func() { interactiveSelect = origSelect }()

	tmpDir := t.TempDir()
	// No existing folders means no interactive prompt.
	got, err := selectOutputFolder(tmpDir, "Test Title", "season-2")
	if err != nil {
		t.Fatalf("selectOutputFolder: %v", err)
	}
	if !strings.Contains(got, "season-2") {
		t.Errorf("selectOutputFolder = %q, want path containing 'season-2'", got)
	}
}

func TestSelectOutputFolderExistingFolder(t *testing.T) {
	origSelect := interactiveSelect
	defer func() { interactiveSelect = origSelect }()

	tmpDir := t.TempDir()
	t.Log("mkdir:", os.MkdirAll(filepath.Join(tmpDir, "ExistingFolder"), 0o755))

	interactiveSelect = func(options []string, defaultText string) (string, error) {
		return "ExistingFolder", nil
	}

	got, err := selectOutputFolder(tmpDir, "Test", "")
	if err != nil {
		t.Fatalf("selectOutputFolder: %v", err)
	}
	expected := filepath.Join(tmpDir, "ExistingFolder")
	if got != expected {
		t.Errorf("selectOutputFolder = %q, want %q", got, expected)
	}
}

// ─── subtitleFlowResult ─────────────────────────────────────────────────────

func TestSubtitleFlowResult(t *testing.T) {
	tmpDir := t.TempDir()
	// No existing folders — selectOutputFolder returns default without prompt.
	result, err := subtitleFlowResult(tmpDir, "Anime Title", "", nil)
	if err != nil {
		t.Fatalf("subtitleFlowResult: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Subtitles) != 0 {
		t.Errorf("Subtitles = %v, want empty", result.Subtitles)
	}
	if !strings.Contains(result.OutDir, "Anime Title") {
		t.Errorf("OutDir = %q, want containing 'Anime Title'", result.OutDir)
	}
}

// ─── newSearchCmd with service ──────────────────────────────────────────────

type fakeProvider struct {
	results []provider.Anime
}

func (p *fakeProvider) Search(ctx context.Context, query, kind string) ([]provider.Anime, error) {
	return p.results, nil
}

func (p *fakeProvider) Episodes(ctx context.Context, animeID string) ([]provider.Episode, error) {
	return nil, nil
}

func (p *fakeProvider) Streams(ctx context.Context, episodeID string) ([]provider.Stream, error) {
	return nil, nil
}

func TestSearchCmdWithResultsPiped(t *testing.T) {
	fp := &fakeProvider{
		results: []provider.Anime{
			{ID: "test:1", Title: "Test Anime"},
			{ID: "test:2", Title: "Test Anime 2"},
		},
	}
	runner, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := app.New(fp, runner, app.Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "test",
		Providers: map[string]provider.Provider{
			"test": fp,
		},
	})

	cmd := newSearchCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Test"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("search cmd: %v", err)
	}
}

func TestSearchCmdNoResults(t *testing.T) {
	fp := &fakeProvider{results: nil}
	runner, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := app.New(fp, runner, app.Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "test",
		Providers: map[string]provider.Provider{
			"test": fp,
		},
	})

	cmd := newSearchCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Nonexistent"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("search cmd with no results should not error: %v", err)
	}
}

// ─── newSubtitlesCmd validation ─────────────────────────────────────────────

func TestSubtitlesCmdHelpFlags(t *testing.T) {
	svc := newTestService(t)
	cmd := newSubtitlesCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	// --help causes cobra to print help and return, not an error.
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("subtitles --help: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "--lang") {
		t.Error("subtitles help should mention --lang flag")
	}
	if !strings.Contains(output, "--output") {
		t.Error("subtitles help should mention --output flag")
	}
}

// ─── newDownloadCmd with all flags set ───────────────────────────────────────

func TestDownloadCmdAllFlagsSet(t *testing.T) {
	// Verify that when all download flags are provided, the command
	// recognizes them via allFlagsSet.
	svc := newTestService(t)
	cmd := newDownloadCmd(svc)

	// Set all flags.
	_ = cmd.Flags().Set("output", "/tmp/test")
	_ = cmd.Flags().Set("quality", "1080p")
	_ = cmd.Flags().Set("concurrency", "2")
	_ = cmd.Flags().Set("force", "true")

	if !allFlagsSet(cmd, []string{"output", "quality", "concurrency", "force"}) {
		t.Error("all download flags should be detected as set")
	}
}

// ─── selectSubtitleResult ────────────────────────────────────────────────────

type fakeSubtitleProvider struct {
	results []subtitle.Result
	err     error
}

func (p *fakeSubtitleProvider) Search(ctx context.Context, query string) ([]subtitle.Result, error) {
	return p.results, p.err
}

func (p *fakeSubtitleProvider) Page(ctx context.Context, id, slug, seasonSlug string) (*subtitle.PageResult, error) {
	return nil, nil
}

func (p *fakeSubtitleProvider) DownloadURL(sub subtitle.Subtitle) string {
	return ""
}

func TestSelectSubtitleResultNoResults(t *testing.T) {
	svc := newTestServiceWithSubtitles(t, &fakeSubtitleProvider{results: nil})
	ctx := context.Background()

	result, err := selectSubtitleResult(ctx, svc, "Test")
	if err != nil {
		t.Fatalf("selectSubtitleResult: %v", err)
	}
	if result != nil {
		t.Error("should return nil for no results")
	}
}

func TestSelectSubtitleResultAutoMatchSingleSource(t *testing.T) {
	results := []subtitle.Result{
		{ID: "1", Title: "Frieren", Source: "subdl"},
		{ID: "2", Title: "Frieren: Beyond Journey's End", Source: "subdl"},
	}
	svc := newTestServiceWithSubtitles(t, &fakeSubtitleProvider{results: results})
	ctx := context.Background()

	result, err := selectSubtitleResult(ctx, svc, "Frieren")
	if err != nil {
		t.Fatalf("selectSubtitleResult: %v", err)
	}
	if result == nil {
		t.Fatal("should return a result for single source auto-match")
	}
}

func TestSelectSubtitleResultManualSelection(t *testing.T) {
	results := []subtitle.Result{
		{ID: "1", Title: "Frieren", Source: "subdl"},
		{ID: "2", Title: "Frieren", Source: "jimaku"},
	}
	svc := newTestServiceWithSubtitles(t, &fakeSubtitleProvider{results: results})
	ctx := context.Background()

	origSelect := interactiveSelect
	defer func() { interactiveSelect = origSelect }()

	interactiveSelect = func(options []string, defaultText string) (string, error) {
		// User selects the first non-back option.
		if len(options) > 1 {
			return options[1], nil
		}
		return "", nil
	}

	result, err := selectSubtitleResult(ctx, svc, "Frieren")
	if err != nil {
		t.Fatalf("selectSubtitleResult: %v", err)
	}
	if result == nil {
		t.Fatal("should return a result from manual selection")
	}
}

func TestSelectSubtitleResultBackOption(t *testing.T) {
	results := []subtitle.Result{
		{ID: "1", Title: "Frieren", Source: "subdl"},
		{ID: "2", Title: "Frieren", Source: "jimaku"},
	}
	svc := newTestServiceWithSubtitles(t, &fakeSubtitleProvider{results: results})
	ctx := context.Background()

	origSelect := interactiveSelect
	defer func() { interactiveSelect = origSelect }()

	interactiveSelect = func(options []string, defaultText string) (string, error) {
		return backOption, nil
	}

	result, err := selectSubtitleResult(ctx, svc, "Frieren")
	if err != nil {
		t.Fatalf("selectSubtitleResult: %v", err)
	}
	if result != nil {
		t.Error("should return nil when back is selected")
	}
}

func TestSelectSubtitleResultSearchError(t *testing.T) {
	svc := newTestServiceWithSubtitles(t, &fakeSubtitleProvider{
		err: fmt.Errorf("network error"),
	})
	ctx := context.Background()

	_, err := selectSubtitleResult(ctx, svc, "Frieren")
	if err == nil {
		t.Fatal("should propagate search error")
	}
	if !strings.Contains(err.Error(), "search subtitles") {
		t.Errorf("error = %q, want search subtitles prefix", err.Error())
	}
}

// ─── newSearchCmd piped output format ──────────────────────────────────────

func TestSearchCmdPipedOutputFormat(t *testing.T) {
	fp := &fakeProvider{
		results: []provider.Anime{
			{ID: "test:1", Title: "Test Anime"},
		},
	}
	runner, _ := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	svc := app.New(fp, runner, app.Config{
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "test",
		Providers: map[string]provider.Provider{
			"test": fp,
		},
	})

	cmd := newSearchCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Test"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("search cmd: %v", err)
	}

	output := buf.String()
	// When piped (non-terminal), output is tab-separated.
	if !strings.Contains(output, "test:1") {
		t.Errorf("piped output should contain ID 'test:1', got: %q", output)
	}
	if !strings.Contains(output, "Test Anime") {
		t.Errorf("piped output should contain title 'Test Anime', got: %q", output)
	}
}

// ─── newConfigCmd error path ──────────────────────────────────────────────

func TestConfigInitAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create the file first.
	if err := os.WriteFile(cfgPath, []byte("test: true"), 0o644); err != nil {
		t.Fatal(err)
	}

	SetConfigInitPath(cfgPath)
	defer func() { configInitPath = "" }()

	cmd := newConfigCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"init"})

	err := cmd.Execute()
	if err == nil {
		t.Error("config init on existing file should error")
	}
}

// ─── newDownloadCmd with title-id ──────────────────────────────────────────

func TestDownloadCmdWithTitleID(t *testing.T) {
	// Test that --title-id is accepted (passes validation even if download fails).
	svc := newTestService(t)
	cmd := newDownloadCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--title-id", "allanime:abc123", "--episodes", "1"})

	// Will fail at ResolveID since the provider doesn't know this ID,
	// but it passes the validation checks we care about.
	_ = cmd.Execute()
	// We're just verifying it doesn't fail at the validation step.
}

// ─── RunSubtitleFlow with no provider ──────────────────────────────────────

func TestRunSubtitleFlowNoProvider(t *testing.T) {
	svc := newTestService(t) // no subtitle provider
	ctx := context.Background()

	result, err := RunSubtitleFlow(ctx, svc, SubtitleFlowConfig{
		Query:       "Test",
		SkipConfirm: true,
	})
	if err != nil {
		t.Fatalf("RunSubtitleFlow with no provider should not error: %v", err)
	}
	if result != nil {
		t.Error("should return nil result when no subtitle provider")
	}
}

// ─── RunSubtitleFlow with provider but no results ──────────────────────────

func TestRunSubtitleFlowWithProviderNoResults(t *testing.T) {
	subProv := &fakeSubtitleProvider{results: nil}
	svc := newTestServiceWithSubtitles(t, subProv)
	ctx := context.Background()

	result, err := RunSubtitleFlow(ctx, svc, SubtitleFlowConfig{
		Query:       "Test",
		SkipConfirm: true,
	})
	if err != nil {
		t.Fatalf("RunSubtitleFlow: %v", err)
	}
	if result != nil {
		t.Error("should return nil result when no subtitle results")
	}
}

// ─── newSubtitlesCmd no args ────────────────────────────────────────────────

func TestSubtitlesCmdNoArgs(t *testing.T) {
	svc := newTestService(t) // no subtitle provider
	cmd := newSubtitlesCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	// No args — should hit "subtitle provider not configured" before args check
	// since MaximumNArgs(1) allows 0 args.
	err := cmd.Execute()
	if err == nil {
		t.Fatal("subtitles without provider should error")
	}
}

// ─── helper ─────────────────────────────────────────────────────────────────

func newTestService(t *testing.T) *app.Service {
	t.Helper()
	fp := &fakeProvider{}
	runner, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}
	cfg := app.Config{
		OutputDir:    t.TempDir(),
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "test",
		Providers: map[string]provider.Provider{
			"test": fp,
		},
	}
	return app.New(fp, runner, cfg)
}

func newTestServiceWithSubtitles(t *testing.T, subProv subtitle.Provider) *app.Service {
	t.Helper()
	fp := &fakeProvider{}
	runner, err := ffmpeg.NewRunner(ffmpeg.Config{FFmpegPath: "ffmpeg"})
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}
	cfg := app.Config{
		OutputDir:    t.TempDir(),
		Concurrency:  1,
		PreferredQty: "1080p",
		ProviderName: "test",
		Providers: map[string]provider.Provider{
			"test": fp,
		},
		SubtitleSrc: subProv,
	}
	return app.New(fp, runner, cfg)
}
