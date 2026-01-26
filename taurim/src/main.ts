import { h, render } from 'preact';
import { App } from './components/App';
import { setupVoiceListeners } from './services/voiceService';
import { startVoiceRecognition } from './state/voiceState';
import { isAndroid } from './services/timerService';

render(h(App, {}), document.getElementById('app')!);

if (isAndroid()) {
  setupVoiceListeners()
    .then(() => startVoiceRecognition())
    .catch((error) => console.error('Failed to initialize voice recognition:', error));
}
