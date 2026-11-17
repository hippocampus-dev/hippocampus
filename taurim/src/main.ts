import { effect } from "@preact/signals";
import { requestPermissions } from "@tauri-apps/api/core";
import { h, render } from "preact";
import { App } from "./components/App";
import { isAndroid, setupTimerListeners } from "./services/timerService";
import { setupVoiceListeners } from "./services/voiceService";
import { isAlarmPlaying } from "./state/timerState";
import {
  isListening,
  startVoiceRecognition,
  stopVoiceRecognition,
} from "./state/voiceState";

render(h(App, {}), document.getElementById("app")!);

if (isAndroid()) {
  setupTimerListeners(() => {
    navigator.vibrate?.([200, 100, 200]);
  }).catch((error) => console.error("Failed to setup timer listeners:", error));

  setupVoiceListeners()
    .then(() => requestPermissions("speech"))
    .then(() => {
      effect(() => {
        if (isAlarmPlaying.value) {
          startVoiceRecognition().catch((error) =>
            console.error("Failed to start voice recognition:", error),
          );
        } else if (isListening.peek()) {
          stopVoiceRecognition().catch((error) =>
            console.error("Failed to stop voice recognition:", error),
          );
        }
      });
    })
    .catch((error) =>
      console.error("Failed to initialize voice recognition:", error),
    );
}
