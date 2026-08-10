import { readdir, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createServerFn } from "@tanstack/react-start";

const SCORES_DIR =
  process.env.SCORES_DIR || resolve(process.cwd(), "public", "scores");

export const fetchScoreList = createServerFn({ method: "GET" }).handler(
  async (): Promise<string[]> => {
    try {
      const files = await readdir(SCORES_DIR);
      return files.filter((f) => f.endsWith(".musicxml") || f.endsWith(".xml"));
    } catch {
      return [];
    }
  },
);

export const fetchScoreXml = createServerFn({ method: "POST" })
  .inputValidator((input: unknown) => {
    const obj = input as Record<string, unknown>;
    if (typeof obj?.filename !== "string") {
      throw new Error("filename is required");
    }
    return { filename: obj.filename };
  })
  .handler(async ({ data }): Promise<string> => {
    const { filename } = data;
    const filePath = resolve(SCORES_DIR, filename);
    if (!filePath.startsWith(SCORES_DIR + "/")) {
      throw new Error("Invalid filename");
    }
    return readFile(filePath, "utf-8");
  });
