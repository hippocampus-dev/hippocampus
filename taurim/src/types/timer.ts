export type TimerStatus = 'idle' | 'running' | 'paused' | 'completed';

export interface Timer {
  id: string;
  label: string;
  durationSeconds: number;
  remainingSeconds: number;
  status: TimerStatus;
}

export interface TimerGroup {
  id: string;
  name: string;
  timers: Timer[];
  currentIndex: number;
  status: TimerStatus;
}

export interface SavedGroup {
  id: string;
  name: string;
  durations: number[];
}

export function createTimer(durationSeconds: number, label: string = ''): Timer {
  return {
    id: crypto.randomUUID(),
    label,
    durationSeconds,
    remainingSeconds: durationSeconds,
    status: 'idle',
  };
}

export function createTimerGroup(name: string, timers: Timer[]): TimerGroup {
  return {
    id: crypto.randomUUID(),
    name,
    timers,
    currentIndex: 0,
    status: 'idle',
  };
}

export function formatTime(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
}
