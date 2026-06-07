# Catalog Provider

## Purpose

Orphion uses one unofficial catalog provider to discover episodic video content
and download it as MKV files through FFmpeg.

The provider is treated as a local implementation detail of Orphion. The CLI
must not wrap or execute ani-cli or any other external downloader.

## Supported Content

Phase 1 treats these catalog groups as first-class:

- Anime
- Drama

The same provider path may return both groups. Search callers must pass an
explicit content type so the user can choose the right catalog view.

## Contract

The provider exposes three operations:

1. Search titles by query and content type.
2. List episodes for a selected title.
3. List stream candidates for a selected episode.

Identifiers are opaque. Orphion must store and pass them back unchanged.

## Reference Behavior

`gonwatch` is the broad behavioral reference for mixed-content browsing because
its README describes anime, TV series, movies, and live sports. Orphion does
not copy its code or depend on its internals.

`ani-cli` remains the behavioral reference for the anime-specific upstream
streaming flow and header quirks.

## Rules

- Keep provider-specific endpoints, headers, and decoding inside the provider
  package.
- Do not expose upstream URLs to callers unless they are already normalized for
  download.
- Do not persist signed URLs or cookies.
- Do not require the user to know upstream provider details.

