import { defineEventHandler, readBody, createError } from "h3";
import { getRunState, updateRunState, createRunAbortController } from "@/lib/run-state";
import { queuePrompt, getComfyUIWebSocketUrl } from "@/lib/comfyui";
import {
  convertUIToAPI,
  applySeedControls,
  autoDetectSeedControls,
  isUIWorkflow,
  isAPIPrompt,
} from "@/lib/workflow-converter";
import type { SeedControl } from "@/lib/workflow-converter";
import { generateAndInjectPrompts } from "@/lib/prompt-generator";
import { broadcast } from "../ws";
import WebSocket from "ws";

export default defineEventHandler(async (event) => {
  const currentState = getRunState();
  if (currentState.status === "running") {
    throw createError({
      statusCode: 409,
      message: "Already running",
    });
  }

  const body = await readBody<{
    workflow?: unknown;
    mode?: string;
    count?: number;
    workflowName?: string;
    concept?: string;
    topologyHash?: string;
  }>(event);
  if (!body?.workflow) {
    throw createError({ statusCode: 400, message: "workflow is required" });
  }

  let prompt: Record<string, unknown>;
  let seedControls: SeedControl[] = [];
  if (isAPIPrompt(body.workflow)) {
    prompt = body.workflow;
  } else if (isUIWorkflow(body.workflow)) {
    const result = await convertUIToAPI(body.workflow);
    prompt = result.prompt;
    seedControls = result.seedControls;
  } else {
    throw createError({
      statusCode: 400,
      message: "Invalid workflow format",
    });
  }

  if (seedControls.length === 0) {
    seedControls = autoDetectSeedControls(prompt);
  }

  const mode = body.mode === "count" ? "count" : "infinite";
  const targetCount = mode === "count" ? Math.max(1, Number(body.count) || 1) : 0;

  updateRunState({
    status: "running",
    mode,
    targetCount,
    completedCount: 0,
    workflowName: body.workflowName || "",
    progress: null,
    errors: [],
    currentPromptId: null,
  });
  broadcast({ type: "run_state", data: getRunState() });

  const controller = createRunAbortController();

  const concept = body.concept || "";
  const topologyHash = body.topologyHash || "";

  runLoop(prompt, seedControls, concept, topologyHash, controller.signal).catch((error) => {
    if (!controller.signal.aborted) {
      updateRunState({
        status: "idle",
        errors: [
          ...getRunState().errors,
          error instanceof Error ? error.message : String(error),
        ],
      });
      broadcast({ type: "run_state", data: getRunState() });
    }
  });

  return { ok: true };
});

function connectComfyUI(): WebSocket {
  return new WebSocket(getComfyUIWebSocketUrl());
}

async function runLoop(
  prompt: Record<string, unknown>,
  seedControls: SeedControl[],
  concept: string,
  topologyHash: string,
  signal: AbortSignal,
) {
  let ws = connectComfyUI();
  await waitForOpen(ws, signal);

  try {
    while (true) {
      if (signal.aborted) break;
      const state = getRunState();
      if (state.status === "stopping") break;
      if (state.mode === "count" && state.completedCount >= state.targetCount)
        break;

      if (ws.readyState !== WebSocket.OPEN) {
        ws.terminate();
        ws = connectComfyUI();
        await waitForOpen(ws, signal);
      }

      try {
        let iterationPrompt = prompt;
        if (concept) {
          try {
            iterationPrompt = await generateAndInjectPrompts(prompt, concept, topologyHash);
          } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            broadcast({ type: "error", data: { message: `Prompt generation failed: ${message}` } });
          }
        }
        const effectivePrompt = applySeedControls(iterationPrompt, seedControls);
        const { prompt_id } = await queuePrompt(effectivePrompt);
        updateRunState({ currentPromptId: prompt_id, progress: null });
        broadcast({ type: "run_state", data: getRunState() });

        await waitForCompletion(ws, prompt_id, signal);

        const completedCount = getRunState().completedCount + 1;
        updateRunState({
          completedCount,
          currentPromptId: null,
          progress: null,
        });
        broadcast({
          type: "completed",
          data: { promptId: prompt_id, completedCount },
        });
        broadcast({ type: "run_state", data: getRunState() });
      } catch (error) {
        if (signal.aborted) break;
        const message =
          error instanceof Error ? error.message : String(error);
        updateRunState({
          errors: [...getRunState().errors, message],
        });
        broadcast({ type: "error", data: { message } });
        broadcast({ type: "run_state", data: getRunState() });
        await new Promise((resolve) => {
          const timer = setTimeout(resolve, 3000);
          signal.addEventListener("abort", () => {
            clearTimeout(timer);
            resolve(undefined);
          }, { once: true });
        });
      }
    }
  } finally {
    ws.terminate();
  }

  updateRunState({ status: "idle", currentPromptId: null, progress: null });
  broadcast({ type: "run_state", data: getRunState() });
}

function waitForOpen(ws: WebSocket, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const onAbort = () => {
      cleanup();
      reject(new Error("Aborted"));
    };

    const onOpen = () => {
      cleanup();
      resolve();
    };

    const onError = (error: Error) => {
      cleanup();
      reject(error);
    };

    function cleanup() {
      signal.removeEventListener("abort", onAbort);
      ws.off("open", onOpen);
      ws.off("error", onError);
    }

    signal.addEventListener("abort", onAbort, { once: true });
    ws.on("open", onOpen);
    ws.on("error", onError);
  });
}

function waitForCompletion(
  ws: WebSocket,
  promptId: string,
  signal: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup();
      reject(new Error("Timeout waiting for prompt completion"));
    }, 600_000);

    const onAbort = () => {
      cleanup();
      reject(new Error("Aborted"));
    };

    const onMessage = (data: WebSocket.Data) => {
      try {
        const message = JSON.parse(data.toString());

        if (
          message.type === "progress" &&
          message.data?.prompt_id === promptId
        ) {
          const { value, max } = message.data;
          updateRunState({ progress: { current: value, total: max } });
          broadcast({
            type: "progress",
            data: { promptId, current: value, total: max },
          });
        }

        if (
          message.type === "executing" &&
          message.data?.prompt_id === promptId &&
          message.data?.node === null
        ) {
          cleanup();
          resolve();
        }

        if (
          message.type === "execution_success" &&
          message.data?.prompt_id === promptId
        ) {
          cleanup();
          resolve();
        }

        if (
          message.type === "execution_error" &&
          message.data?.prompt_id === promptId
        ) {
          cleanup();
          reject(
            new Error(
              message.data?.exception_message ||
                "ComfyUI execution error",
            ),
          );
        }
      } catch {}
    };

    const onError = (error: Error) => {
      cleanup();
      reject(error);
    };

    const onClose = () => {
      cleanup();
      reject(new Error("ComfyUI WebSocket closed unexpectedly"));
    };

    function cleanup() {
      clearTimeout(timeout);
      signal.removeEventListener("abort", onAbort);
      ws.off("message", onMessage);
      ws.off("error", onError);
      ws.off("close", onClose);
    }

    signal.addEventListener("abort", onAbort, { once: true });
    ws.on("message", onMessage);
    ws.on("error", onError);
    ws.on("close", onClose);
  });
}
