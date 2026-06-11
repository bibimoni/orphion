package common

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigConstantsNonZero(t *testing.T) {
	if DefaultOutputDir == "" {
		t.Error("DefaultOutputDir is empty")
	}
	if DefaultQuality == "" {
		t.Error("DefaultQuality is empty")
	}
	if DefaultProvider == "" {
		t.Error("DefaultProvider is empty")
	}
	if DefaultFFmpegPath == "" {
		t.Error("DefaultFFmpegPath is empty")
	}
	if DefaultSubtitleLang == "" {
		t.Error("DefaultSubtitleLang is empty")
	}
	if DefaultConcurrency <= 0 {
		t.Errorf("DefaultConcurrency = %d, want positive", DefaultConcurrency)
	}
	if MaxConcurrency < DefaultConcurrency {
		t.Errorf("MaxConcurrency = %d < DefaultConcurrency = %d", MaxConcurrency, DefaultConcurrency)
	}
}

func TestHTTPConstantsNonZero(t *testing.T) {
	if DefaultUserAgent == "" {
		t.Error("DefaultUserAgent is empty")
	}
	if DefaultHTTPTimeout <= 0 {
		t.Errorf("DefaultHTTPTimeout = %v, want positive", DefaultHTTPTimeout)
	}
	if SubtitleSearchTimeout <= 0 {
		t.Errorf("SubtitleSearchTimeout = %v, want positive", SubtitleSearchTimeout)
	}
	if DownloadHTTPTimeout <= 0 {
		t.Errorf("DownloadHTTPTimeout = %v, want positive", DownloadHTTPTimeout)
	}
	if SubtitleSearchTimeout >= DefaultHTTPTimeout {
		t.Errorf("SubtitleSearchTimeout = %v should be less than DefaultHTTPTimeout = %v", SubtitleSearchTimeout, DefaultHTTPTimeout)
	}
}

func TestExtractionLimitConstantsPositive(t *testing.T) {
	if MaxZIPSize <= 0 {
		t.Errorf("MaxZIPSize = %d, want positive", MaxZIPSize)
	}
	if MaxSubtitleFileSize <= 0 {
		t.Errorf("MaxSubtitleFileSize = %d, want positive", MaxSubtitleFileSize)
	}
	if MaxResponseSize <= 0 {
		t.Errorf("MaxResponseSize = %d, want positive", MaxResponseSize)
	}
	if MaxSubtitleFileSize >= MaxZIPSize {
		t.Errorf("MaxSubtitleFileSize = %d should be less than MaxZIPSize = %d", MaxSubtitleFileSize, MaxZIPSize)
	}
}

func TestURLConstantsAreValid(t *testing.T) {
	urls := map[string]string{
		"AllAnimeAPIURL":      AllAnimeAPIURL,
		"AllAnimeSiteURL":     AllAnimeSiteURL,
		"AllAnimeMediaURL":    AllAnimeMediaURL,
		"BettermelonAPIURL":   BettermelonAPIURL,
		"BettermelonProxyURL": BettermelonProxyURL,
		"AniListAPIURL":       AniListAPIURL,
		"SubDLSiteURL":        SubDLSiteURL,
		"SubDLDownloadURL":    SubDLDownloadURL,
		"KitsunekkoSiteURL":   KitsunekkoSiteURL,
		"JimakuSiteURL":       JimakuSiteURL,
	}
	for name, url := range urls {
		if url == "" {
			t.Errorf("%s is empty", name)
			continue
		}
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("%s = %q, want https:// prefix", name, url)
		}
	}
}

func TestMatchScoreConstantsInRange(t *testing.T) {
	if MatchMinScore <= 0 || MatchMinScore > 1 {
		t.Errorf("MatchMinScore = %f, want (0,1]", MatchMinScore)
	}
	if RankMinScore <= 0 || RankMinScore > 1 {
		t.Errorf("RankMinScore = %f, want (0,1]", RankMinScore)
	}
	if FolderMatchMinScore <= 0 || FolderMatchMinScore > 1 {
		t.Errorf("FolderMatchMinScore = %f, want (0,1]", FolderMatchMinScore)
	}
	if RankDefaultMax <= 0 {
		t.Errorf("RankDefaultMax = %d, want positive", RankDefaultMax)
	}
}

func TestMatchBonusConstantsSmall(t *testing.T) {
	if MatchTVBonus <= 0 || MatchTVBonus > 0.5 {
		t.Errorf("MatchTVBonus = %f, want (0,0.5]", MatchTVBonus)
	}
	if MatchSubCountBonus <= 0 || MatchSubCountBonus > 0.5 {
		t.Errorf("MatchSubCountBonus = %f, want (0,0.5]", MatchSubCountBonus)
	}
}

func TestFuzzyTokenConstantsValid(t *testing.T) {
	if FuzzyTokenCredit <= 0 || FuzzyTokenCredit > 1 {
		t.Errorf("FuzzyTokenCredit = %f, want (0,1]", FuzzyTokenCredit)
	}
	if FuzzyEditDistance < 1 {
		t.Errorf("FuzzyEditDistance = %d, want >= 1", FuzzyEditDistance)
	}
}

func TestWeightConstantsSumToOne(t *testing.T) {
	sum := TokenWeight + CharWeight
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("TokenWeight + CharWeight = %f, want ~1.0", sum)
	}
	if TokenWeight <= 0 || CharWeight <= 0 {
		t.Errorf("TokenWeight = %f, CharWeight = %f, both should be positive", TokenWeight, CharWeight)
	}
}

func TestTimeoutsAreReasonable(t *testing.T) {
	// Timeouts should be at least 1 second and less than 5 minutes.
	minTimeout := 1 * time.Second
	maxTimeout := 5 * time.Minute

	timeouts := map[string]time.Duration{
		"DefaultHTTPTimeout":    DefaultHTTPTimeout,
		"SubtitleSearchTimeout": SubtitleSearchTimeout,
		"DownloadHTTPTimeout":   DownloadHTTPTimeout,
		"KitsunekkoTimeout":     KitsunekkoTimeout,
		"JimakuTimeout":         JimakuTimeout,
	}
	for name, d := range timeouts {
		if d < minTimeout {
			t.Errorf("%s = %v, too short (min %v)", name, d, minTimeout)
		}
		if d > maxTimeout {
			t.Errorf("%s = %v, too long (max %v)", name, d, maxTimeout)
		}
	}
}

func TestInteractiveMaxHeightPositive(t *testing.T) {
	if InteractiveMaxHeight <= 0 {
		t.Errorf("InteractiveMaxHeight = %d, want positive", InteractiveMaxHeight)
	}
}

func TestBettermelonConstantsNonZero(t *testing.T) {
	if BettermelonDefaultProvider == "" {
		t.Error("BettermelonDefaultProvider is empty")
	}
	if BettermelonAPIURL == "" {
		t.Error("BettermelonAPIURL is empty")
	}
	if BettermelonProxyURL == "" {
		t.Error("BettermelonProxyURL is empty")
	}
}

func TestAllAnimeQueryHashNonEmpty(t *testing.T) {
	if AllAnimeEpisodeQueryHash == "" {
		t.Error("AllAnimeEpisodeQueryHash is empty")
	}
}
