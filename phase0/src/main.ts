import "./style.css";

import { mountHarness } from "./harness";
import { parseSrt } from "./subtitles";

const app = document.querySelector<HTMLElement>("#app");

if (!app) {
  throw new Error("Missing #app element");
}

const root = app;

async function start(): Promise<void> {
  const response = await fetch("/sample.srt");
  if (!response.ok) {
    throw new Error(`Unable to load sample subtitles: HTTP ${response.status}`);
  }
  mountHarness(root, parseSrt(await response.text()));
}

void start().catch((error: unknown) => {
  root.textContent = error instanceof Error ? error.message : "Harness startup failed";
});
