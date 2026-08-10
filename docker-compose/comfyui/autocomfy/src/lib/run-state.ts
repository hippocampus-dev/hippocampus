import type { RunState } from "./types";

let state: RunState = {
  status: "idle",
  mode: "infinite",
  targetCount: 1,
  completedCount: 0,
  currentPromptId: null,
  workflowName: "",
  progress: null,
  errors: [],
};

let abortController: AbortController | null = null;

export function getRunState(): RunState {
  return { ...state };
}

export function updateRunState(patch: Partial<RunState>): void {
  state = { ...state, ...patch };
}

export function createRunAbortController(): AbortController {
  abortController = new AbortController();
  return abortController;
}

export function abortRun(): void {
  abortController?.abort();
  abortController = null;
}
