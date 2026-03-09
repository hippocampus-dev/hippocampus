import {Router} from "preact-router";
import {h} from "preact";

import Index from "./pages/Index.tsx";
import Settings from "./pages/Settings.tsx";
import Translation from "./pages/Translation.tsx";
import VoiceIndicator from "./pages/VoiceIndicator.tsx";

const App = ({}) => {
    return (
        h(Router, {}, [
            h(Index, {path: "/"}),
            h(Settings, {path: "/settings"}),
            h(Translation, {path: "/translation"}),
            h(VoiceIndicator, {path: "/voice-indicator"}),
        ])
    );
}

export default App;
