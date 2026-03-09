import { signal } from '@preact/signals';
import { invoke } from '@tauri-apps/api/core';

export type VoiceStateType = 'idle' | 'listening' | 'processing' | 'error';

export const isListening = signal<boolean>(false);
export const lastTranscript = signal<string>('');
export const voiceError = signal<string | null>(null);
export const voiceState = signal<VoiceStateType>('idle');

export async function startVoiceRecognition(): Promise<void> {
  try {
    await invoke('plugin:speech|start_listening');
    isListening.value = true;
    voiceState.value = 'listening';
    voiceError.value = null;
  } catch (error) {
    console.error('Failed to start voice recognition:', error);
    voiceError.value = error instanceof Error ? error.message : String(error);
    voiceState.value = 'error';
    throw error;
  }
}

export async function stopVoiceRecognition(): Promise<void> {
  try {
    await invoke('plugin:speech|stop_listening');
    isListening.value = false;
    voiceState.value = 'idle';
  } catch (error) {
    console.error('Failed to stop voice recognition:', error);
    throw error;
  }
}

export function setVoiceState(state: VoiceStateType): void {
  voiceState.value = state;
}

export function setLastTranscript(transcript: string): void {
  lastTranscript.value = transcript;
}

export function setVoiceError(error: string | null): void {
  voiceError.value = error;
  if (error) {
    voiceState.value = 'error';
  }
}
