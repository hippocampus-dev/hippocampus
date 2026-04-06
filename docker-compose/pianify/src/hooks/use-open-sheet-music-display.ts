import { useEffect, useRef } from "react";
import { OpenSheetMusicDisplay } from "opensheetmusicdisplay";

interface MeasurePosition {
  x: number;
}

export interface MeasureLayout {
  positions: MeasurePosition[];
  endX: number;
}

interface UseOpenSheetMusicDisplayOptions {
  musicXml: string;
}

interface UseOpenSheetMusicDisplayReturn {
  containerRef: React.RefObject<HTMLDivElement | null>;
  measureLayoutRef: React.RefObject<MeasureLayout>;
}

export function useOpenSheetMusicDisplay({
  musicXml,
}: UseOpenSheetMusicDisplayOptions): UseOpenSheetMusicDisplayReturn {
  const containerRef = useRef<HTMLDivElement>(null);
  const measureLayoutRef = useRef<MeasureLayout>({ positions: [], endX: 0 });

  useEffect(() => {
    if (!containerRef.current || !musicXml) return;

    let cancelled = false;

    containerRef.current.innerHTML = "";
    containerRef.current.style.width = "30000px";

    const osmd = new OpenSheetMusicDisplay(containerRef.current, {
      autoResize: false,
      backend: "svg",
      drawTitle: false,
      drawComposer: false,
      drawPartNames: true,
      renderSingleHorizontalStaffline: true,
    });
    osmd.EngravingRules.FixedMeasureWidth = true;
    osmd.EngravingRules.SheetMaximumWidth = 999999;

    osmd
      .load(musicXml)
      .then(() => {
        if (cancelled) return;

        osmd.render();

        if (!containerRef.current) return;

        const svg = containerRef.current.querySelector("svg");
        if (svg) {
          containerRef.current.style.width = `${svg.getBoundingClientRect().width}px`;
        }

        const containerLeft = containerRef.current.getBoundingClientRect().left;
        const cursor = osmd.cursor;
        cursor.show();
        cursor.reset();

        const positions: MeasurePosition[] = [];
        let lastMeasure = -1;

        while (!cursor.iterator.EndReached) {
          const measure = cursor.iterator.CurrentMeasureIndex;
          if (measure !== lastMeasure) {
            const element = cursor.cursorElement;
            if (element) {
              positions.push({
                x: element.getBoundingClientRect().left - containerLeft,
              });
            }
            lastMeasure = measure;
          }
          cursor.next();
        }

        const endX = svg
          ? svg.getBoundingClientRect().width
          : positions.length > 0
            ? positions[positions.length - 1].x
            : 0;

        measureLayoutRef.current = { positions, endX };
        cursor.hide();
      })
      .catch((error) => {
        if (!cancelled) {
          console.error("Failed to load MusicXML:", error);
        }
      });

    return () => {
      cancelled = true;
      measureLayoutRef.current = { positions: [], endX: 0 };
      if (containerRef.current) {
        containerRef.current.innerHTML = "";
      }
    };
  }, [musicXml]);

  return { containerRef, measureLayoutRef };
}
