import typescript from "@rollup/plugin-typescript";
import {nodeResolve} from "@rollup/plugin-node-resolve";

const extensions = [".ts", ".js"];

export default {
    input: "./src/index.ts",
    output: {
        dir: "dist",
        format: "cjs",
    },
    external: [
        "@actions/core",
        "@actions/github",
        "fs/promises",
    ],
    plugins: [
        typescript({}),
        nodeResolve({
            extensions,
            preferBuiltins: true,
        }),
    ],
}
