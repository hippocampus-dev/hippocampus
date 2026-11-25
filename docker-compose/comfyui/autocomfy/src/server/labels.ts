import { readdir, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import type { LabelData } from "@/lib/types";

export const LABELS_DIR = resolve(process.cwd(), "data", "labels");

export async function loadLabels(): Promise<LabelData[]> {
  let names: string[];
  try {
    names = await readdir(LABELS_DIR);
  } catch (error) {
    // saveLabel creates the directory, so it is missing until the first label is written
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") {
      console.error("Failed to read labels directory:", error);
    }
    return [];
  }

  const labels: LabelData[] = [];
  for (const name of names.filter((name) => name.endsWith(".label.json"))) {
    try {
      const content = await readFile(resolve(LABELS_DIR, name), "utf-8");
      labels.push(JSON.parse(content));
    } catch (error) {
      console.error(`Failed to read label file ${name}:`, error);
    }
  }
  return labels;
}
