import { useCallback, useEffect, useRef, useState } from "react";
import type { PlaybackStatus } from "@/lib/types";

interface UsePlaybackClockOptions {
  totalDurationMs: number;
}

interface UsePlaybackClockReturn {
  status: PlaybackStatus;
  currentTimeMs: number;
  tempoMultiplier: number;
  play: () => void;
  pause: () => void;
  stop: () => void;
  seek: (timeMs: number) => void;
  setTempoMultiplier: (multiplier: number) => void;
}

export function usePlaybackClock({
  totalDurationMs,
}: UsePlaybackClockOptions): UsePlaybackClockReturn {
  const [status, setStatus] = useState<PlaybackStatus>("stopped");
  const [currentTimeMs, setCurrentTimeMs] = useState(0);
  const [tempoMultiplier, setTempoMultiplier] = useState(1);

  const statusRef = useRef(status);
  const currentTimeMsRef = useRef(currentTimeMs);
  const tempoMultiplierRef = useRef(tempoMultiplier);
  const lastTickTimeRef = useRef<number | null>(null);
  const animationRef = useRef<number | null>(null);

  statusRef.current = status;
  currentTimeMsRef.current = currentTimeMs;
  tempoMultiplierRef.current = tempoMultiplier;

  const tick = useCallback(() => {
    if (statusRef.current !== "playing") return;

    const now = performance.now();

    if (lastTickTimeRef.current !== null) {
      const delta =
        (now - lastTickTimeRef.current) * tempoMultiplierRef.current;
      const newTime = currentTimeMsRef.current + delta;

      if (newTime >= totalDurationMs) {
        setCurrentTimeMs(totalDurationMs);
        setStatus("stopped");
        lastTickTimeRef.current = null;
        animationRef.current = null;
        return;
      }

      setCurrentTimeMs(newTime);
    }

    lastTickTimeRef.current = now;
    animationRef.current = requestAnimationFrame(tick);
  }, [totalDurationMs]);

  const cancelAnimation = useCallback(() => {
    if (animationRef.current !== null) {
      cancelAnimationFrame(animationRef.current);
      animationRef.current = null;
    }
  }, []);

  const play = useCallback(() => {
    if (currentTimeMsRef.current >= totalDurationMs) {
      setCurrentTimeMs(0);
    }
    lastTickTimeRef.current = null;
    setStatus("playing");
  }, [totalDurationMs]);

  const pause = useCallback(() => {
    setStatus("paused");
    lastTickTimeRef.current = null;
    cancelAnimation();
  }, [cancelAnimation]);

  const stop = useCallback(() => {
    setStatus("stopped");
    setCurrentTimeMs(0);
    lastTickTimeRef.current = null;
    cancelAnimation();
  }, [cancelAnimation]);

  const seek = useCallback(
    (timeMs: number) => {
      setCurrentTimeMs(Math.max(0, Math.min(timeMs, totalDurationMs)));
      lastTickTimeRef.current = null;
    },
    [totalDurationMs],
  );

  useEffect(() => {
    if (status === "playing") {
      animationRef.current = requestAnimationFrame(tick);
    }
    return cancelAnimation;
  }, [status, tick, cancelAnimation]);

  return {
    status,
    currentTimeMs,
    tempoMultiplier,
    play,
    pause,
    stop,
    seek,
    setTempoMultiplier,
  };
}
