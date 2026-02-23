import { invoke } from '@tauri-apps/api/core';
import { setRemainingSeconds, completeCurrentTimer, startAlarm } from '../state/timerState';

declare global {
  interface Window {
    __TAURI_INTERNALS__?: unknown;
  }
}

interface TimerTickEvent {
  remaining: number;
}

interface Channel {
  id: number;
  onmessage: (message: unknown) => void;
}

interface TauriInternals {
  transformCallback: (callback: (message: unknown) => void) => number;
}

let tickChannel: Channel | null = null;
let completeChannel: Channel | null = null;

function createChannel(onMessage: (message: unknown) => void): Channel {
  const internals = (window as { __TAURI_INTERNALS__?: TauriInternals }).__TAURI_INTERNALS__;
  const id = internals!.transformCallback(onMessage);
  return { id, onmessage: onMessage };
}

async function registerTimerListener(event: string, channel: Channel): Promise<void> {
  await invoke('plugin:timer|register_listener', {
    event,
    handler: { id: channel.id, __tauriChannel: true },
  });
}

export async function setupTimerListeners(onComplete: () => void): Promise<void> {
  const internals = (window as { __TAURI_INTERNALS__?: TauriInternals }).__TAURI_INTERNALS__;
  if (!internals) {
    console.log('Not running in Tauri environment, skipping timer listeners');
    return;
  }

  tickChannel = createChannel((payload) => {
    const event = payload as TimerTickEvent;
    setRemainingSeconds(event.remaining);
  });

  completeChannel = createChannel(() => {
    const completed = completeCurrentTimer();
    if (completed) {
      startAlarm();
      onComplete();
    }
  });

  await registerTimerListener('tick', tickChannel);
  await registerTimerListener('complete', completeChannel);

  console.log('Timer listeners registered');
}

export async function startBackgroundTimer(durationSeconds: number): Promise<void> {
  try {
    await invoke('plugin:timer|start_timer', { durationSeconds });
  } catch (error) {
    console.error('Failed to start background timer:', error);
    throw error;
  }
}

export async function stopBackgroundTimer(): Promise<void> {
  try {
    await invoke('plugin:timer|stop_timer');
  } catch (error) {
    console.error('Failed to stop background timer:', error);
    throw error;
  }
}

export async function stopBackgroundAlarm(): Promise<void> {
  try {
    await invoke('plugin:timer|stop_alarm');
  } catch (error) {
    console.error('Failed to stop background alarm:', error);
    throw error;
  }
}

export function isAndroid(): boolean {
  return typeof window !== 'undefined' &&
         window.__TAURI_INTERNALS__ !== undefined &&
         navigator.userAgent.toLowerCase().includes('android');
}

let audioContext: AudioContext | null = null;
let gainNode: GainNode | null = null;
let alarmInterval: number | null = null;
const pendingTimeouts = new Set<number>();
const activeOscillators = new Set<OscillatorNode>();

export async function playAlarm(): Promise<void> {
  stopAlarmSound();

  audioContext = new AudioContext();

  if (audioContext.state === 'suspended') {
    await audioContext.resume();
  }

  gainNode = audioContext.createGain();
  gainNode.connect(audioContext.destination);
  gainNode.gain.value = 0;

  const playBeep = () => {
    if (!audioContext || !gainNode) return;

    const osc = audioContext.createOscillator();
    osc.type = 'sine';
    osc.frequency.value = 880;
    osc.connect(gainNode);
    osc.start();
    activeOscillators.add(osc);

    gainNode.gain.setValueAtTime(0.3, audioContext.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.3);

    const timeoutId = window.setTimeout(() => {
      pendingTimeouts.delete(timeoutId);
      if (activeOscillators.has(osc)) {
        try {
          osc.stop();
          osc.disconnect();
        } catch {
          // Already stopped
        }
        activeOscillators.delete(osc);
      }
    }, 300);
    pendingTimeouts.add(timeoutId);
  };

  playBeep();
  alarmInterval = window.setInterval(playBeep, 600);
}

export function stopAlarmSound(): void {
  if (alarmInterval !== null) {
    clearInterval(alarmInterval);
    alarmInterval = null;
  }

  for (const timeoutId of pendingTimeouts) {
    clearTimeout(timeoutId);
  }
  pendingTimeouts.clear();

  for (const osc of activeOscillators) {
    try {
      osc.stop();
      osc.disconnect();
    } catch {
      // Already stopped
    }
  }
  activeOscillators.clear();

  if (gainNode) {
    gainNode.disconnect();
    gainNode = null;
  }

  if (audioContext) {
    audioContext.close().catch(() => {
      // Ignore close errors
    });
    audioContext = null;
  }
}
