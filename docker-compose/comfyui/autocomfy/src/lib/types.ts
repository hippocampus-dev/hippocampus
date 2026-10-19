export interface RunState {
  status: "idle" | "running" | "stopping";
  mode: "infinite" | "count";
  targetCount: number;
  completedCount: number;
  currentPromptId: string | null;
  workflowName: string;
  progress: { current: number; total: number } | null;
  errors: string[];
}

export interface LabelData {
  filename: string;
  label: "good" | "bad";
  reason: string;
  workflow: string;
  promptId: string;
  labeledAt: string;
}

export interface WorkflowInfo {
  name: string;
  filename: string;
}

export interface ComfyUIHistoryEntry {
  prompt: [number, string, Record<string, unknown>, Record<string, unknown>];
  outputs: Record<
    string,
    {
      images?: ComfyUIOutputFile[];
      gifs?: ComfyUIOutputFile[];
      audio?: ComfyUIOutputFile[];
    }
  >;
  status: { status_str: string; completed: boolean };
}

export interface PromptText {
  nodeId: string;
  nodeType: string;
  inputName: string;
  text: string;
}

export interface ComfyUIOutputFile {
  filename: string;
  subfolder: string;
  type: string;
  promptId?: string;
  topologyHash?: string;
  prompts?: PromptText[];
}

export type ServerMessage =
  | { type: "run_state"; data: RunState }
  | {
      type: "progress";
      data: { promptId: string; current: number; total: number };
    }
  | { type: "completed"; data: { promptId: string; completedCount: number } }
  | { type: "error"; data: { message: string } };
