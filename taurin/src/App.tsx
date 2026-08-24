import { h } from "preact";
import { Router } from "preact-router";

import Index from "./pages/Index.tsx";
import Settings from "./pages/Settings.tsx";
import TmuxMonitor from "./pages/TmuxMonitor.tsx";
import Translation from "./pages/Translation.tsx";
import VoiceIndicator from "./pages/VoiceIndicator.tsx";

const App = ({}) => {
  return h(Router, {}, [
    h(Index, { path: "/" }),
    h(Settings, { path: "/settings" }),
    h(Translation, { path: "/translation" }),
    h(VoiceIndicator, { path: "/voice-indicator" }),
    h(TmuxMonitor, { path: "/tmux-monitor" }),
  ]);
};

export default App;
