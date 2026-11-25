import { addPluginListener } from "@tauri-apps/api/core";
import {
  advanceToNextTimer,
  isAlarmPlaying,
  resetTimer,
  stopAlarm,
  timerGroup,
} from "../state/timerState";
import {
  setLastTranscript,
  setVoiceError,
  setVoiceState,
  voiceState,
} from "../state/voiceState";
import { isStopAlarmIntent } from "./intentService";
import { stopBackgroundAlarm, stopBackgroundTimer } from "./timerService";

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

let debounceTimer: number | null = null;
const DEBOUNCE_MS = 600;
const STOP_ALARM_PATTERN = /止め|とめ|ストップ|stop/i;

export async function setupVoiceListeners(): Promise<void> {
  await addPluginListener<SpeechResultEvent>(
    "speech",
    "result",
    handleSpeechResult,
  );
  await addPluginListener<SpeechErrorEvent>(
    "speech",
    "error",
    handleSpeechError,
  );
  await addPluginListener<SpeechStateEvent>(
    "speech",
    "state",
    handleSpeechState,
  );

  console.log("Voice listeners registered");
}

async function dismissAlarm(): Promise<void> {
  if (timerGroup.value.status === "completed") {
    stopAlarm();
    await stopBackgroundTimer();
    resetTimer();
  } else {
    stopAlarm();
    await stopBackgroundAlarm();
    advanceToNextTimer();
  }
}

function handleSpeechResult(event: SpeechResultEvent): void {
  if (!isAlarmPlaying.value) return;

  const { transcript, isFinal } = event;
  setLastTranscript(transcript);

  if (debounceTimer !== null) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }

  if (isFinal && STOP_ALARM_PATTERN.test(transcript)) {
    dismissAlarm().catch((error) => {
      console.error("Failed to dismiss alarm by voice:", error);
    });
    return;
  }

  if (isFinal) {
    setVoiceState("processing");
    isStopAlarmIntent(transcript)
      .then((stop) => {
        console.log(
          `Final transcript processed: "${transcript}", stop: ${stop}`,
        );
        if (stop && isAlarmPlaying.value) return dismissAlarm();
      })
      .catch((error) => {
        console.error("Error processing transcript:", error);
      })
      .finally(() => {
        if (voiceState.value === "processing") {
          setVoiceState("listening");
        }
      });
  } else {
    debounceTimer = window.setTimeout(() => {
      debounceTimer = null;
      if (isAlarmPlaying.value && voiceState.value !== "processing") {
        setVoiceState("processing");
        isStopAlarmIntent(transcript)
          .then((stop) => {
            console.log(
              `Debounced partial processed: "${transcript}", stop: ${stop}`,
            );
            if (stop && isAlarmPlaying.value) return dismissAlarm();
          })
          .catch((error) => {
            console.error("Error processing partial transcript:", error);
          })
          .finally(() => {
            if (voiceState.value === "processing") {
              setVoiceState("listening");
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
  if (event.state === "listening") {
    setVoiceState("listening");
  } else if (event.state === "idle") {
    setVoiceState("idle");
  } else if (event.state === "error") {
    setVoiceState("error");
  }
}
