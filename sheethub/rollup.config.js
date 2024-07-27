import typescript from "@rollup/plugin-typescript";
import { nodeResolve } from "@rollup/plugin-node-resolve";

const extensions = [".ts", ".js"];

export default {
  input: "./src/main.ts",
  output: {
    dir: "dist",
    format: "iife", // Prevent exporting to global
    name: "_", // https://developers.google.com/apps-script/guides/libraries#best_practices
  },
  plugins: [
    typescript({}),
    nodeResolve({
      extensions
    }),
  ],
}
