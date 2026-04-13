import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { filterNotesByHand, getActiveNotes } from "@/lib/score-queries";
import type { HandSelection, NoteEvent, PlaybackStatus } from "@/lib/types";
import { usePlaybackAudio } from "./use-playback-audio";
import { usePlaybackClock } from "./use-playback-clock";

interface UsePlaybackOptions {
  notes: NoteEvent[];
  totalDurationMs: number;
}

interface UsePlaybackReturn {
  status: PlaybackStatus;
  currentTimeMs: number;
  tempoMultiplier: number;
  handSelection: HandSelection;
  activeNotes: NoteEvent[];
  filteredNotes: NoteEvent[];
  volume: number;
  muted: boolean;
  checkpointMs: number | null;
  play: () => void;
  pause: () => void;
  stop: () => void;
  seek: (timeMs: number) => void;
  setTempoMultiplier: (multiplier: number) => void;
  setHandSelection: (hand: HandSelection) => void;
  setVolume: (volume: number) => void;
  setMuted: (muted: boolean) => void;
  setCheckpoint: () => void;
  clearCheckpoint: () => void;
  playFromCheckpoint: () => void;
}

export function usePlayback({
  notes,
  totalDurationMs,
}: UsePlaybackOptions): UsePlaybackReturn {
  const [handSelection, setHandSelection] = useState<HandSelection>("both");
  const [checkpointMs, setCheckpointMs] = useState<number | null>(null);

  const filteredNotes = useMemo(
    () => filterNotesByHand(notes, handSelection),
    [notes, handSelection],
  );

  const clock = usePlaybackClock({ totalDurationMs });

  const activeNotes = getActiveNotes(filteredNotes, clock.currentTimeMs);

  const audio = usePlaybackAudio({
    status: clock.status,
    currentTimeMs: clock.currentTimeMs,
    activeNotes,
  });

  const previousStatusRef = useRef(clock.status);
  useEffect(() => {
    const previous = previousStatusRef.current;
    previousStatusRef.current = clock.status;
    if (previous === "playing" && clock.status === "stopped") {
      audio.silenceAll();
    }
  }, [clock.status, audio.silenceAll]);

  const pause = useCallback(() => {
    clock.pause();
    audio.silenceAll();
  }, [clock.pause, audio.silenceAll]);

  const stop = useCallback(() => {
    clock.stop();
    audio.silenceAll();
  }, [clock.stop, audio.silenceAll]);

  const seek = useCallback(
    (timeMs: number) => {
      clock.seek(timeMs);
      audio.silenceAll();
    },
    [clock.seek, audio.silenceAll],
  );

  const setCheckpoint = useCallback(() => {
    setCheckpointMs(clock.currentTimeMs);
  }, [clock.currentTimeMs]);

  const clearCheckpoint = useCallback(() => {
    setCheckpointMs(null);
  }, []);

  const playFromCheckpoint = useCallback(() => {
    if (checkpointMs === null) return;
    clock.seek(checkpointMs);
    audio.silenceAll();
    clock.play();
  }, [checkpointMs, clock.seek, clock.play, audio.silenceAll]);

  useEffect(() => {
    clock.stop();
    audio.silenceAll();
    setCheckpointMs(null);
  }, [notes, totalDurationMs, clock.stop, audio.silenceAll]);

  return {
    status: clock.status,
    currentTimeMs: clock.currentTimeMs,
    tempoMultiplier: clock.tempoMultiplier,
    handSelection,
    activeNotes,
    filteredNotes,
    volume: audio.volume,
    muted: audio.muted,
    checkpointMs,
    play: clock.play,
    pause,
    stop,
    seek,
    setTempoMultiplier: clock.setTempoMultiplier,
    setHandSelection,
    setVolume: audio.setVolume,
    setMuted: audio.setMuted,
    setCheckpoint,
    clearCheckpoint,
    playFromCheckpoint,
  };
}
