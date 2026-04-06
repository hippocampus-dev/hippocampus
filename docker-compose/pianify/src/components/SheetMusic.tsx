import { useCallback, useEffect, useRef } from "react";
import { useOpenSheetMusicDisplay } from "@/hooks/use-open-sheet-music-display";

interface SheetMusicProps {
  musicXml: string;
  currentTimeMs: number;
  totalDurationMs: number;
  totalMeasures: number;
}

export default function SheetMusic({
  musicXml,
  currentTimeMs,
  totalDurationMs,
  totalMeasures,
}: SheetMusicProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const lineRef = useRef<HTMLDivElement>(null);

  const { containerRef, measureLayoutRef } = useOpenSheetMusicDisplay({
    musicXml,
  });

  const updateLine = useCallback(() => {
    if (!lineRef.current || !wrapperRef.current) return;

    const { positions, endX } = measureLayoutRef.current;
    if (totalDurationMs <= 0 || positions.length === 0) return;

    const measureDurationMs = totalDurationMs / totalMeasures;
    const currentMeasure = currentTimeMs / measureDurationMs;
    const measureIndex = Math.floor(currentMeasure);
    const fraction = currentMeasure - measureIndex;

    let x: number;
    if (measureIndex >= positions.length) {
      x = endX;
    } else if (measureIndex >= positions.length - 1) {
      const last = positions[positions.length - 1];
      x = last.x + fraction * (endX - last.x);
    } else {
      const a = positions[Math.max(0, measureIndex)];
      const b = positions[Math.min(measureIndex + 1, positions.length - 1)];
      x = a.x + fraction * (b.x - a.x);
    }

    lineRef.current.style.transform = `translateX(${x}px)`;

    const wrapperWidth = wrapperRef.current.clientWidth;
    const scrollTarget = x - wrapperWidth / 3;
    wrapperRef.current.scrollLeft = Math.max(0, scrollTarget);
  }, [currentTimeMs, totalDurationMs, totalMeasures, measureLayoutRef]);

  useEffect(() => {
    updateLine();
  }, [updateLine]);

  return (
    <div
      ref={wrapperRef}
      className="relative w-full overflow-x-auto bg-white rounded-lg border border-border"
    >
      <div className="relative inline-block min-w-full">
        <div ref={containerRef} className="py-2" />
        {totalDurationMs > 0 && (
          <div
            ref={lineRef}
            className="absolute top-0 bottom-0 w-0.5 bg-blue-500/60 pointer-events-none"
            style={{ left: 0 }}
          />
        )}
      </div>
    </div>
  );
}
