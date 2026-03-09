import { readdir, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import {
  extractPromptTexts,
  getOutputFiles,
} from "./comfyui";
import { generatePromptTexts } from "./openai";
import type { LabelData, PromptText } from "./types";

const LABELS_DIR = resolve(process.cwd(), "data", "labels");

interface GoodExample {
  prompts: PromptText[];
  reason: string;
}

async function getGoodExamples(
  topologyHash: string,
): Promise<GoodExample[]> {
  const [files, labels] = await Promise.all([
    getOutputFiles(),
    loadLabels(),
  ]);

  const goodLabels = new Map(
    labels
      .filter((l) => l.label === "good")
      .map((l) => [l.filename, l.reason || ""]),
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

async function loadLabels(): Promise<LabelData[]> {
  try {
    const files = await readdir(LABELS_DIR);
    const labelFiles = files.filter((f) => f.endsWith(".label.json"));

    const labels: LabelData[] = [];
    for (const file of labelFiles) {
      try {
        const content = await readFile(resolve(LABELS_DIR, file), "utf-8");
        labels.push(JSON.parse(content));
      } catch {}
    }
    return labels;
  } catch {
    return [];
  }
}

export async function generateAndInjectPrompts(
  prompt: Record<string, unknown>,
  concept: string,
  topologyHash: string,
): Promise<Record<string, unknown>> {
  const textNodes = extractPromptTexts(prompt).map((t) => ({
    nodeId: t.nodeId,
    inputName: t.inputName,
    currentText: t.text,
  }));

  if (textNodes.length === 0) return prompt;

  const goodExamples = await getGoodExamples(topologyHash);
  const generated = await generatePromptTexts(concept, textNodes, goodExamples);

  if (Object.keys(generated).length === 0) return prompt;

  const allowedKeys = new Set(textNodes.map((n) => `${n.nodeId}:${n.inputName}`));
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
