import { invoke } from "@tauri-apps/api/core";

interface ToolCall {
  tool: string;
}

interface ToolCallsResponse {
  tools: ToolCall[];
}

export async function isStopAlarmIntent(transcript: string): Promise<boolean> {
  const normalized = transcript.trim();

  if (normalized.length === 0) {
    return false;
  }

  try {
    const result = await invoke<ToolCallsResponse>(
      "plugin:gemini|parse_intent",
      { transcript: normalized },
    );
    return result.tools.some((toolCall) => toolCall.tool === "stop_alarm");
  } catch (error) {
    console.error("Intent parsing failed:", error);
    return false;
  }
}
