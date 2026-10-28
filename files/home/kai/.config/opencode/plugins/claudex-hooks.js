// Bridges opencode plugin events to the shared Claude Code hook scripts in ~/.config/claudex/config/hooks/, so the hook logic stays single-sourced. opencode events are translated into the Claude Code stdin JSON contract, piped to the matching script, and the exit code / stdout decision is mapped back to opencode semantics.

import { spawn } from "node:child_process";
import path from "node:path";

const HOOKS_DIR = `${process.env.HOME}/.config/claudex/config/hooks`;

// opencode lowercases tool names; the hook scripts expect Claude Code names.
const PRE_TOOL_NAME = { shell: "Bash" };
const POST_TOOL_NAME = { write: "Write", edit: "Edit" };

export default {
  id: "claudex-hooks",
  async setup(ctx) {
    // Mirrors Stop.sh's stop_hook_active: block once per stop cycle.
    let stopHookActive = false;

    const runHook = (script, payload) => {
      return new Promise((resolve, reject) => {
        // The shared opencode service chdirs to $HOME (packages/cli/src/server-process.ts in anomalyco/opencode v2).
        const child = spawn("bash", [`${HOOKS_DIR}/${script}`], {
          cwd: ctx.location.directory,
        });
        let stdout = "";
        let stderr = "";
        child.stdout.on("data", (chunk) => {
          stdout += chunk;
        });
        child.stderr.on("data", (chunk) => {
          stderr += chunk;
        });
        child.on("error", reject);
        child.on("close", (exitCode) => resolve({ exitCode, stdout, stderr }));
        child.stdin.end(JSON.stringify(payload));
      });
    };

    await ctx.tool.hook("execute.before", async (event) => {
      const toolName = PRE_TOOL_NAME[event.tool];
      if (!toolName) return;

      const result = await runHook("PreToolUse.sh", {
        tool_name: toolName,
        tool_input: event.input,
      });

      if (result.exitCode === 2) {
        throw new Error(result.stderr.trim() || "Blocked by PreToolUse hook");
      }
    });

    await ctx.tool.hook("execute.after", async (event) => {
      const toolName = POST_TOOL_NAME[event.tool];
      if (!toolName || event.status !== "completed") return;

      await runHook("PostToolUse.sh", {
        tool_name: toolName,
        tool_input: {
          file_path: path.resolve(ctx.location.directory, event.input.path),
        },
      });
    });

    const controller = new AbortController();
    void (async () => {
      for await (const event of ctx.event.subscribe({
        signal: controller.signal,
      })) {
        if (event.type !== "session.execution.succeeded") continue;

        const result = await runHook("Stop.sh", {
          stop_hook_active: stopHookActive,
        });
        const stdout = result.stdout.trim();
        if (!stdout) continue;

        let decision;
        try {
          decision = JSON.parse(stdout);
        } catch {
          continue;
        }

        if (decision.decision === "block" && !stopHookActive) {
          stopHookActive = true;
          await ctx.session.prompt({
            sessionID: event.data.sessionID,
            text: decision.reason,
          });
        } else {
          stopHookActive = false;
        }
      }
    })();

    return () => controller.abort();
  },
};
