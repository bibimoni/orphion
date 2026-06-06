import type { SubtitleCue } from "./subtitles";
import { createPlayer } from "./player";
import type { HarnessPlayer, PlayerVariant } from "./player";
import { parseSrt } from "./subtitles";

export function mountHarness(
  app: HTMLElement,
  initialCues: SubtitleCue[]
): void {
  app.innerHTML = `
    <header>
      <h1>Orphion Migaku Compatibility Harness</h1>
      <p>This disposable page tests which browser subtitle arrangement Migaku can mine.</p>
    </header>

    <section class="controls" aria-label="Harness controls">
      <fieldset>
        <legend>Media source</legend>
        <button type="button" data-source="/sample.mp4">MP4</button>
        <button type="button" data-source="/hls/master.m3u8">HLS</button>
      </fieldset>

      <fieldset>
        <legend>Subtitle arrangement</legend>
        <button type="button" data-variant="track">Native track</button>
        <button type="button" data-variant="dom">Selectable DOM</button>
        <button type="button" data-variant="combined">Combined</button>
      </fieldset>

      <label>
        Upload SRT
        <input type="file" accept=".srt">
      </label>

      <label>
        Subtitle offset (milliseconds)
        <input type="number" value="0" step="100" data-offset>
      </label>

      <button type="button" data-reset>Reset fixture</button>
      <output data-status aria-live="polite">Ready</output>
    </section>

    <section data-player-host aria-label="Compatibility player"></section>
  `;

  const playerHost = app.querySelector<HTMLElement>("[data-player-host]");
  const fileInput = app.querySelector<HTMLInputElement>('input[type="file"]');
  const offsetInput = app.querySelector<HTMLInputElement>("[data-offset]");
  const status = app.querySelector<HTMLOutputElement>("[data-status]");

  if (!playerHost || !fileInput || !offsetInput || !status) {
    throw new Error("Harness controls failed to initialize");
  }

  let sourceUrl = "/sample.mp4";
  let variant: PlayerVariant = "track";
  let cues = [...initialCues];
  let player: HarnessPlayer | undefined;

  const renderPlayer = () => {
    player?.destroy();
    player = createPlayer(sourceUrl, cues, variant);
    player.setOffsetMs(Number(offsetInput.value));
    playerHost.replaceChildren(player.root);
    status.value = `${variant} with ${sourceUrl.endsWith(".m3u8") ? "HLS" : "MP4"}; ${cues.length} cues`;
  };

  app.querySelectorAll<HTMLButtonElement>("[data-source]").forEach((button) => {
    button.addEventListener("click", () => {
      sourceUrl = button.dataset.source ?? "/sample.mp4";
      renderPlayer();
    });
  });

  app.querySelectorAll<HTMLButtonElement>("[data-variant]").forEach((button) => {
    button.addEventListener("click", () => {
      variant = (button.dataset.variant ?? "track") as PlayerVariant;
      renderPlayer();
    });
  });

  fileInput.addEventListener("change", async () => {
    const file = fileInput.files?.[0];
    if (!file) {
      return;
    }

    try {
      cues = parseSrt(await file.text());
      renderPlayer();
    } catch (error) {
      status.value = error instanceof Error ? error.message : "Unable to parse SRT";
    }
  });

  offsetInput.addEventListener("input", () => {
    player?.setOffsetMs(Number(offsetInput.value));
  });

  app.querySelector<HTMLButtonElement>("[data-reset]")?.addEventListener("click", () => {
    sourceUrl = "/sample.mp4";
    variant = "track";
    cues = [...initialCues];
    offsetInput.value = "0";
    fileInput.value = "";
    renderPlayer();
  });

  renderPlayer();
}
