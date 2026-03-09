import { invoke } from '@tauri-apps/api/core';
import {
  savedGroups,
  loadGroup,
  startTimer,
  pauseTimer,
  resetTimer,
  advanceToNextTimer,
} from '../state/timerState';
import { SavedGroup } from '../types/timer';

interface ToolCall {
  tool: string;
  parameters: {
    group_name?: string | null;
  };
}

interface ToolCallsResponse {
  tools: ToolCall[];
}

export async function processTranscript(transcript: string): Promise<boolean> {
  const normalized = transcript.trim();

  if (normalized.length === 0) {
    return false;
  }

  try {
    const groups = savedGroups.value.map((g: SavedGroup) => g.name);
    const result = await invoke<ToolCallsResponse>('plugin:gemini|parse_intent', {
      transcript: normalized,
      availableGroups: groups,
    });

    if (result.tools.length === 0) {
      return false;
    }

    for (const toolCall of result.tools) {
      executeToolCall(toolCall);
    }
    return true;
  } catch (error) {
    console.error('Intent parsing failed:', error);
    return false;
  }
}

function executeToolCall(toolCall: ToolCall): void {
  switch (toolCall.tool) {
    case 'start_timer':
      startTimer();
      break;
    case 'pause_timer':
      pauseTimer();
      break;
    case 'reset_timer':
      resetTimer();
      break;
    case 'next_timer':
      advanceToNextTimer();
      break;
    case 'load_group':
      if (toolCall.parameters.group_name) {
        const group = savedGroups.value.find(
          (g: SavedGroup) => g.name === toolCall.parameters.group_name
        );
        if (group) {
          loadGroup(group);
        }
      }
      break;
  }
}
