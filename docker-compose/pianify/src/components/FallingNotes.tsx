import { useEffect, useRef } from "react";
import {
  isBlackKey,
  KEY_POSITIONS,
  PIANO_HIGH,
  PIANO_LOW,
  resolveHandColors,
  toSolfege,
} from "@/lib/keyboard";
import { effectiveDuration } from "@/lib/score-queries";
import type { NoteEvent } from "@/lib/types";

interface FallingNotesProps {
  notes: NoteEvent[];
  currentTimeMs: number;
}

const VISIBLE_WINDOW_MS = 3000;

export default function FallingNotes({
  notes,
  currentTimeMs,
}: FallingNotesProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const animationRef = useRef<number | null>(null);

  const notesRef = useRef(notes);
  const currentTimeMsRef = useRef(currentTimeMs);
  notesRef.current = notes;
  currentTimeMsRef.current = currentTimeMs;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const context = canvas.getContext("2d");
    if (!context) return;

    function draw() {
      if (!canvas || !context) return;

      const colors = resolveHandColors(canvas);

      const rect = canvas.getBoundingClientRect();
      const devicePixelRatio = window.devicePixelRatio || 1;
      canvas.width = rect.width * devicePixelRatio;
      canvas.height = rect.height * devicePixelRatio;
      context.scale(devicePixelRatio, devicePixelRatio);

      const width = rect.width;
      const height = rect.height;
      const time = currentTimeMsRef.current;

      context.clearRect(0, 0, width, height);

      const windowStart = time;
      const windowEnd = time + VISIBLE_WINDOW_MS;

      for (const note of notesRef.current) {
        if (note.rest) continue;
        if (note.midiNote < PIANO_LOW || note.midiNote > PIANO_HIGH) continue;

        const noteEnd = note.startMs + effectiveDuration(note);
        if (noteEnd < windowStart || note.startMs > windowEnd) continue;

        const position = KEY_POSITIONS[note.midiNote - PIANO_LOW];
        const x = position.x * width;
        const w = position.w * width - 1;

        const yBottom =
          height - ((note.startMs - windowStart) / VISIBLE_WINDOW_MS) * height;
        const yTop =
          height - ((noteEnd - windowStart) / VISIBLE_WINDOW_MS) * height;
        const h = Math.max(yBottom - yTop, 2);

        const isRight = note.staff === 1;
        const isActive = time >= note.startMs && time < noteEnd;
        const isBlack = isBlackKey(note.midiNote);

        if (isRight) {
          context.fillStyle = isActive
            ? isBlack
              ? colors.rightHandDarkActive
              : colors.rightHandActive
            : isBlack
              ? colors.rightHandDark
              : colors.rightHand;
        } else {
          context.fillStyle = isActive
            ? isBlack
              ? colors.leftHandDarkActive
              : colors.leftHandActive
            : isBlack
              ? colors.leftHandDark
              : colors.leftHand;
        }

        context.beginPath();
        context.roundRect(x, yTop, w, h, 2);
        context.fill();

        if (isBlack) {
          context.strokeStyle = "rgba(0, 0, 0, 0.3)";
          context.lineWidth = 1;
          context.stroke();
        }

        if (note.articulation === "accent") {
          context.fillStyle = "rgba(255, 255, 255, 0.7)";
          context.beginPath();
          context.moveTo(x + 2, yBottom - 2);
          context.lineTo(x + w / 2, yBottom - 6);
          context.lineTo(x + w - 2, yBottom - 2);
          context.fill();
        }

        const fontSize = Math.min(w * 0.7, 11);
        if (fontSize >= 5) {
          const label = toSolfege(note.pitch, note.alter);
          context.font = `${fontSize}px sans-serif`;
          context.fillStyle = "rgba(255, 255, 255, 0.85)";
          context.textAlign = "center";
          context.textBaseline = "top";
          const labelY = Math.max(yTop + 2, yBottom - fontSize - 2);
          context.fillText(label, x + w / 2, labelY, w);
        }
      }

      animationRef.current = requestAnimationFrame(draw);
    }

    animationRef.current = requestAnimationFrame(draw);

    return () => {
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="w-full h-48 bg-gray-900 rounded-t-lg"
      style={{ display: "block" }}
    />
  );
}
