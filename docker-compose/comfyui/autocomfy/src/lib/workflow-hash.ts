import { createHash } from "node:crypto";

export function computeTopologyHash(
  prompt: Record<string, unknown>,
): string | undefined {
  const topology: string[][] = [];

  for (const [nodeId, nodeData] of Object.entries(prompt).sort(([a], [b]) =>
    a.localeCompare(b, undefined, { numeric: true }),
  )) {
    if (typeof nodeData !== "object" || nodeData === null) continue;
    const { class_type, inputs } = nodeData as {
      class_type?: string;
      inputs?: Record<string, unknown>;
    };
    if (!class_type) continue;

    const connections: string[] = [];
    if (inputs) {
      for (const [inputName, value] of Object.entries(inputs)) {
        if (
          Array.isArray(value) &&
          value.length === 2 &&
          typeof value[0] === "string" &&
          typeof value[1] === "number"
        ) {
          connections.push(`${inputName}:${value[0]}:${value[1]}`);
        }
      }
    }
    connections.sort();
    topology.push([nodeId, class_type, ...connections]);
  }

  if (topology.length === 0) return undefined;

  return createHash("sha256")
    .update(JSON.stringify(topology))
    .digest("hex")
    .slice(0, 16);
}
