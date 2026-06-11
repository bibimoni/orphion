# Troubleshooting

Common issues and how to fix them.

## FFmpeg Not Found

**Error:** `exec: "ffmpeg": executable file not found in $PATH`

Orphion requires FFmpeg to download and remux streams into MKV files.

### Install FFmpeg

| Platform | Command |
|----------|---------|
| macOS | `brew install ffmpeg` |
| Ubuntu/Debian | `sudo apt install ffmpeg` |
| Fedora | `sudo dnf install ffmpeg` |
| Arch | `sudo pacman -S ffmpeg` |
| Windows | Download from [ffmpeg.org](https://ffmpeg.org/download.html) |

### Verify installation

```bash
ffmpeg -version
```

### Custom FFmpeg path

If FFmpeg is installed in a non-standard location, set the path in your config:

```bash
orphion config init
```

Then edit `~/.config/orphion/config.yaml`:

```yaml
ffmpeg_path: /usr/local/bin/ffmpeg
```

To specify a custom FFmpeg path, set it in your config file (see below).

---

## No Results Found

**Error:** `No results found` or `Found 0 result(s)`

### Try different search terms

- Use the **English title** (e.g., `Frieren` instead of `Sousou no Frieren`)
- Use a **shorter query** (e.g., `Naruto` instead of `Naruto Shippuden`)
- Try **romanized Japanese** (e.g., `Shingeki no Kyojin`)
- Avoid special characters or season numbers in the query

### Switch providers

Different providers have different catalogs. Try switching:

```bash
# In interactive mode, select a different provider when prompted
orphion

# Or specify a provider in config
# Edit ~/.config/orphion/config.yaml and change the provider field
```

See [Providers](providers.md) for details on each provider's catalog.

---

## Download Failed: No Streams

**Error:** `no streams for episode X` or `Download failed: no streams`

This means the provider found the episode listing but couldn't retrieve a playable stream.

### Common causes

- The content may not be available on that provider
- The provider's upstream source may be temporarily down
- The episode may be too new (not yet uploaded)

### Workarounds

1. **Switch to a different provider** and try again
2. **Try a different episode** — some episodes may be available while others are not
3. **Wait and retry later** — upstream sources frequently update their catalogs

---

## Config File Error

**Error:** `decode config: yaml: unmarshal errors` or similar

This usually means the config file has an invalid or misspelled field.

### Reset config

```bash
# Remove the broken config
rm ~/.config/orphion/config.yaml

# Create a fresh default config
orphion config init
```

### Validate config manually

The config file (`~/.config/orphion/config.yaml`) should look like this:

```yaml
output_dir: ~/Anime
preferred_quality: 1080p
concurrency: 1
provider: allanime
ffmpeg_path: ffmpeg
subtitle_lang: english
```

Common mistakes:
- `concurrency` must be between 1 and 4
- Unknown YAML fields are rejected — check for typos
- Do not quote numeric values (use `concurrency: 2`, not `concurrency: "2"`)

---

## Permission Denied on Output Directory

**Error:** `create output dir: mkdir /path: permission denied`

### Fix permissions

```bash
# Check current permissions
ls -la ~/Anime

# Fix ownership
sudo chown -R $(whoami) ~/Anime

# Or choose a different output directory
orphion download --title "Frieren" --episodes 1 --output ~/Downloads
```

### Common causes

- The output directory (`~/Anime` by default) is owned by another user
- The directory is on a read-only filesystem
- Disk is full (check with `df -h`)

---

## Episode Not Found

**Error:** `no episodes matching "X"` or `warning: missing episodes: [...]`

Episode numbering varies by provider. Some providers number specials as episode 0, others start at episode 1.

### Tips

- Use `all` to download all available episodes:
  ```bash
  orphion download --title "Naruto" --episodes all
  ```
- Check which episodes are available first:
  ```bash
  orphion search "Naruto"
  ```
  Then use the interactive mode to see the full episode list.

- Episode expressions are flexible:
  ```
  1-4           Episodes 1 through 4
  1,3,7         Specific episodes
  1-3,7,10-12   Mixed ranges and lists
  ```

---

## Network Timeout

**Error:** `context deadline exceeded` or `fetch: connection refused`

### Check your connection

```bash
# Test basic connectivity
curl -I https://api.allanime.day

# Test with a short timeout
curl --connect-timeout 5 https://api.bettermelon.ru
```

### Common causes

- **VPN or proxy interference** — try disabling VPN
- **DNS issues** — try `8.8.8.8` or `1.1.1.1` as your DNS server
- **Firewall blocking** — ensure outbound HTTPS (port 443) is allowed
- **Provider downtime** — providers occasionally go offline; try again later or switch providers

### Regional restrictions

Some providers may be blocked in certain regions. If you experience consistent timeouts with one provider, try switching to another.

---

## Corrupted Download (.part.mkv Files)

When a download is interrupted (Ctrl+C, network error, crash), you may see `.part.mkv` files in your output directory.

### Safe to delete

These are incomplete downloads — they are **safe to delete**:

```bash
# Find leftover partial files
find ~/Anime -name "*.part.mkv" -ls

# Remove them
find ~/Anime -name "*.part.mkv" -delete
```

### Resume downloads

Orphion does not resume partial downloads. Re-run the download command:

```bash
orphion download --title "Frieren" --episodes 1-4
```

Episodes that already completed (`.mkv` files exist) will be **skipped** unless you use `--force`:

```bash
orphion download --title "Frieren" --episodes 1-4 --force
```

---

## Subtitle Provider Not Configured

**Error:** `subtitle provider not configured`

Orphion needs at least one subtitle provider to be available. This is set up automatically in most installations.

### Check your setup

1. Ensure Orphion was built with subtitle providers (they are included by default)
2. Verify with:
   ```bash
   orphion subtitles "Frieren"
   ```

### Subtitle language

By default, Orphion searches for English subtitles. To change the language:

```bash
orphion subtitles --lang spanish "Frieren"
```

Or set it in config:

```yaml
# ~/.config/orphion/config.yaml
subtitle_lang: spanish
```

---

## Interactive Prompts Show Garbled Text

**Cause:** Terminal encoding issues or unsupported terminal emulator.

### Workarounds

- Use a modern terminal emulator (iTerm2, Alacritty, Kitty, Windows Terminal)
- Set `TERM=xterm-256color` or `TERM=xterm-utf8`
- On macOS, ensure "Set locale environment variables on startup" is enabled in Terminal preferences

### Non-interactive mode

If interactive prompts don't work, use CLI flags instead:

```bash
orphion download --title "Frieren" --episodes 1-4 --quality 1080p
```

---

## Provider-Specific Issues

### AllAnime

- **Slow responses**: The AllAnime API can be slow under load. Try again in a few minutes.
- **Empty episode lists**: Some titles may not have episodes available yet. Check back later.
- **Stream resolution failures**: Try a different quality (e.g., `720p` instead of `1080p`).

### Bettermelon

- **Segment download failures**: Bettermelon downloads HLS segments before running FFmpeg. If some segments fail, the download will be retried up to 5 times. Persistent failures usually mean the CDN is having issues.
- **"resolving stream" hangs**: Bettermelon needs to resolve the stream via an upstream provider (default: hianime). If the upstream is down, this step will time out. Try again later.
- **Quality limitations**: Available qualities depend on what the upstream provider offers.

### SubDL (subtitles)

- **Missing subtitles for new episodes**: SubDL relies on community uploads. New episodes may not have subtitles yet.
- **Season navigation**: Some shows require selecting a specific season. Orphion automatically tries the first non-specials season.

### Kitsunekko (subtitles)

- **Slow search**: Kitsunekko returns all entries and Orphion filters client-side. The first search may take a few seconds.
- **No search API**: Kitsunekko doesn't have a search endpoint — Orphion fetches the full directory listing and filters locally using fuzzy matching.

### Jimaku (subtitles)

- **Japanese-only focus**: Jimaku primarily hosts Japanese subtitles. For English subtitles, use SubDL or Kitsunekko.
- **Home page caching**: Jimaku's entry listing is cached for the session. If new entries appear, restart Orphion.

---

## Still Having Issues?

- [Open an issue](https://github.com/bibimoni/orphion/issues) on GitHub
- Include the error message, your OS, and the command you ran
- Check [Providers](providers.md) for provider-specific documentation
