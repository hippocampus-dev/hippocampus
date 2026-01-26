import { signal, computed } from '@preact/signals';
import { Timer, TimerGroup, SavedGroup, createTimer, createTimerGroup } from '../types/timer';

const STORAGE_KEY = 'taurim-saved-groups';

export const timerGroup = signal<TimerGroup>(
  createTimerGroup('', [createTimer(60)])
);

export const viewIndex = signal<number>(0);

export const savedGroups = signal<SavedGroup[]>(loadSavedGroups());

export const currentTimer = computed<Timer | null>(() => {
  const group = timerGroup.value;
  if (group.currentIndex >= 0 && group.currentIndex < group.timers.length) {
    return group.timers[group.currentIndex];
  }
  return null;
});

export const isRunning = computed(() =>
  timerGroup.value.status === 'running'
);

export const isAlarmPlaying = signal<boolean>(false);

export function startAlarm(): void {
  isAlarmPlaying.value = true;
}

export function stopAlarm(): void {
  isAlarmPlaying.value = false;
}

export function setViewIndex(index: number): void {
  const group = timerGroup.value;
  if (index >= 0 && index < group.timers.length) {
    viewIndex.value = index;
  }
}

export function addTimer(durationSeconds: number = 60): void {
  const group = timerGroup.value;
  const newTimers = [...group.timers, createTimer(durationSeconds)];
  timerGroup.value = {
    ...group,
    timers: newTimers,
  };
  viewIndex.value = newTimers.length - 1;
}

export function removeTimer(index: number): void {
  const group = timerGroup.value;
  if (group.timers.length <= 1) return;
  const newTimers = group.timers.filter((_: Timer, i: number) => i !== index);
  timerGroup.value = {
    ...group,
    timers: newTimers,
    currentIndex: Math.min(group.currentIndex, newTimers.length - 1),
  };
  viewIndex.value = Math.min(viewIndex.value, newTimers.length - 1);
}

export function updateTimerDuration(index: number, durationSeconds: number): void {
  const group = timerGroup.value;
  const newTimers = group.timers.map((timer: Timer, i: number) =>
    i === index
      ? { ...timer, durationSeconds, remainingSeconds: durationSeconds }
      : timer
  );
  timerGroup.value = { ...group, timers: newTimers };
}

export function startTimer(): void {
  const group = timerGroup.value;
  if (group.status === 'running') return;

  const startIndex = group.status === 'idle' ? viewIndex.value : group.currentIndex;

  const newTimers = group.timers.map((timer: Timer, i: number) =>
    i === startIndex
      ? { ...timer, status: 'running' as const }
      : timer
  );
  timerGroup.value = {
    ...group,
    timers: newTimers,
    currentIndex: startIndex,
    status: 'running',
  };
}

export function pauseTimer(): void {
  const group = timerGroup.value;
  if (group.status !== 'running') return;

  const newTimers = group.timers.map((timer: Timer, i: number) =>
    i === group.currentIndex
      ? { ...timer, status: 'paused' as const }
      : timer
  );
  timerGroup.value = { ...group, timers: newTimers, status: 'paused' };
}

export function resetTimer(): void {
  const group = timerGroup.value;
  const newTimers = group.timers.map((timer: Timer) => ({
    ...timer,
    remainingSeconds: timer.durationSeconds,
    status: 'idle' as const,
  }));
  timerGroup.value = {
    ...group,
    timers: newTimers,
    currentIndex: 0,
    status: 'idle',
  };
  viewIndex.value = 0;
}

export function tick(): boolean {
  const group = timerGroup.value;
  if (group.status !== 'running') return false;

  const currentIdx = group.currentIndex;
  const timer = group.timers[currentIdx];
  if (!timer) return false;

  if (timer.remainingSeconds <= 1) {
    const newTimers = [...group.timers];
    newTimers[currentIdx] = { ...timer, remainingSeconds: 0, status: 'completed' };

    const isLastTimer = currentIdx >= group.timers.length - 1;
    timerGroup.value = {
      ...group,
      timers: newTimers,
      status: isLastTimer ? 'completed' : 'paused',
    };
    return true;
  } else {
    const newTimers = [...group.timers];
    newTimers[currentIdx] = { ...timer, remainingSeconds: timer.remainingSeconds - 1 };
    timerGroup.value = { ...group, timers: newTimers };
    return false;
  }
}

export function advanceToNextTimer(): void {
  const group = timerGroup.value;
  const currentIdx = group.currentIndex;

  if (currentIdx >= group.timers.length - 1) return;

  const nextIdx = currentIdx + 1;
  timerGroup.value = {
    ...group,
    currentIndex: nextIdx,
    status: 'paused',
  };
  viewIndex.value = nextIdx;
}

export function saveCurrentGroup(name: string): void {
  const group = timerGroup.value;
  const newSavedGroup: SavedGroup = {
    id: crypto.randomUUID(),
    name,
    durations: group.timers.map((t: Timer) => t.durationSeconds),
  };
  const newSavedGroups = [...savedGroups.value, newSavedGroup];
  savedGroups.value = newSavedGroups;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(newSavedGroups));
}

export function loadGroup(savedGroup: SavedGroup): void {
  const timers = savedGroup.durations.map((d: number) => createTimer(d));
  timerGroup.value = createTimerGroup(savedGroup.name, timers);
}

export function deleteSavedGroup(id: string): void {
  const newSavedGroups = savedGroups.value.filter((g: SavedGroup) => g.id !== id);
  savedGroups.value = newSavedGroups;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(newSavedGroups));
}

function loadSavedGroups(): SavedGroup[] {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      return JSON.parse(stored) as SavedGroup[];
    }
  } catch {
    // ignore
  }
  return [];
}
