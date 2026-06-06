import Hls from "hls.js";

import type { SubtitleCue } from "./subtitles";
import { addSubtitleElements } from "./variants";

export type PlayerVariant = "track" | "dom" | "combined";

export interface HarnessPlayer {
  root: HTMLElement;
  video: HTMLVideoElement;
  destroy(): void;
  setOffsetMs(offsetMs: number): void;
}

export function createPlayer(
  sourceUrl: string,
  cues: SubtitleCue[],
  variant: PlayerVariant
): HarnessPlayer {
  const root = document.createElement("section");
  root.className = "harness-player";
  root.dataset.variant = variant;

  const video = document.createElement("video");
  video.controls = true;
  video.preload = "metadata";
  video.playsInline = true;
  root.append(video);

  let hls: Hls | undefined;
  const isHls = sourceUrl.toLowerCase().includes(".m3u8");

  if (isHls && !video.canPlayType("application/vnd.apple.mpegurl") && Hls.isSupported()) {
    hls = new Hls();
    hls.loadSource(sourceUrl);
    hls.attachMedia(video);
  } else {
    video.src = sourceUrl;
  }

  const subtitleElements = addSubtitleElements(root, video, cues, variant);
  let offsetMs = 0;

  const updateCueDisplay = () => {
    if (!subtitleElements.cueDisplay) {
      return;
    }

    const playbackMs = (video.currentTime * 1000) - offsetMs;
    const activeCue = cues.find(
      (cue) => playbackMs >= cue.startMs && playbackMs <= cue.endMs
    );

    subtitleElements.cueDisplay.textContent = activeCue?.text ?? "";
    if (activeCue) {
      subtitleElements.cueDisplay.dataset.cueIndex = activeCue.index.toString();
    } else {
      delete subtitleElements.cueDisplay.dataset.cueIndex;
    }
  };

  video.addEventListener("timeupdate", updateCueDisplay);
  video.addEventListener("seeked", updateCueDisplay);

  return {
    root,
    video,
    setOffsetMs(value: number) {
      offsetMs = Number.isFinite(value) ? value : 0;
      updateCueDisplay();
    },
    destroy() {
      video.removeEventListener("timeupdate", updateCueDisplay);
      video.removeEventListener("seeked", updateCueDisplay);
      hls?.destroy();
      if (subtitleElements.trackUrl) {
        URL.revokeObjectURL(subtitleElements.trackUrl);
      }
      video.removeAttribute("src");
      root.remove();
    }
  };
}
