import { getObjectInfo } from "./comfyui";

let cachedObjectInfo: Record<string, unknown> | null = null;

async function ensureObjectInfo(): Promise<Record<string, unknown>> {
  if (!cachedObjectInfo) {
    cachedObjectInfo = await getObjectInfo();
  }
  return cachedObjectInfo;
}

interface UIWorkflow {
  nodes: Array<{
    id: number;
    type: string;
    widgets_values?: unknown[];
    inputs?: Array<{ name: string; type: string; link: number | null }>;
  }>;
  links: Array<[number, number, number, number, number, string]>;
}

interface APIPrompt {
  [nodeId: string]: {
    class_type: string;
    inputs: Record<string, unknown>;
  };
}

export interface SeedControl {
  nodeId: string;
  inputName: string;
  mode: string;
}

interface ConvertResult {
  prompt: APIPrompt;
  seedControls: SeedControl[];
}

const UI_ONLY_NODE_TYPES = new Set([
  "PrimitiveNode",
  "Reroute",
  "Note",
  "MarkdownNote",
]);

export async function convertUIToAPI(
  workflow: UIWorkflow,
): Promise<ConvertResult> {
  const objectInfo = await ensureObjectInfo();
  const seedControls: SeedControl[] = [];

  const nodeMap = new Map<number, UIWorkflow["nodes"][number]>();
  for (const node of workflow.nodes) {
    nodeMap.set(node.id, node);
  }

  const linkMap = new Map<
    number,
    { sourceNodeId: number; sourceSlot: number }
  >();
  for (const link of workflow.links) {
    const [linkId, sourceNodeId, sourceSlot] = link;
    linkMap.set(linkId, { sourceNodeId, sourceSlot });
  }

  function resolveLink(
    linkId: number,
  ): { type: "node"; nodeId: number; slot: number } | { type: "value"; value: unknown } | null {
    const visited = new Set<number>();
    let currentLinkId = linkId;
    while (true) {
      if (visited.has(currentLinkId)) return null;
      visited.add(currentLinkId);
      const linkInfo = linkMap.get(currentLinkId);
      if (!linkInfo) return null;
      const sourceNode = nodeMap.get(linkInfo.sourceNodeId);
      if (!sourceNode) return null;

      if (sourceNode.type === "PrimitiveNode") {
        return { type: "value", value: sourceNode.widgets_values?.[0] ?? null };
      }

      if (sourceNode.type === "Reroute") {
        const rerouteInput = sourceNode.inputs?.[0];
        if (!rerouteInput?.link) return null;
        currentLinkId = rerouteInput.link;
        continue;
      }

      return { type: "node", nodeId: linkInfo.sourceNodeId, slot: linkInfo.sourceSlot };
    }
  }

  const result: APIPrompt = {};

  for (const node of workflow.nodes) {
    const nodeId = String(node.id);
    const classType = node.type;

    if (UI_ONLY_NODE_TYPES.has(classType)) continue;

    const nodeInfo = (objectInfo as Record<string, { input?: { required?: Record<string, unknown>; optional?: Record<string, unknown> } }>)[classType];

    if (!nodeInfo?.input) continue;

    const inputs: Record<string, unknown> = {};
    const requiredInputs = nodeInfo.input.required ?? {};
    const optionalInputs = nodeInfo.input.optional ?? {};
    const allInputDefs = { ...requiredInputs, ...optionalInputs };

    const widgetInputNames: string[] = [];
    const linkedInputNames = new Set<string>();

    if (node.inputs) {
      for (const input of node.inputs) {
        if (input.link !== null) {
          const resolved = resolveLink(input.link);
          if (!resolved) continue;
          if (resolved.type === "value") {
            inputs[input.name] = resolved.value;
          } else {
            linkedInputNames.add(input.name);
            inputs[input.name] = [
              String(resolved.nodeId),
              resolved.slot,
            ];
          }
        }
      }
    }

    for (const inputName of Object.keys(allInputDefs)) {
      if (!linkedInputNames.has(inputName)) {
        widgetInputNames.push(inputName);
      }
    }

    if (node.widgets_values) {
      let widgetIndex = 0;
      for (const inputName of widgetInputNames) {
        if (widgetIndex >= node.widgets_values.length) break;
        inputs[inputName] = node.widgets_values[widgetIndex];
        widgetIndex++;

        const inputDef = allInputDefs[inputName] as unknown[];
        if (hasControlAfterGenerate(inputDef)) {
          const mode = String(node.widgets_values[widgetIndex] ?? "fixed");
          if (mode !== "fixed") {
            seedControls.push({ nodeId, inputName, mode });
          }
          widgetIndex++;
        }
      }
    }

    result[nodeId] = {
      class_type: classType,
      inputs,
    };
  }

  return { prompt: result, seedControls };
}

export function applySeedControls(
  prompt: Record<string, unknown>,
  seedControls: SeedControl[],
): Record<string, unknown> {
  if (seedControls.length === 0) return prompt;
  const copy = structuredClone(prompt) as APIPrompt;
  for (const { nodeId, inputName, mode } of seedControls) {
    const node = copy[nodeId];
    if (!node) continue;
    const current = Number(node.inputs[inputName]) || 0;
    if (mode === "randomize") {
      node.inputs[inputName] = Math.floor(Math.random() * 2 ** 53);
    } else if (mode === "increment") {
      node.inputs[inputName] = current + 1;
    } else if (mode === "decrement") {
      node.inputs[inputName] = current - 1;
    }
  }
  return copy;
}

function hasControlAfterGenerate(inputDef: unknown[] | undefined): boolean {
  if (!Array.isArray(inputDef) || inputDef.length < 2) return false;
  const config = inputDef[1];
  return (
    typeof config === "object" &&
    config !== null &&
    "control_after_generate" in config &&
    (config as Record<string, unknown>).control_after_generate === true
  );
}

export function isUIWorkflow(data: unknown): data is UIWorkflow {
  if (typeof data !== "object" || data === null) return false;
  const obj = data as Record<string, unknown>;
  return Array.isArray(obj.nodes) && Array.isArray(obj.links);
}

const SEED_INPUT_NAMES = new Set(["seed", "noise_seed"]);

export function autoDetectSeedControls(
  prompt: Record<string, unknown>,
): SeedControl[] {
  const controls: SeedControl[] = [];
  for (const [nodeId, node] of Object.entries(prompt)) {
    if (typeof node !== "object" || node === null) continue;
    const { inputs } = node as { inputs?: Record<string, unknown> };
    if (!inputs) continue;
    for (const [inputName, value] of Object.entries(inputs)) {
      if (SEED_INPUT_NAMES.has(inputName) && typeof value === "number") {
        controls.push({ nodeId, inputName, mode: "randomize" });
      }
    }
  }
  return controls;
}

export function isAPIPrompt(data: unknown): data is APIPrompt {
  if (typeof data !== "object" || data === null) return false;
  const entries = Object.entries(data as Record<string, unknown>);
  if (entries.length === 0) return false;
  return entries.every(
    ([, value]) =>
      typeof value === "object" &&
      value !== null &&
      "class_type" in value &&
      "inputs" in value,
  );
}
