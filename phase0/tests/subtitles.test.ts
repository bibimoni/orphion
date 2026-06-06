import { describe, expect, it } from "vitest";

import { cuesToWebVtt, parseSrt } from "../src/subtitles";

describe("parseSrt", () => {
  it("parses CRLF input, optional indexes, multiline text, and UTF-8", () => {
    const input = [
      "1",
      "00:00:00,500 --> 00:00:03,000",
      "最初の字幕です。",
      "",
      "00:00:03,500 --> 00:00:06,000",
      "Second cue,",
      "with two lines."
    ].join("\r\n");

    expect(parseSrt(input)).toEqual([
      {
        index: 1,
        startMs: 500,
        endMs: 3000,
        text: "最初の字幕です。"
      },
      {
        index: 2,
        startMs: 3500,
        endMs: 6000,
        text: "Second cue,\nwith two lines."
      }
    ]);
  });

  it("rejects cues whose end precedes their start", () => {
    const input = "1\n00:00:03,000 --> 00:00:02,000\nInvalid";

    expect(() => parseSrt(input)).toThrow(
      "Cue 1 ends before it starts"
    );
  });

  it("rejects malformed cue blocks", () => {
    expect(() => parseSrt("not a subtitle")).toThrow(
      "Cue 1 is missing a valid timing line"
    );
  });
});

describe("cuesToWebVtt", () => {
  it("emits a valid WebVTT document with period millisecond separators", () => {
    const output = cuesToWebVtt([
      {
        index: 1,
        startMs: 500,
        endMs: 3000,
        text: "Line one\nLine two"
      }
    ]);

    expect(output).toBe(
      "WEBVTT\n\n1\n00:00:00.500 --> 00:00:03.000\nLine one\nLine two\n"
    );
  });
});
