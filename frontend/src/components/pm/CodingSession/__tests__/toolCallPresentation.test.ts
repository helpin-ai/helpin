import { describe, expect, it } from 'vitest';

import { describeToolCall } from '../toolCallPresentation';
import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';

function buildToolCall(overrides: Partial<CodingSessionLiveToolCall> & Pick<CodingSessionLiveToolCall, 'tool_name' | 'args_text'>): CodingSessionLiveToolCall {
  return {
    tool_call_id: overrides.tool_call_id ?? 'tool-1',
    tool_name: overrides.tool_name,
    args_text: overrides.args_text,
    status: overrides.status ?? 'completed',
    parent_message_id: overrides.parent_message_id,
    duration_ms: overrides.duration_ms,
    started_at: overrides.started_at,
    completed_at: overrides.completed_at,
    result: overrides.result,
  };
}

describe('describeToolCall', () => {
  it('formats read_file_range with path and line range', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'read_file_range',
      args_text: JSON.stringify({
        path: 'frontend/src/components/ContentStudioShareModal/ContentStudioShareModal.tsx',
        start_line: 440,
        end_line: 468,
      }),
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Read frontend/src/components/ContentStudioShareModal/ContentStudioShareModal.tsx:440-468',
      secondaryLabel: 'Read File Range',
      chips: ['29 lines'],
    });
  });

  it('formats ripgrep with pattern and path', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'ripgrep',
      args_text: JSON.stringify({
        pattern: 'openShareModal',
        path: 'frontend/src',
      }),
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Search "openShareModal" in frontend/src',
      secondaryLabel: 'Ripgrep',
      chips: [],
    });
  });

  it('formats run_command with command and cwd', () => {
    const presentation = describeToolCall(buildToolCall({
      tool_name: 'run_command',
      args_text: JSON.stringify({
        command: 'rg "openShareModal"',
        cwd: 'frontend/src',
      }),
    }));

    expect(presentation).toEqual({
      primaryLabel: 'Run rg "openShareModal"',
      secondaryLabel: 'Run Command',
      chips: ['frontend/src'],
    });
  });
});
