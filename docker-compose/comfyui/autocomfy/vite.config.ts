import {defineConfig} from "vite";
import {devtools} from "@tanstack/devtools-vite";
import {tanstackStart} from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import viteTsConfigPaths from "vite-tsconfig-paths";
import tailwindcss from "@tailwindcss/vite";
import {nitro} from "nitro/vite";

// https://vitejs.dev/config/
const config = defineConfig({
    plugins: [
        devtools(),
        nitro({serverDir: "server", features: {websocket: true}}),
        viteTsConfigPaths({
            projects: ["./tsconfig.json"],
        }),
        tailwindcss(),
        tanstackStart(),
        viteReact(),
    ],
});

export default config;
