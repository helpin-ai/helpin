import { describe, expect, it } from 'vitest';

import { deriveLiveStatusLabel } from '../liveStatus';
import type { CodingSessionLiveTurnSegment } from '@/lib/pmTypes';

function toolSegment(
  toolName: string,
  status: 'running' | 'completed' | 'failed',
  argsText = '{}',
): CodingSessionLiveTurnSegment {
  return {
    segment_id: `seg:${toolName}:${status}`,
    kind: 'tool_call',
    tool_call: {
      tool_call_id: `tc:${toolName}`,
      tool_name: toolName,
      args_text: argsText,
      status,
    },
  };
}

describe('deriveLiveStatusLabel', () => {
  it('labels the currently running tool', () => {
    const label = deriveLiveStatusLabel(
      {
        live_turn_segments: [toolSegment('read_file', 'running', '{"path":"Dockerfile"}')],
        live_reasoning_message: null,
      },
      'running',
    );
    expect(label).toContain('Dockerfile');
  });

  it('ignores update_plan and completed tools, falling back to reasoning', () => {
    const label = deriveLiveStatusLabel(
      {
        live_turn_segments: [
          toolSegment('read_file', 'completed'),
          toolSegment('update_plan', 'running'),
        ],
        live_reasoning_message: {
          message_id: 'r1',
          content: 'thinking hard',
          status: 'streaming',
        },
      },
      'running',
    );
    expect(label).toBe('Thinking…');
  });

  it('reports lifecycle phrasing when nothing is live', () => {
    expect(deriveLiveStatusLabel(null, 'queued')).toBe('Starting agent…');
    expect(deriveLiveStatusLabel(null, 'running')).toBe('Thinking…');
    expect(deriveLiveStatusLabel(null, 'paused')).toBeNull();
    expect(deriveLiveStatusLabel(null)).toBeNull();
  });
});
