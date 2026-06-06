export interface SubtitleCue {
  index: number;
  startMs: number;
  endMs: number;
  text: string;
}

const TIMING_PATTERN =
  /^(\d{2,}):([0-5]\d):([0-5]\d),(\d{3})\s+-->\s+(\d{2,}):([0-5]\d):([0-5]\d),(\d{3})(?:\s+.*)?$/;

function timestampToMs(parts: string[]): number {
  const [hours, minutes, seconds, milliseconds] = parts.map(Number);
  return (((hours * 60 + minutes) * 60 + seconds) * 1000) + milliseconds;
}

function formatTimestamp(valueMs: number): string {
  const totalSeconds = Math.floor(valueMs / 1000);
  const milliseconds = valueMs % 1000;
  const seconds = totalSeconds % 60;
  const totalMinutes = Math.floor(totalSeconds / 60);
  const minutes = totalMinutes % 60;
  const hours = Math.floor(totalMinutes / 60);

  return [
    hours.toString().padStart(2, "0"),
    minutes.toString().padStart(2, "0"),
    seconds.toString().padStart(2, "0")
  ].join(":") + `.${milliseconds.toString().padStart(3, "0")}`;
}

export function parseSrt(input: string): SubtitleCue[] {
  const normalized = input.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n").trim();

  if (!normalized) {
    return [];
  }

  return normalized.split(/\n{2,}/).map((block, blockIndex) => {
    const lines = block.split("\n");
    const hasIndex = /^\d+$/.test(lines[0]?.trim() ?? "");
    const timingLineIndex = hasIndex ? 1 : 0;
    const match = TIMING_PATTERN.exec(lines[timingLineIndex]?.trim() ?? "");
    const cueIndex = hasIndex ? Number(lines[0].trim()) : blockIndex + 1;

    if (!match) {
      throw new Error(`Cue ${cueIndex} is missing a valid timing line`);
    }

    const startMs = timestampToMs(match.slice(1, 5));
    const endMs = timestampToMs(match.slice(5, 9));

    if (endMs < startMs) {
      throw new Error(`Cue ${cueIndex} ends before it starts`);
    }

    return {
      index: cueIndex,
      startMs,
      endMs,
      text: lines.slice(timingLineIndex + 1).join("\n").trim()
    };
  });
}

export function cuesToWebVtt(cues: SubtitleCue[]): string {
  const body = cues.map((cue) => [
    cue.index.toString(),
    `${formatTimestamp(cue.startMs)} --> ${formatTimestamp(cue.endMs)}`,
    cue.text
  ].join("\n")).join("\n\n");

  return `WEBVTT\n\n${body}${body ? "\n" : ""}`;
}
