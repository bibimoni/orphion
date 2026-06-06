import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createPlayer } from "../src/player";
import type { SubtitleCue } from "../src/subtitles";

const cues: SubtitleCue[] = [
  { index: 1, startMs: 500, endMs: 3000, text: "First cue" },
  { index: 2, startMs: 3500, endMs: 6000, text: "Second cue" }
];

describe("createPlayer", () => {
  beforeEach(() => {
    vi.stubGlobal("URL", {
      createObjectURL: vi.fn(() => "blob:subtitle"),
      revokeObjectURL: vi.fn()
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  });

  it.each([
    ["track", true, false],
    ["dom", false, true],
    ["combined", true, true]
  ] as const)(
    "creates the %s variant with the expected accessible DOM",
    (variant, hasTrack, hasDomCue) => {
      const player = createPlayer("/sample.mp4", cues, variant);
      document.body.append(player.root);

      expect(player.root.querySelectorAll("video")).toHaveLength(1);
      expect(player.root.querySelector("iframe")).toBeNull();
      expect(Boolean(player.root.querySelector('track[kind="subtitles"]')))
        .toBe(hasTrack);
      expect(Boolean(player.root.querySelector('[aria-live="polite"]')))
        .toBe(hasDomCue);

      player.destroy();
    }
  );

  it("updates selectable DOM cue text when playback time changes", () => {
    const player = createPlayer("/sample.mp4", cues, "dom");
    const display = player.root.querySelector<HTMLElement>("[data-current-cue]");

    Object.defineProperty(player.video, "currentTime", {
      configurable: true,
      value: 4,
      writable: true
    });
    player.video.dispatchEvent(new Event("timeupdate"));

    expect(display?.textContent).toBe("Second cue");
    expect(display?.dataset.cueIndex).toBe("2");

    player.destroy();
  });

  it("applies offset without mutating source cues", () => {
    const original = structuredClone(cues);
    const player = createPlayer("/sample.mp4", cues, "dom");
    const display = player.root.querySelector<HTMLElement>("[data-current-cue]");

    Object.defineProperty(player.video, "currentTime", {
      configurable: true,
      value: 3.25,
      writable: true
    });
    player.setOffsetMs(500);
    player.video.dispatchEvent(new Event("timeupdate"));

    expect(display?.textContent).toBe("First cue");
    expect(cues).toEqual(original);

    player.destroy();
  });

  it("revokes the generated subtitle track URL during cleanup", () => {
    const player = createPlayer("/sample.mp4", cues, "track");

    player.destroy();

    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:subtitle");
  });
});
