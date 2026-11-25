import { effect } from "@preact/signals";
import { requestPermissions } from "@tauri-apps/api/core";
import { h, render } from "preact";
import { App } from "./components/App";
import { installUpdate, waitForUpdate } from "./services/autoUpdateService";
import { isAndroid, setupTimerListeners } from "./services/timerService";
import { setupVoiceListeners } from "./services/voiceService";
import { isAlarmPlaying, timerGroup } from "./state/timerState";
import {
  isListening,
  startVoiceRecognition,
  stopVoiceRecognition,
} from "./state/voiceState";

const AUTO_UPDATE_MAX_RETRIES = 60;
const AUTO_UPDATE_RETRY_INTERVAL_MILLISECONDS = 60_000;

render(h(App, {}), document.getElementById("app")!);

if (isAndroid()) {
  (async () => {
    await waitForUpdate();
    for (let retries = 0; ; retries++) {
      // Installing an update kills the app's processes: https://developer.android.com/reference/android/content/pm/PackageInstaller.SessionParams#setDontKillApp(boolean)
      await new Promise<void>((resolve) => {
        effect(function () {
          const status = timerGroup.value.status;
          if (
            status === "running" ||
            status === "paused" ||
            isAlarmPlaying.value
          ) {
            return;
          }
          this.dispose();
          resolve();
        });
      });
      try {
        await installUpdate();
        return;
      } catch (error) {
        if (retries >= AUTO_UPDATE_MAX_RETRIES) {
          throw error;
        }
        await new Promise((resolve) =>
          setTimeout(resolve, AUTO_UPDATE_RETRY_INTERVAL_MILLISECONDS),
        );
      }
    }
  })().catch((error) => console.error("Failed to auto update:", error));
}

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
