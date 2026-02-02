import { h, render } from 'preact';
import { App } from './components/App';
import { setupVoiceListeners } from './services/voiceService';
import { startVoiceRecognition } from './state/voiceState';
import { isAndroid, setupTimerListeners } from './services/timerService';

render(h(App, {}), document.getElementById('app')!);

if (isAndroid()) {
  setupTimerListeners(() => {
    navigator.vibrate?.([200, 100, 200]);
  }).catch((error) => console.error('Failed to setup timer listeners:', error));

  setupVoiceListeners()
    .then(() => startVoiceRecognition())
    .catch((error) => console.error('Failed to initialize voice recognition:', error));
}
