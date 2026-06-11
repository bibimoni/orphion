// Package common defines shared constants used across Orphion.
// All hardcoded defaults, URLs, and magic values should live here
// so they are defined once and referenced everywhere.
package common

import "time"

// ── Default Configuration ──────────────────────────────────────────────

const (
	// DefaultOutputDir is the default download directory (may contain ~).
	DefaultOutputDir = "~/Anime"

	// DefaultQuality is the preferred stream quality.
	DefaultQuality = "1080p"

	// DefaultProvider is the default content provider name.
	DefaultProvider = "allanime"

	// DefaultFFmpegPath is the default path to the FFmpeg binary.
	DefaultFFmpegPath = "ffmpeg"

	// DefaultSubtitleLang is the default subtitle language.
	DefaultSubtitleLang = "english"

	// DefaultConcurrency is the default number of concurrent downloads.
	DefaultConcurrency = 1

	// MaxConcurrency is the maximum allowed concurrency.
	MaxConcurrency = 4

	// DefaultSegmentWorkers is the default number of parallel segment download workers.
	DefaultSegmentWorkers = 4

	// MaxSegmentWorkers is the maximum number of parallel segment download workers.
	MaxSegmentWorkers = 4
)

// ── HTTP & User Agent ─────────────────────────────────────────────────

const (
	// DefaultUserAgent is the HTTP User-Agent used for all upstream requests.
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:150.0) Gecko/20100101 Firefox/150.0"

	// DefaultHTTPTimeout is the default timeout for HTTP requests to providers.
	DefaultHTTPTimeout = 30 * time.Second

	// SubtitleSearchTimeout is the maximum time allowed for a subtitle search
	// across all providers. Slow providers are cut off and their partial
	// results (if any) are discarded.
	SubtitleSearchTimeout = 12 * time.Second

	// DownloadHTTPTimeout is the timeout for downloading subtitle ZIP archives.
	DownloadHTTPTimeout = 60 * time.Second
)

// ── Subtitle Extraction Limits ─────────────────────────────────────────

const (
	// MaxZIPSize is the maximum subtitle ZIP file size we accept downloading (64 MB).
	MaxZIPSize = 64 << 20

	// MaxSubtitleFileSize is the maximum size per extracted subtitle file (4 MB).
	MaxSubtitleFileSize = 4 << 20

	// MaxResponseSize is the maximum upstream HTML/JSON response size (8 MB).
	MaxResponseSize = 8 << 20
)

// ── AllAnime Provider ───────────────────────────────────────────────────

const (
	// AllAnimeAPIURL is the AllAnime GraphQL API endpoint.
	AllAnimeAPIURL = "https://api.allanime.day/api"

	// AllAnimeSiteURL is the AllAnime site URL used for stream resolution.
	AllAnimeSiteURL = "https://youtu-chan.com"

	// AllAnimeMediaURL is the AllAnime media base URL.
	AllAnimeMediaURL = "https://allanime.day"

	// AllAnimeEpisodeQueryHash is the persisted query hash for episode lookups.
	AllAnimeEpisodeQueryHash = "d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec"
)

// ── Bettermelon Provider ─────────────────────────────────────────────────

const (
	// BettermelonAPIURL is the Bettermelon API base endpoint.
	BettermelonAPIURL = "https://api.bettermelon.ru"

	// BettermelonProxyURL is the Bettermelon proxy for CDN access.
	BettermelonProxyURL = "https://proxy.bettermelon.ru"

	// BettermelonDefaultProvider is the default upstream provider within Bettermelon.
	BettermelonDefaultProvider = "hianime"
)

// ── AniList API ──────────────────────────────────────────────────────────

const (
	// AniListAPIURL is the AniList GraphQL API endpoint used for text-to-ID resolution.
	AniListAPIURL = "https://graphql.anilist.co"
)

// ── SubDL Provider ─────────────────────────────────────────────────────

const (
	// SubDLSiteURL is the SubDL website base URL.
	SubDLSiteURL = "https://subdl.com"

	// SubDLDownloadURL is the SubDL download server base URL.
	SubDLDownloadURL = "https://dl.subdl.com"
)

// ── Kitsunekko Provider ──────────────────────────────────────────────────

const (
	// KitsunekkoSiteURL is the kitsunekko.net base URL.
	KitsunekkoSiteURL = "https://kitsunekko.net"

	// KitsunekkoTimeout is the HTTP timeout for kitsunekko requests.
	// Set lower than the default because kitsunekko.net can be slow
	// and directory listings should not take long to fetch.
	KitsunekkoTimeout = 10 * time.Second
)

// ── Jimaku Provider ────────────────────────────────────────────────────────

const (
	// JimakuSiteURL is the jimaku.cc base URL.
	JimakuSiteURL = "https://jimaku.cc"

	// JimakuTimeout is the HTTP timeout for jimaku.cc requests.
	JimakuTimeout = 15 * time.Second
)

// ── Subtitle Matching ──────────────────────────────────────────────────

const (
	// MatchMinScore is the minimum similarity score for BestMatch to
	// consider a result a valid match. Below this, no match is returned.
	MatchMinScore = 0.3

	// RankDefaultMax is the default maximum number of results returned
	// by RankResults when maxResults is <= 0.
	RankDefaultMax = 20

	// RankMinScore is the minimum similarity score used by the CLI when
	// ranking subtitle search results. Results below this score are
	// excluded from the selection list.
	RankMinScore = 0.35

	// FolderMatchMinScore is the minimum similarity score for FolderMatch
	// to include a folder name in results.
	FolderMatchMinScore = 0.1

	// MatchTVBonus is the score bonus added when a result's type is "tv"
	// (anime TV series are the most common search target).
	MatchTVBonus = 0.05

	// MatchSubCountBonus is the score bonus added when a result has
	// subtitles available, helping prefer entries with actual content.
	MatchSubCountBonus = 0.02

	// FuzzyTokenCredit is the partial score credit (0–1) given when a
	// query token fuzzy-matches a result token within edit distance 1.
	// For example, "stein" fuzzy-matches "steins" with this credit.
	FuzzyTokenCredit = 0.8

	// FuzzyEditDistance is the maximum edit distance for fuzzy token
	// matching. Tokens differing by this many edits or fewer count as
	// partial matches (e.g. "stein" ↔ "steins" at distance 1).
	FuzzyEditDistance = 1

	// TokenWeight is the weight given to token-based overlap in the
	// combined similarity score. The remainder (1-TokenWeight) goes
	// to character-level bigram similarity.
	TokenWeight = 0.6

	// CharWeight is the weight given to character-level bigram similarity
	// in the combined score (1 - TokenWeight).
	CharWeight = 0.4
)

// ── Interactive UI ──────────────────────────────────────────────────────

const (
	// InteractiveMaxHeight is the maximum number of visible rows in
	// interactive select/multi-select prompts (pterm WithMaxHeight).
	InteractiveMaxHeight = 20
)
