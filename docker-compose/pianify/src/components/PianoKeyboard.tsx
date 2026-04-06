import { BLACK_KEYS, WHITE_KEY_WIDTH, WHITE_KEYS } from "@/lib/keyboard";
import type { NoteEvent } from "@/lib/types";

interface PianoKeyboardProps {
  activeNotes: NoteEvent[];
}

const BLACK_KEY_WIDTH = WHITE_KEY_WIDTH * 0.6;

export default function PianoKeyboard({ activeNotes }: PianoKeyboardProps) {
  const activeMidiMap = new Map<number, NoteEvent>();
  for (const note of activeNotes) {
    if (!activeMidiMap.has(note.midiNote)) {
      activeMidiMap.set(note.midiNote, note);
    }
  }

  return (
    <svg
      viewBox="0 0 100 14"
      className="w-full rounded-b-lg"
      preserveAspectRatio="none"
      style={{ display: "block" }}
    >
      {WHITE_KEYS.map(({ midiNote, index }) => {
        const active = activeMidiMap.get(midiNote);
        return (
          <rect
            key={midiNote}
            x={index * WHITE_KEY_WIDTH}
            y={0}
            width={WHITE_KEY_WIDTH}
            height={14}
            fill={
              active
                ? active.staff === 1
                  ? "var(--right-hand)"
                  : "var(--left-hand)"
                : "#ffffff"
            }
            stroke="#ccc"
            strokeWidth={0.05}
          />
        );
      })}
      {BLACK_KEYS.map(({ midiNote, whiteIndex }) => {
        const active = activeMidiMap.get(midiNote);
        return (
          <rect
            key={midiNote}
            x={whiteIndex * WHITE_KEY_WIDTH - BLACK_KEY_WIDTH / 2}
            y={0}
            width={BLACK_KEY_WIDTH}
            height={9}
            fill={
              active
                ? active.staff === 1
                  ? "var(--right-hand)"
                  : "var(--left-hand)"
                : "#222"
            }
            stroke="#111"
            strokeWidth={0.03}
            rx={0.15}
          />
        );
      })}
    </svg>
  );
}
