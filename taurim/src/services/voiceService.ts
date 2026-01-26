import { invoke } from '@tauri-apps/api/core';
import { processTranscript } from './intentService';
import {
  setLastTranscript,
  setVoiceState,
  setVoiceError,
  voiceState,
} from '../state/voiceState';

interface SpeechResultEvent {
  transcript: string;
  isFinal: boolean;
}

interface SpeechErrorEvent {
  code: string;
  message: string;
}

interface SpeechStateEvent {
  state: string;
}

interface Channel {
  id: number;
  onmessage: (message: unknown) => void;
}

declare global {
  interface Window {
    __TAURI_INTERNALS__?: {
      transformCallback: (callback: (message: unknown) => void) => number;
    };
  }
}

let debounceTimer: number | null = null;
const DEBOUNCE_MS = 600;

let resultChannel: Channel | null = null;
let errorChannel: Channel | null = null;
let stateChannel: Channel | null = null;

function createChannel(onMessage: (message: unknown) => void): Channel {
  const id = window.__TAURI_INTERNALS__!.transformCallback(onMessage);
  return { id, onmessage: onMessage };
}

async function registerListener(event: string, channel: Channel): Promise<void> {
  await invoke('plugin:speech|register_listener', {
    event,
    handler: { id: channel.id, __tauriChannel: true },
  });
}

export async function setupVoiceListeners(): Promise<void> {
  if (!window.__TAURI_INTERNALS__) {
    console.log('Not running in Tauri environment, skipping voice listeners');
    return;
  }

  resultChannel = createChannel((payload) => {
    const event = payload as SpeechResultEvent;
    handleSpeechResult(event);
  });

  errorChannel = createChannel((payload) => {
    const event = payload as SpeechErrorEvent;
    handleSpeechError(event);
  });

  stateChannel = createChannel((payload) => {
    const event = payload as SpeechStateEvent;
    handleSpeechState(event);
  });

  await registerListener('result', resultChannel);
  await registerListener('error', errorChannel);
  await registerListener('state', stateChannel);

  console.log('Voice listeners registered');
}

function handleSpeechResult(event: SpeechResultEvent): void {
  const { transcript, isFinal } = event;
  setLastTranscript(transcript);

  if (debounceTimer !== null) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }

  if (isFinal) {
    setVoiceState('processing');
    processTranscript(transcript)
      .then((handled) => {
        console.log(`Final transcript processed: "${transcript}", handled: ${handled}`);
      })
      .catch((error) => {
        console.error('Error processing transcript:', error);
      })
      .finally(() => {
        if (voiceState.value === 'processing') {
          setVoiceState('listening');
        }
      });
  } else {
    debounceTimer = window.setTimeout(() => {
      debounceTimer = null;
      if (voiceState.value !== 'processing') {
        setVoiceState('processing');
        processTranscript(transcript)
          .then((handled) => {
            console.log(`Debounced partial processed: "${transcript}", handled: ${handled}`);
          })
          .catch((error) => {
            console.error('Error processing partial transcript:', error);
          })
          .finally(() => {
            if (voiceState.value === 'processing') {
              setVoiceState('listening');
            }
          });
      }
    }, DEBOUNCE_MS);
  }
}

function handleSpeechError(event: SpeechErrorEvent): void {
  console.error(`Speech error: ${event.code} - ${event.message}`);
  setVoiceError(event.message);
}

function handleSpeechState(event: SpeechStateEvent): void {
  console.log(`Speech state changed: ${event.state}`);
  if (event.state === 'listening') {
    setVoiceState('listening');
  } else if (event.state === 'idle') {
    setVoiceState('idle');
  } else if (event.state === 'error') {
    setVoiceState('error');
  }
}
