import typescript from "@rollup/plugin-typescript";
import commonjs from "@rollup/plugin-commonjs";
import {nodeResolve} from "@rollup/plugin-node-resolve";

const extensions = [".ts", ".js"];

export default {
    input: "./src/index.ts",
    output: {
        dir: "dist",
        format: "iife", // Prevent exporting to global
        name: "_", // https://developers.google.com/apps-script/guides/libraries#best_practices
    },
    plugins: [
        commonjs(),
        typescript({}),
        nodeResolve({
            extensions
        }),
    ],
}
