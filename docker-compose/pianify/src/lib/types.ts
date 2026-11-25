export type Articulation =
  | "none"
  | "staccato"
  | "staccatissimo"
  | "accent"
  | "tenuto";

export interface NoteEvent {
  pitch: string;
  alter: number;
  octave: number;
  midiNote: number;
  partId: string;
  voice: number;
  staff: number;
  measure: number;
  beat: number;
  durationBeats: number;
  startMs: number;
  durationMs: number;
  tied: boolean;
  articulation: Articulation;
  velocity: number;
}

export interface TimeSignatureChange {
  measure: number;
  beats: number;
  beatType: number;
}

export interface TempoChange {
  measure: number;
  beat: number;
  bpm: number;
  startMs: number;
}

export interface MeasureTiming {
  measure: number;
  startMs: number;
}

export interface PartInfo {
  id: string;
  name: string;
}

export interface ScoreMetadata {
  title: string;
  composer: string;
  parts: PartInfo[];
  timeSignatures: TimeSignatureChange[];
  tempoChanges: TempoChange[];
  measureTimings: MeasureTiming[];
  totalDurationMs: number;
  totalMeasures: number;
}

export interface ParsedScore {
  metadata: ScoreMetadata;
  notes: NoteEvent[];
}

export type HandSelection = "left" | "right" | "both";

export type PlaybackStatus = "stopped" | "playing" | "paused";
