export function midiNoteToFrequency(midiNote: number): number {
  return 440 * 2 ** ((midiNote - 69) / 12);
}

interface ActiveNote {
  oscillators: OscillatorNode[];
  intermediateGains: GainNode[];
  gain: GainNode;
}

export class AudioEngine {
  private context: AudioContext | null = null;
  private masterGain: GainNode | null = null;
  private activeNotes = new Map<number, ActiveNote>();
  private _volume = 0.5;
  private _muted = false;

  get volume(): number {
    return this._volume;
  }

  get muted(): boolean {
    return this._muted;
  }

  private ensureContext(): AudioContext {
    if (this.context === null) {
      this.context = new AudioContext();
      this.masterGain = this.context.createGain();
      this.masterGain.gain.value = this._muted ? 0 : this._volume;
      this.masterGain.connect(this.context.destination);
    }
    if (this.context.state === "suspended") {
      this.context.resume();
    }
    return this.context;
  }

  setVolume(volume: number): void {
    this._volume = Math.max(0, Math.min(1, volume));
    if (this.masterGain !== null) {
      this.masterGain.gain.value = this._muted ? 0 : this._volume;
    }
  }

  setMuted(muted: boolean): void {
    this._muted = muted;
    if (this.masterGain !== null) {
      this.masterGain.gain.value = this._muted ? 0 : this._volume;
    }
  }

  noteOn(midiNote: number, velocity = 0.7): void {
    if (this.activeNotes.has(midiNote)) return;

    const context = this.ensureContext();
    if (this.masterGain === null) return;

    const frequency = midiNoteToFrequency(midiNote);
    const now = context.currentTime;
    const peak = 0.3 * velocity;
    const sustain = 0.08 * velocity;

    const noteGain = context.createGain();
    noteGain.gain.setValueAtTime(0, now);
    noteGain.gain.linearRampToValueAtTime(peak, now + 0.01);
    noteGain.gain.exponentialRampToValueAtTime(peak * 0.5, now + 0.1);
    noteGain.gain.exponentialRampToValueAtTime(
      Math.max(sustain, 0.001),
      now + 0.5,
    );
    noteGain.connect(this.masterGain);

    const fundamental = context.createOscillator();
    fundamental.type = "triangle";
    fundamental.frequency.setValueAtTime(frequency, now);
    fundamental.connect(noteGain);
    fundamental.start(now);

    const harmonic = context.createOscillator();
    harmonic.type = "sine";
    harmonic.frequency.setValueAtTime(frequency * 2, now);
    const harmonicGain = context.createGain();
    harmonicGain.gain.setValueAtTime(0.1, now);
    harmonic.connect(harmonicGain);
    harmonicGain.connect(noteGain);
    harmonic.start(now);

    this.activeNotes.set(midiNote, {
      oscillators: [fundamental, harmonic],
      intermediateGains: [harmonicGain],
      gain: noteGain,
    });
  }

  noteOff(midiNote: number): void {
    const active = this.activeNotes.get(midiNote);
    if (!active) return;

    const context = this.context;
    if (context === null) return;

    const now = context.currentTime;
    active.gain.gain.cancelScheduledValues(now);
    active.gain.gain.setValueAtTime(active.gain.gain.value, now);
    active.gain.gain.exponentialRampToValueAtTime(0.001, now + 0.3);

    const oscillators = active.oscillators;
    const intermediateGains = active.intermediateGains;
    setTimeout(() => {
      for (const oscillator of oscillators) {
        try {
          oscillator.stop();
          oscillator.disconnect();
        } catch {}
      }
      for (const node of intermediateGains) {
        node.disconnect();
      }
      active.gain.disconnect();
    }, 400);

    this.activeNotes.delete(midiNote);
  }

  allNotesOff(): void {
    for (const midiNote of this.activeNotes.keys()) {
      this.noteOff(midiNote);
    }
  }

  dispose(): void {
    this.allNotesOff();
    if (this.context !== null) {
      this.context.close();
      this.context = null;
      this.masterGain = null;
    }
  }
}
