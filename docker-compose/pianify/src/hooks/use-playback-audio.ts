import { useCallback, useEffect, useRef, useState } from "react";
import { AudioEngine } from "@/lib/audio-engine";
import type { NoteEvent, PlaybackStatus } from "@/lib/types";

interface UsePlaybackAudioOptions {
  status: PlaybackStatus;
  currentTimeMs: number;
  activeNotes: NoteEvent[];
}

interface UsePlaybackAudioReturn {
  volume: number;
  muted: boolean;
  setVolume: (volume: number) => void;
  setMuted: (muted: boolean) => void;
  silenceAll: () => void;
}

export function usePlaybackAudio({
  status,
  currentTimeMs,
  activeNotes,
}: UsePlaybackAudioOptions): UsePlaybackAudioReturn {
  const audioEngineRef = useRef<AudioEngine | null>(null);
  const previousActiveNotesRef = useRef<Map<number, number>>(new Map());
  const volumeRef = useRef(0.5);
  const mutedRef = useRef(false);
  const [volume, setVolumeState] = useState(0.5);
  const [muted, setMutedState] = useState(false);

  useEffect(() => {
    if (status !== "playing") return;

    if (audioEngineRef.current === null) {
      audioEngineRef.current = new AudioEngine();
      audioEngineRef.current.setVolume(volumeRef.current);
      audioEngineRef.current.setMuted(mutedRef.current);
    }
    const engine = audioEngineRef.current;

    const currentKeys = new Map<number, number>();
    for (const note of activeNotes) {
      if (!currentKeys.has(note.midiNote)) {
        currentKeys.set(note.midiNote, note.startMs);
      }
    }
    const previous = previousActiveNotesRef.current;

    for (const [midiNote] of previous) {
      if (!currentKeys.has(midiNote)) {
        engine.noteOff(midiNote);
      }
    }

    for (const note of activeNotes) {
      if (currentKeys.get(note.midiNote) !== note.startMs) continue;
      const previousStartMs = previous.get(note.midiNote);
      if (previousStartMs === undefined || previousStartMs !== note.startMs) {
        if (!note.tied || previousStartMs === undefined) {
          engine.noteOn(note.midiNote, note.velocity);
        }
      }
    }

    previousActiveNotesRef.current = currentKeys;
  }, [status, currentTimeMs, activeNotes]);

  const silenceAll = useCallback(() => {
    audioEngineRef.current?.allNotesOff();
    previousActiveNotesRef.current = new Map();
  }, []);

  const setVolume = useCallback((v: number) => {
    volumeRef.current = v;
    setVolumeState(v);
    audioEngineRef.current?.setVolume(v);
  }, []);

  const setMuted = useCallback((m: boolean) => {
    mutedRef.current = m;
    setMutedState(m);
    audioEngineRef.current?.setMuted(m);
  }, []);

  useEffect(() => {
    return () => {
      audioEngineRef.current?.dispose();
      audioEngineRef.current = null;
    };
  }, []);

  return {
    volume,
    muted,
    setVolume,
    setMuted,
    silenceAll,
  };
}
