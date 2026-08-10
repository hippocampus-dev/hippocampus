import { useMemo } from "react";
import {
  BLACK_KEYS,
  KEY_POSITIONS,
  PIANO_LOW,
  WHITE_KEYS,
} from "@/lib/keyboard";
import type { NoteEvent } from "@/lib/types";

interface PianoKeyboardProps {
  activeNotes: NoteEvent[];
}

const VIEWBOX_SCALE = 100;

function handColor(staff: number): string {
  return staff === 1 ? "var(--right-hand)" : "var(--left-hand)";
}

export default function PianoKeyboard({ activeNotes }: PianoKeyboardProps) {
  const activeMidiMap = useMemo(() => {
    const map = new Map<number, NoteEvent>();
    for (const note of activeNotes) {
      if (!map.has(note.midiNote)) {
        map.set(note.midiNote, note);
      }
    }
    return map;
  }, [activeNotes]);

  return (
    <svg
      viewBox={`0 0 ${VIEWBOX_SCALE} 14`}
      className="w-full rounded-b-lg"
      preserveAspectRatio="none"
      style={{ display: "block" }}
    >
      {WHITE_KEYS.map(({ midiNote }) => {
        const position = KEY_POSITIONS[midiNote - PIANO_LOW];
        const active = activeMidiMap.get(midiNote);
        return (
          <rect
            key={midiNote}
            x={position.x * VIEWBOX_SCALE}
            y={0}
            width={position.w * VIEWBOX_SCALE}
            height={14}
            fill={active ? handColor(active.staff) : "#ffffff"}
            stroke="#ccc"
            strokeWidth={0.05}
          />
        );
      })}
      {BLACK_KEYS.map(({ midiNote }) => {
        const position = KEY_POSITIONS[midiNote - PIANO_LOW];
        const active = activeMidiMap.get(midiNote);
        return (
          <rect
            key={midiNote}
            x={position.x * VIEWBOX_SCALE}
            y={0}
            width={position.w * VIEWBOX_SCALE}
            height={9}
            fill={active ? handColor(active.staff) : "#222"}
            stroke="#111"
            strokeWidth={0.03}
            rx={0.15}
          />
        );
      })}
    </svg>
  );
}
