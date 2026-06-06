# Phase 0 Media Fixtures

These deterministic fixtures contain a generated test pattern and a 440 Hz
tone. They contain no third-party media.

## Pinned Generator

```text
jrottenberg/ffmpeg:7.1-alpine
image digest: sha256:8ec1ee1f6a0fcd37c97725827b6b7832795c9596e3439b8da56d7700d61ae778
```

The image currently runs through amd64 emulation on Apple Silicon.

## Generate MP4

```bash
docker run --rm \
  -v "$PWD/phase0/fixtures:/out" \
  jrottenberg/ffmpeg:7.1-alpine \
  -f lavfi -i testsrc2=size=1280x720:rate=30 \
  -f lavfi -i sine=frequency=440:sample_rate=48000 \
  -t 12 \
  -c:v libx264 -pix_fmt yuv420p \
  -c:a aac -b:a 128k \
  -movflags +faststart \
  /out/sample.mp4
```

## Generate HLS

```bash
docker run --rm \
  -v "$PWD/phase0/fixtures:/work" \
  -w /work \
  jrottenberg/ffmpeg:7.1-alpine \
  -i sample.mp4 \
  -c copy \
  -hls_time 6 \
  -hls_list_size 0 \
  -hls_segment_filename "hls/segment-%03d.ts" \
  hls/media.m3u8
```

## SHA-256

```text
fa9b69fe8f7c48e18b075806756bb33518ecda2dfdc2ba3dd0f931a0ec3b1ce3  sample.mp4
7d75093ab69e29a72b277b86e09d5972002adb5c8d531b70325884ec9d02de85  hls/media.m3u8
e7ac8bd7c3847c472d43872faa34b9e80bd075caafa966f5154eb2bd920c8d19  hls/segment-000.ts
e9f97894beddff70aea68476a9dd1ca282ea042a333d871ec5132ab3d5ef4073  hls/segment-001.ts
```
