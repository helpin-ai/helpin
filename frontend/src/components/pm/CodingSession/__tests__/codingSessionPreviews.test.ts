import { describe, expect, it } from 'vitest';

import { collectCodingSessionPreviews } from '../codingSessionPreviews';
import type { CodingSessionEvent, CodingSessionLiveTurnSegment } from '@/lib/pmTypes';

function buildEvent(overrides: Partial<CodingSessionEvent> & Pick<CodingSessionEvent, 'id' | 'type' | 'sequence_no'>): CodingSessionEvent {
  return {
    id: overrides.id,
    session_id: 'session-1',
    run_id: 'run-1',
    sequence_no: overrides.sequence_no,
    timestamp: overrides.timestamp ?? '2026-03-31T10:00:00Z',
    type: overrides.type,
    runtime_kind: overrides.runtime_kind ?? 'codex',
    payload: overrides.payload ?? {},
    runtime_metadata: overrides.runtime_metadata,
  };
}

describe('collectCodingSessionPreviews', () => {
  it('prefers persisted preview artifacts over older assistant message tool invocations', () => {
    const previews = collectCodingSessionPreviews([
      buildEvent({
        id: 'assistant-message',
        type: 'assistant.message.completed',
        sequence_no: 1,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          content: 'Review the draft in the preview pane.',
          tool_invocations: [
            {
              tool_name: 'publish_preview',
              input: {
                panel_key: 'prd_draft',
                title: 'PRD Draft',
                format: 'markdown',
                content: '# Problem\n\nStale body',
              },
            },
          ],
        },
      }),
      buildEvent({
        id: 'preview-artifact',
        type: 'preview.updated',
        sequence_no: 2,
        payload: {
          content: {
            panel_key: 'prd_draft',
            title: 'PRD Draft',
            format: 'markdown',
            content: '# Problem\n\nArtifact-backed draft',
          },
        },
      }),
    ], []);

    expect(previews.get('prd_draft')).toMatchObject({
      panelKey: 'prd_draft',
      title: 'PRD Draft',
      format: 'markdown',
      content: '# Problem\n\nArtifact-backed draft',
    });
  });

  it('overlays live preview tool calls on top of persisted previews', () => {
    const liveSegments: CodingSessionLiveTurnSegment[] = [{
      segment_id: 'tool-1',
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-1',
        tool_name: 'publish_task_plan_doc',
        args_text: JSON.stringify({
          panel_key: 'task_plan_doc',
          title: 'Task Planning Document',
          format: 'markdown',
          content: '# Flow\n\nUpdated live draft',
        }),
        status: 'running',
      },
    }];

    const previews = collectCodingSessionPreviews([], liveSegments);

    expect(previews.get('task_plan_doc')).toMatchObject({
      panelKey: 'task_plan_doc',
      title: 'Task Planning Document',
      format: 'markdown',
      content: '# Flow\n\nUpdated live draft',
    });
  });

  it('ignores failed live preview tool calls', () => {
    const liveSegments: CodingSessionLiveTurnSegment[] = [{
      segment_id: 'tool-failed',
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-failed',
        tool_name: 'publish_task_plan',
        args_text: JSON.stringify({
          content: {
            open_questions: [{ question: 'Should Kafka tests run on every PR?' }],
            proposed_tasks: [],
          },
        }),
        status: 'failed',
        result: {
          content: 'publish_task_plan content must include a non-empty summary',
          error: 'publish_task_plan content must include a non-empty summary',
        },
      },
    }];

    const previews = collectCodingSessionPreviews([], liveSegments);

    expect(previews.size).toBe(0);
  });
});
