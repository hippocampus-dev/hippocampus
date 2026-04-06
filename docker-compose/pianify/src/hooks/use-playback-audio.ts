import { useCallback, useEffect, useRef, useState } from "react";
import { AudioEngine } from "@/lib/audio-engine";
import { effectiveDuration } from "@/lib/score-queries";
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
  const previousActiveNotesRef = useRef<Set<number>>(new Set());
  const [volume, setVolumeState] = useState(0.5);
  const [muted, setMutedState] = useState(false);

  function getAudioEngine(): AudioEngine {
    if (audioEngineRef.current === null) {
      audioEngineRef.current = new AudioEngine();
      audioEngineRef.current.setVolume(volume);
      audioEngineRef.current.setMuted(muted);
    }
    return audioEngineRef.current;
  }

  useEffect(() => {
    if (status !== "playing") return;

    const engine = getAudioEngine();
    const time = currentTimeMs;

    const currentActive = new Map<number, NoteEvent>();
    for (const note of activeNotes) {
      if (!currentActive.has(note.midiNote)) {
        if (time < note.startMs + effectiveDuration(note)) {
          currentActive.set(note.midiNote, note);
        }
      }
    }

    const currentKeys = new Set(currentActive.keys());
    const previous = previousActiveNotesRef.current;

    for (const midiNote of previous) {
      if (!currentKeys.has(midiNote)) {
        engine.noteOff(midiNote);
      }
    }

    for (const [midiNote, note] of currentActive) {
      if (!previous.has(midiNote)) {
        engine.noteOn(midiNote, note.velocity);
      }
    }

    previousActiveNotesRef.current = currentKeys;
  }, [status, currentTimeMs, activeNotes]);

  const silenceAll = useCallback(() => {
    audioEngineRef.current?.allNotesOff();
    previousActiveNotesRef.current = new Set();
  }, []);

  const setVolume = useCallback((v: number) => {
    setVolumeState(v);
    audioEngineRef.current?.setVolume(v);
  }, []);

  const setMuted = useCallback((m: boolean) => {
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
