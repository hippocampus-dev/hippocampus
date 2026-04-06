import type { NoteEvent } from "./types";

export function effectiveDuration(note: NoteEvent): number {
  if (note.articulation === "staccatissimo") return note.durationMs * 0.25;
  if (note.articulation === "staccato") return note.durationMs * 0.5;
  return note.durationMs;
}

export function filterNotesByHand(
  notes: NoteEvent[],
  hand: "left" | "right" | "both",
): NoteEvent[] {
  if (hand === "both") return notes;
  if (hand === "right") return notes.filter((n) => n.staff === 1);
  return notes.filter((n) => n.staff === 2);
}

export function getActiveNotes(
  notes: NoteEvent[],
  currentTimeMs: number,
): NoteEvent[] {
  return notes.filter(
    (n) =>
      !n.rest &&
      currentTimeMs >= n.startMs &&
      currentTimeMs < n.startMs + n.durationMs,
  );
}
