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
