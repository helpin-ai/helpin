import { describe, expect, it } from 'vitest';

import type { TranscriptSegment } from '@/components/agents/transcript';
import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';
import { groupAdjacentDockTools } from '../dockTranscriptGrouping';

function toolCall(id: string, repoAlias: string): CodingSessionLiveToolCall {
  return {
    tool_call_id: id,
    tool_name: 'list_directory',
    args_text: JSON.stringify({ path: '', repo_alias: repoAlias }),
    status: 'completed',
  };
}

function toolSegment(id: string, repoAlias: string): TranscriptSegment {
  return { kind: 'tool', id, toolCall: toolCall(id, repoAlias) };
}

describe('groupAdjacentDockTools', () => {
  it('does not collapse calls made against different repositories', () => {
    const entries = groupAdjacentDockTools([
      toolSegment('events', 'events-pipeline'),
      toolSegment('website', 'website'),
    ]);

    expect(entries).toHaveLength(2);
    expect(entries.every((entry) => entry.toolGroup == null)).toBe(true);
  });

  it('still groups repeated calls against the same repository', () => {
    const entries = groupAdjacentDockTools([
      toolSegment('events-1', 'events-pipeline'),
      toolSegment('events-2', 'events-pipeline'),
    ]);

    expect(entries).toHaveLength(1);
    expect(entries[0].toolGroup?.count).toBe(2);
  });
});
