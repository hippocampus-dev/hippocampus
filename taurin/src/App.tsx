import { h } from "preact";
import { Router } from "preact-router";

import CreateGitHubRepositories from "./pages/CreateGitHubRepositories.tsx";
import Index from "./pages/Index.tsx";
import Settings from "./pages/Settings.tsx";
import Spotify from "./pages/Spotify.tsx";
import TmuxMonitor from "./pages/TmuxMonitor.tsx";
import Translation from "./pages/Translation.tsx";
import VoiceIndicator from "./pages/VoiceIndicator.tsx";

const App = ({}) => {
  return h(Router, {}, [
    h(Index, { path: "/" }),
    h(Settings, { path: "/settings" }),
    h(Translation, { path: "/translation" }),
    h(CreateGitHubRepositories, { path: "/create-github-repositories" }),
    h(VoiceIndicator, { path: "/voice-indicator" }),
    h(TmuxMonitor, { path: "/tmux-monitor" }),
    h(Spotify, { path: "/spotify" }),
  ]);
};

export default App;
