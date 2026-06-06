import type { SubtitleCue } from "./subtitles";
import { cuesToWebVtt } from "./subtitles";
import type { PlayerVariant } from "./player";

export interface SubtitleElements {
  cueDisplay?: HTMLElement;
  trackUrl?: string;
}

export function addSubtitleElements(
  root: HTMLElement,
  video: HTMLVideoElement,
  cues: SubtitleCue[],
  variant: PlayerVariant
): SubtitleElements {
  const elements: SubtitleElements = {};

  if (variant === "track" || variant === "combined") {
    const trackUrl = URL.createObjectURL(
      new Blob([cuesToWebVtt(cues)], { type: "text/vtt" })
    );
    const track = document.createElement("track");
    track.kind = "subtitles";
    track.label = "Uploaded SRT";
    track.srclang = "und";
    track.src = trackUrl;
    track.default = true;
    video.append(track);
    elements.trackUrl = trackUrl;
  }

  if (variant === "dom" || variant === "combined") {
    const cueDisplay = document.createElement("div");
    cueDisplay.className = "current-cue";
    cueDisplay.dataset.currentCue = "";
    cueDisplay.setAttribute("aria-live", "polite");
    cueDisplay.setAttribute("aria-atomic", "true");
    cueDisplay.tabIndex = 0;
    root.append(cueDisplay);
    elements.cueDisplay = cueDisplay;
  }

  return elements;
}
