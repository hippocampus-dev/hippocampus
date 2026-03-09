import type { ComfyUIHistoryEntry, ComfyUIOutputFile, PromptText } from "./types";
import { computeTopologyHash } from "./workflow-hash";

const COMFYUI_URL =
  process.env.COMFYUI_URL || "http://comfyui:8188";

export async function queuePrompt(
  prompt: Record<string, unknown>,
): Promise<{ prompt_id: string }> {
  const response = await fetch(`${COMFYUI_URL}/prompt`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ prompt, client_id: "autocomfy" }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(
      `ComfyUI /prompt failed: status=${response.status}, body=${body}`,
    );
  }
  return response.json();
}

const TEXT_INPUT_NAMES = new Set([
  "text",
  "tags",
  "lyrics",
  "prompt",
  "positive",
  "negative",
]);

function isTextEncodeNode(classType: string): boolean {
  const lower = classType.toLowerCase();
  return lower.includes("textencode") || lower.includes("textencod");
}

export function extractPromptTexts(
  promptData: Record<string, unknown>,
): PromptText[] {
  const texts: PromptText[] = [];
  for (const [nodeId, node] of Object.entries(promptData)) {
    if (typeof node !== "object" || node === null) continue;
    const { class_type, inputs } = node as {
      class_type?: string;
      inputs?: Record<string, unknown>;
    };
    if (!class_type || !inputs) continue;
    if (!isTextEncodeNode(class_type)) continue;
    for (const [inputName, value] of Object.entries(inputs)) {
      if (TEXT_INPUT_NAMES.has(inputName) && typeof value === "string" && value.trim()) {
        texts.push({ nodeId, nodeType: class_type, inputName, text: value });
      }
    }
  }
  return texts;
}

export async function getOutputFiles(): Promise<ComfyUIOutputFile[]> {
  const response = await fetch(`${COMFYUI_URL}/history?max_items=500`);
  if (!response.ok) return [];
  const data: Record<string, ComfyUIHistoryEntry> = await response.json();

  const seen = new Set<string>();
  const files: ComfyUIOutputFile[] = [];
  for (const [promptId, entry] of Object.entries(data)) {
    const promptData = entry.prompt?.[2];
    const prompts =
      promptData && typeof promptData === "object"
        ? extractPromptTexts(promptData as Record<string, unknown>)
        : [];
    const topologyHash =
      promptData && typeof promptData === "object"
        ? computeTopologyHash(promptData as Record<string, unknown>)
        : undefined;

    for (const nodeOutput of Object.values(entry.outputs)) {
      for (const list of [nodeOutput.images, nodeOutput.gifs, nodeOutput.audio]) {
        if (!list) continue;
        for (const file of list) {
          if (seen.has(file.filename)) continue;
          seen.add(file.filename);
          files.push({ ...file, promptId, prompts, topologyHash });
        }
      }
    }
  }
  return files;
}

export async function fetchView(
  filename: string,
  subfolder?: string,
  type?: string,
): Promise<Response> {
  const params = new URLSearchParams({ filename });
  if (subfolder) params.set("subfolder", subfolder);
  if (type) params.set("type", type);
  return fetch(`${COMFYUI_URL}/view?${params.toString()}`);
}

export async function getObjectInfo(): Promise<Record<string, unknown>> {
  const response = await fetch(`${COMFYUI_URL}/object_info`);
  if (!response.ok) {
    throw new Error(`ComfyUI /object_info failed: status=${response.status}`);
  }
  return response.json();
}

export async function getWorkflows(): Promise<string[]> {
  const response = await fetch(`${COMFYUI_URL}/api/userdata?dir=workflows`);
  if (!response.ok) return [];
  return response.json();
}

export function getComfyUIWebSocketUrl(): string {
  const url = new URL(COMFYUI_URL);
  const wsProtocol = url.protocol === "https:" ? "wss:" : "ws:";
  return `${wsProtocol}//${url.host}/ws?clientId=autocomfy`;
}
