import { useEffect, useRef } from "react";
import {
  type HandColors,
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
  bpm: number;
}

const VISIBLE_BEATS = 2;

function visibleWindowMs(bpm: number): number {
  return (VISIBLE_BEATS / bpm) * 60 * 1000;
}

function noteColor(
  colors: HandColors,
  isRight: boolean,
  isActive: boolean,
  isBlack: boolean,
): string {
  if (isRight) {
    if (isActive)
      return isBlack ? colors.rightHandDarkActive : colors.rightHandActive;
    return isBlack ? colors.rightHandDark : colors.rightHand;
  }
  if (isActive)
    return isBlack ? colors.leftHandDarkActive : colors.leftHandActive;
  return isBlack ? colors.leftHandDark : colors.leftHand;
}

export default function FallingNotes({
  notes,
  currentTimeMs,
  bpm,
}: FallingNotesProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const animationRef = useRef<number | null>(null);
  const colorsRef = useRef<HandColors | null>(null);
  const sizeRef = useRef({ width: 0, height: 0 });

  const notesRef = useRef(notes);
  const currentTimeMsRef = useRef(currentTimeMs);
  const bpmRef = useRef(bpm);
  notesRef.current = notes;
  currentTimeMsRef.current = currentTimeMs;
  bpmRef.current = bpm;

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const context = canvas.getContext("2d");
    if (!context) return;

    function updateCanvasSize() {
      if (!canvas) return;
      const rect = canvas.getBoundingClientRect();
      const devicePixelRatio = window.devicePixelRatio || 1;
      canvas.width = rect.width * devicePixelRatio;
      canvas.height = rect.height * devicePixelRatio;
      sizeRef.current = { width: rect.width, height: rect.height };
      colorsRef.current = resolveHandColors(canvas);
    }

    updateCanvasSize();

    const resizeObserver = new ResizeObserver(updateCanvasSize);
    resizeObserver.observe(canvas);

    const themeObserver = new MutationObserver(() => {
      colorsRef.current = resolveHandColors(canvas);
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    function draw() {
      if (!canvas || !context) return;

      const colors = colorsRef.current;
      if (!colors) return;

      const { width, height } = sizeRef.current;
      const devicePixelRatio = window.devicePixelRatio || 1;
      context.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);

      const time = currentTimeMsRef.current;

      context.clearRect(0, 0, width, height);

      const windowMs = visibleWindowMs(bpmRef.current);
      const windowStart = time;
      const windowEnd = time + windowMs;

      for (const note of notesRef.current) {
        if (note.midiNote < PIANO_LOW || note.midiNote > PIANO_HIGH) continue;

        if (note.startMs > windowEnd) break;
        const noteEnd = note.startMs + effectiveDuration(note);
        if (noteEnd < windowStart) continue;

        const position = KEY_POSITIONS[note.midiNote - PIANO_LOW];
        const x = position.x * width;
        const w = position.w * width - 1;

        const yBottom =
          height - ((note.startMs - windowStart) / windowMs) * height;
        const yTop = height - ((noteEnd - windowStart) / windowMs) * height;
        const h = Math.max(yBottom - yTop, 2);

        const isBlack = isBlackKey(note.midiNote);

        context.fillStyle = noteColor(
          colors,
          note.staff === 1,
          time >= note.startMs && time < noteEnd,
          isBlack,
        );

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
      resizeObserver.disconnect();
      themeObserver.disconnect();
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
