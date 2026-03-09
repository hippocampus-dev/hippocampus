"use client";

import { useEffect, useRef, useCallback } from "react";

function parseLineRange(hash: string): { start: number; end: number } | null {
  if (!hash.startsWith("#L")) return null;

  const match = hash.match(/^#L(\d+)(?:-L?(\d+))?$/);
  if (!match) return null;

  const start = parseInt(match[1], 10);
  const end = match[2] ? parseInt(match[2], 10) : start;

  return { start: Math.min(start, end), end: Math.max(start, end) };
}

export default function CodeBlock({ html }: { html: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const lastClickedLine = useRef<number | null>(null);

  const applyHighlight = useCallback(
    (range: { start: number; end: number } | null) => {
      if (!containerRef.current) return;

      const lines = containerRef.current.querySelectorAll(".line");
      lines.forEach((line) => {
        line.classList.remove("highlighted");
      });

      if (!range) return;

      for (let i = range.start; i <= range.end; i++) {
        const line = containerRef.current.querySelector(`#L${i}`);
        if (line) {
          line.classList.add("highlighted");
        }
      }
    },
    []
  );

  const scrollToLine = useCallback((lineNumber: number) => {
    if (!containerRef.current) return;

    const line = containerRef.current.querySelector(`#L${lineNumber}`);
    if (line) {
      line.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  }, []);

  useEffect(() => {
    const range = parseLineRange(window.location.hash);
    applyHighlight(range);
    if (range) {
      setTimeout(() => scrollToLine(range.start), 100);
    }

    const handleHashChange = () => {
      const newRange = parseLineRange(window.location.hash);
      applyHighlight(newRange);
      if (newRange) {
        scrollToLine(newRange.start);
      }
    };

    window.addEventListener("hashchange", handleHashChange);
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, [applyHighlight, scrollToLine]);

  const handleLineNumberClick = useCallback(
    (event: React.MouseEvent) => {
      const target = event.target as HTMLElement;

      if (!target.classList.contains("line-number")) return;

      event.preventDefault();

      const lineNumber = parseInt(target.getAttribute("data-line") || "0", 10);
      if (!lineNumber) return;

      let newHash: string;

      if (event.shiftKey && lastClickedLine.current !== null) {
        const start = Math.min(lastClickedLine.current, lineNumber);
        const end = Math.max(lastClickedLine.current, lineNumber);
        newHash = `#L${start}-L${end}`;
      } else {
        newHash = `#L${lineNumber}`;
        lastClickedLine.current = lineNumber;
      }

      window.history.pushState(null, "", newHash);
      const range = parseLineRange(newHash);
      applyHighlight(range);
    },
    [applyHighlight]
  );

  return (
    <div
      ref={containerRef}
      className="rounded-lg overflow-hidden border [&_pre]:!p-4 [&_pre]:!m-0 [&_pre]:overflow-x-auto [&_code]:text-sm"
      onClick={handleLineNumberClick}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
