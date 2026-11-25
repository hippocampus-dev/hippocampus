import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createServerFn } from "@tanstack/react-start";
import { getRequest } from "@tanstack/react-start/server";
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
    const files = await getWorkflows(getRequest().signal);
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
    const record = input as Record<string, unknown>;
    if (typeof record?.filename !== "string") {
      throw new Error("filename is required");
    }
    return { filename: record.filename };
  })
  .handler(async ({ data }) =>
    getWorkflowData(data.filename, getRequest().signal),
  );

export const fetchResults = createServerFn({ method: "GET" })
  .inputValidator((input: unknown) => {
    if (input === undefined || input === null) return {};
    const record = input as Record<string, unknown>;
    return {
      topologyHash:
        typeof record.topologyHash === "string"
          ? record.topologyHash
          : undefined,
    };
  })
  .handler(async ({ data }): Promise<ComfyUIOutputFile[]> => {
    const files = await getOutputFiles(getRequest().signal);
    if (!data.topologyHash) return files;
    return files.filter((file) => file.topologyHash === data.topologyHash);
  });

export const fetchWorkflowTopologyHash = createServerFn({ method: "POST" })
  .inputValidator((input: unknown) => {
    const record = input as Record<string, unknown>;
    if (!record?.workflow) throw new Error("workflow is required");
    return { workflow: record.workflow as unknown };
  })
  .handler(async ({ data }): Promise<string | undefined> => {
    const { workflow } = data;
    if (isAPIPrompt(workflow)) {
      return computeTopologyHash(workflow as Record<string, unknown>);
    }
    if (isUIWorkflow(workflow)) {
      const result = await convertUIToAPI(workflow, getRequest().signal);
      return computeTopologyHash(result.prompt as Record<string, unknown>);
    }
    throw new Error("Invalid workflow format");
  });

export const fetchLabels = createServerFn({ method: "GET" }).handler(
  async (): Promise<LabelData[]> => loadLabels(),
);

export const saveLabel = createServerFn({ method: "POST" })
  .inputValidator((input: unknown) => {
    const record = input as Record<string, unknown>;
    if (typeof record?.filename !== "string") {
      throw new Error("filename is required");
    }
    if (record.label !== "good" && record.label !== "bad") {
      throw new Error("label must be 'good' or 'bad'");
    }
    if (typeof record.reason !== "string") {
      throw new Error("reason is required");
    }
    return {
      filename: record.filename,
      label: record.label as "good" | "bad",
      reason: record.reason,
      workflow:
        typeof record.workflow === "string" ? record.workflow : undefined,
      promptId:
        typeof record.promptId === "string" ? record.promptId : undefined,
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
