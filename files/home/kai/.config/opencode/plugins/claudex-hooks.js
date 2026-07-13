// Bridges opencode plugin events to the shared Claude Code hook scripts in
// ~/.config/claudex/config/hooks/, so the hook logic stays single-sourced.
// opencode events are translated into the Claude Code stdin JSON contract,
// piped to the matching script, and the exit code / stdout decision is mapped
// back to opencode semantics.

const HOOKS_DIR = `${process.env.HOME}/.config/claudex/config/hooks`;

// opencode lowercases tool names; the hook scripts expect Claude Code names.
const PRE_TOOL_NAME = { bash: "Bash" };
const POST_TOOL_NAME = { write: "Write", edit: "Edit" };

export async function ClaudexHooks({ client, $ }) {
  // Mirrors Stop.sh's stop_hook_active: block once per stop cycle.
  let stopHookActive = false;

  const runHook = async (script, payload) => {
    return await $`bash ${`${HOOKS_DIR}/${script}`} < ${new Response(JSON.stringify(payload))}`
      .nothrow()
      .quiet();
  };

  return {
    "tool.execute.before": async (input, output) => {
      const toolName = PRE_TOOL_NAME[input.tool];
      if (!toolName) return;

      const result = await runHook("PreToolUse.sh", {
        tool_name: toolName,
        tool_input: output.args,
      });

      if (result.exitCode === 2) {
        throw new Error(
          result.stderr.toString().trim() || "Blocked by PreToolUse hook",
        );
      }
    },

    "tool.execute.after": async (input, output) => {
      const toolName = POST_TOOL_NAME[input.tool];
      if (!toolName) return;

      await runHook("PostToolUse.sh", {
        tool_name: toolName,
        tool_input: { file_path: input.args.filePath },
      });
    },

    event: async ({ event }) => {
      if (event.type !== "session.idle") return;

      const sessionID = event.properties?.sessionID;
      if (!sessionID) return;

      const result = await runHook("Stop.sh", {
        stop_hook_active: stopHookActive,
      });
      const stdout = result.stdout.toString().trim();
      if (!stdout) return;

      let decision;
      try {
        decision = JSON.parse(stdout);
      } catch {
        return;
      }

      if (decision.decision === "block" && !stopHookActive) {
        stopHookActive = true;
        await client.session.prompt({
          path: { id: sessionID },
          body: { parts: [{ type: "text", text: decision.reason }] },
        });
      } else {
        stopHookActive = false;
      }
    },
  };
}
