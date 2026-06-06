import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { mountHarness } from "../src/harness";

describe("mountHarness", () => {
  beforeEach(() => {
    vi.stubGlobal("URL", {
      createObjectURL: vi.fn(() => "blob:subtitle"),
      revokeObjectURL: vi.fn()
    });
    document.body.innerHTML = '<main id="app"></main>';
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  });

  it("renders source, variant, upload, offset, and reset controls", () => {
    const app = document.querySelector<HTMLElement>("#app");
    if (!app) {
      throw new Error("Missing app");
    }

    mountHarness(app, []);

    expect(app.querySelectorAll("button[data-variant]")).toHaveLength(3);
    expect(app.querySelectorAll("button[data-source]")).toHaveLength(2);
    expect(app.querySelector('input[type="file"][accept=".srt"]')).not.toBeNull();
    expect(app.querySelector('input[type="number"][data-offset]')).not.toBeNull();
    expect(app.querySelector("button[data-reset]")).not.toBeNull();
    expect(app.querySelector("video")).not.toBeNull();
  });

  it("switches player variants without leaving duplicate video elements", () => {
    const app = document.querySelector<HTMLElement>("#app");
    if (!app) {
      throw new Error("Missing app");
    }

    mountHarness(app, []);
    app.querySelector<HTMLButtonElement>('[data-variant="combined"]')?.click();

    expect(app.querySelectorAll("video")).toHaveLength(1);
    expect(app.querySelector('[data-player-host] [data-variant="combined"]'))
      .not.toBeNull();
  });
});
