import type { PromptText } from "@/lib/types";
import { extractPromptTexts, getOutputFiles } from "./comfyui";
import { loadLabels } from "./labels";
import { generatePromptTexts } from "./openai";

interface GoodExample {
  prompts: PromptText[];
  reason: string;
}

async function getGoodExamples(topologyHash: string): Promise<GoodExample[]> {
  const [files, labels] = await Promise.all([getOutputFiles(), loadLabels()]);

  const goodLabels = new Map(
    labels
      .filter((label) => label.label === "good")
      .map((label) => [label.filename, label.reason || ""]),
  );

  const seen = new Set<string>();
  const examples: GoodExample[] = [];

  for (const file of files) {
    if (file.topologyHash !== topologyHash) continue;
    if (!goodLabels.has(file.filename)) continue;
    if (!file.prompts?.length) continue;

    const key = file.promptId ?? file.filename;
    if (seen.has(key)) continue;
    seen.add(key);

    examples.push({
      prompts: file.prompts,
      reason: goodLabels.get(file.filename) || "",
    });
  }

  return examples;
}

export async function generateAndInjectPrompts(
  prompt: Record<string, unknown>,
  concept: string,
  topologyHash: string,
  signal?: AbortSignal,
): Promise<Record<string, unknown>> {
  const textNodes = extractPromptTexts(prompt).map((promptText) => ({
    nodeId: promptText.nodeId,
    inputName: promptText.inputName,
    currentText: promptText.text,
  }));

  if (textNodes.length === 0) return prompt;

  const goodExamples = await getGoodExamples(topologyHash);
  const generated = await generatePromptTexts(
    concept,
    textNodes,
    goodExamples,
    signal,
  );

  if (Object.keys(generated).length === 0) return prompt;

  const allowedKeys = new Set(
    textNodes.map((textNode) => `${textNode.nodeId}:${textNode.inputName}`),
  );
  const result = structuredClone(prompt);
  for (const [key, text] of Object.entries(generated)) {
    if (!allowedKeys.has(key)) continue;
    const [nodeId, inputName] = key.split(":");
    const node = result[nodeId] as
      | { inputs?: Record<string, unknown> }
      | undefined;
    if (node?.inputs && inputName in node.inputs) {
      node.inputs[inputName] = text;
    }
  }

  return result;
}
