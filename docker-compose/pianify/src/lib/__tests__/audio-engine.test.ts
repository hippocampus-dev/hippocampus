import { describe, expect, it } from "vitest";
import { midiNoteToFrequency } from "../audio-engine";

describe("midiNoteToFrequency", () => {
  it("converts A4 (MIDI 69) to 440 Hz", () => {
    expect(midiNoteToFrequency(69)).toBeCloseTo(440, 2);
  });

  it("converts C4 (MIDI 60) to ~261.63 Hz", () => {
    expect(midiNoteToFrequency(60)).toBeCloseTo(261.63, 1);
  });

  it("converts A3 (MIDI 57) to 220 Hz", () => {
    expect(midiNoteToFrequency(57)).toBeCloseTo(220, 2);
  });

  it("converts A5 (MIDI 81) to 880 Hz", () => {
    expect(midiNoteToFrequency(81)).toBeCloseTo(880, 2);
  });

  it("converts C1 (MIDI 24) to ~32.70 Hz", () => {
    expect(midiNoteToFrequency(24)).toBeCloseTo(32.7, 1);
  });

  it("converts C8 (MIDI 108) to ~4186.01 Hz", () => {
    expect(midiNoteToFrequency(108)).toBeCloseTo(4186.01, 0);
  });

  it("doubles frequency per octave", () => {
    const c4 = midiNoteToFrequency(60);
    const c5 = midiNoteToFrequency(72);
    expect(c5 / c4).toBeCloseTo(2, 5);
  });
});
