import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createServerFn } from "@tanstack/react-start";
import type { ComfyUIOutputFile, LabelData, WorkflowInfo } from "@/lib/types";
import { getOutputFiles, getWorkflowData, getWorkflows } from "./comfyui";
import { LABELS_DIR, loadLabels } from "./labels";
import {
  convertUIToAPI,
  isAPIPrompt,
  isUIWorkflow,
} from "./workflow-converter";
import { computeTopologyHash } from "./workflow-hash";

export const fetchWorkflows = createServerFn({ method: "GET" }).handler(
  async (): Promise<WorkflowInfo[]> => {
    const files = await getWorkflows();
    return files
      .filter((filename: string) => filename.endsWith(".json"))
      .map((filename: string) => ({
        name: filename.replace(/\.json$/, ""),
        filename,
      }));
  },
);

export const fetchWorkflowData = createServerFn({ method: "GET" })
  .inputValidator((input: unknown) => {
    const obj = input as Record<string, unknown>;
    if (typeof obj?.filename !== "string") {
      throw new Error("filename is required");
    }
    return { filename: obj.filename };
  })
  .handler(async ({ data }) => getWorkflowData(data.filename));

export const fetchResults = createServerFn({ method: "GET" })
  .inputValidator((input: unknown) => {
    if (input === undefined || input === null) return {};
    const obj = input as Record<string, unknown>;
    return {
      topologyHash:
        typeof obj.topologyHash === "string" ? obj.topologyHash : undefined,
    };
  })
  .handler(async ({ data }): Promise<ComfyUIOutputFile[]> => {
    const files = await getOutputFiles();
    if (!data.topologyHash) return files;
    return files.filter((file) => file.topologyHash === data.topologyHash);
  });

export const fetchWorkflowTopologyHash = createServerFn({ method: "POST" })
  .inputValidator((input: unknown) => {
    const obj = input as Record<string, unknown>;
    if (!obj?.workflow) throw new Error("workflow is required");
    return { workflow: obj.workflow as unknown };
  })
  .handler(async ({ data }): Promise<string | undefined> => {
    const { workflow } = data;
    if (isAPIPrompt(workflow)) {
      return computeTopologyHash(workflow as Record<string, unknown>);
    }
    if (isUIWorkflow(workflow)) {
      const result = await convertUIToAPI(workflow);
      return computeTopologyHash(result.prompt as Record<string, unknown>);
    }
    throw new Error("Invalid workflow format");
  });

export const fetchLabels = createServerFn({ method: "GET" }).handler(
  async (): Promise<LabelData[]> => loadLabels(),
);

export const saveLabel = createServerFn({ method: "POST" })
  .inputValidator((input: unknown) => {
    const obj = input as Record<string, unknown>;
    if (typeof obj?.filename !== "string") {
      throw new Error("filename is required");
    }
    if (obj.label !== "good" && obj.label !== "bad") {
      throw new Error("label must be 'good' or 'bad'");
    }
    if (typeof obj.reason !== "string") {
      throw new Error("reason is required");
    }
    return {
      filename: obj.filename,
      label: obj.label as "good" | "bad",
      reason: obj.reason,
      workflow: typeof obj.workflow === "string" ? obj.workflow : undefined,
      promptId: typeof obj.promptId === "string" ? obj.promptId : undefined,
    };
  })
  .handler(async ({ data }): Promise<LabelData> => {
    const { filename, label, reason, workflow, promptId } = data;

    if (!reason) {
      throw new Error("reason is required");
    }

    const labelPath = resolve(LABELS_DIR, `${filename}.label.json`);
    if (!labelPath.startsWith(`${LABELS_DIR}/`)) {
      throw new Error("Invalid filename");
    }

    const labelData: LabelData = {
      filename,
      label,
      reason,
      workflow: workflow || "",
      promptId: promptId || "",
      labeledAt: new Date().toISOString(),
    };

    await mkdir(LABELS_DIR, { recursive: true });
    await writeFile(labelPath, JSON.stringify(labelData, null, 2), "utf-8");
    return labelData;
  });
