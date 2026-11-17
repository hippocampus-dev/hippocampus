import { useEffect, useRef } from "react";
import { useOpenSheetMusicDisplay } from "@/hooks/use-open-sheet-music-display";
import type { MeasureTiming } from "@/lib/types";

interface SheetMusicProps {
  musicXml: string;
  currentTimeMs: number;
  totalDurationMs: number;
  totalMeasures: number;
  measureTimings: MeasureTiming[];
}

export default function SheetMusic({
  musicXml,
  currentTimeMs,
  totalDurationMs,
  totalMeasures,
  measureTimings,
}: SheetMusicProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const lineRef = useRef<HTMLDivElement>(null);
  const currentTimeMsRef = useRef(currentTimeMs);
  currentTimeMsRef.current = currentTimeMs;

  const { containerRef, measureLayoutRef } = useOpenSheetMusicDisplay({
    musicXml,
  });

  useEffect(() => {
    let animationId: number | null = null;

    function updateLine() {
      if (!lineRef.current || !wrapperRef.current) {
        animationId = requestAnimationFrame(updateLine);
        return;
      }

      const time = currentTimeMsRef.current;
      const { positions, endX } = measureLayoutRef.current;

      if (totalDurationMs > 0 && positions.length > 0) {
        let measureIndex = 0;
        let fraction = 0;

        if (measureTimings.length > 0) {
          let low = 0;
          let high = measureTimings.length - 1;
          while (low < high) {
            const mid = (low + high + 1) >>> 1;
            if (measureTimings[mid].startMs <= time) {
              low = mid;
            } else {
              high = mid - 1;
            }
          }
          measureIndex = low;

          const measureStartMs = measureTimings[measureIndex].startMs;
          const measureEndMs =
            measureIndex + 1 < measureTimings.length
              ? measureTimings[measureIndex + 1].startMs
              : totalDurationMs;
          const measureDurationMs = measureEndMs - measureStartMs;
          fraction =
            measureDurationMs > 0
              ? (time - measureStartMs) / measureDurationMs
              : 0;
        } else {
          const measureDurationMs = totalDurationMs / totalMeasures;
          const currentMeasure = time / measureDurationMs;
          measureIndex = Math.floor(currentMeasure);
          fraction = currentMeasure - measureIndex;
        }

        let x: number;
        if (measureIndex >= positions.length) {
          x = endX;
        } else if (measureIndex >= positions.length - 1) {
          const last = positions[positions.length - 1];
          x = last.x + fraction * (endX - last.x);
        } else {
          const a = positions[measureIndex];
          const b = positions[measureIndex + 1];
          x = a.x + fraction * (b.x - a.x);
        }

        lineRef.current.style.transform = `translateX(${x}px)`;

        const wrapperWidth = wrapperRef.current.clientWidth;
        const scrollTarget = x - wrapperWidth / 3;
        wrapperRef.current.scrollLeft = Math.max(0, scrollTarget);
      }

      animationId = requestAnimationFrame(updateLine);
    }

    animationId = requestAnimationFrame(updateLine);

    return () => {
      if (animationId !== null) {
        cancelAnimationFrame(animationId);
      }
    };
  }, [totalDurationMs, totalMeasures, measureTimings, measureLayoutRef]);

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
