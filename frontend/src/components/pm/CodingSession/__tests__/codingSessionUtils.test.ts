import { describe, expect, it } from 'vitest';

import { codingSessionApprovalStatesByPreviewKey } from '../codingSessionUtils';
import type { CodingSessionEvent } from '@/lib/pmTypes';

function interactionEvent(
  sequenceNo: number,
  payload: CodingSessionEvent['payload'],
): CodingSessionEvent {
  return {
    id: `interaction:${sequenceNo}`,
    session_id: 'session-1',
    run_id: 'run-1',
    sequence_no: sequenceNo,
    timestamp: '2026-05-07T10:00:00Z',
    type: sequenceNo === 1 ? 'interaction.requested' : 'interaction.resolved',
    runtime_kind: 'codex',
    payload,
  };
}

describe('codingSessionApprovalStatesByPreviewKey', () => {
  it('returns the latest resolved approval state for each preview key', () => {
    const states = codingSessionApprovalStatesByPreviewKey([
      interactionEvent(1, {
        interaction_id: 'approval-1',
        interaction_kind: 'approval_request',
        status: 'pending',
        request_schema_version: 'helpin.v1',
        request_payload: {
          preview_panel_key: 'story_plan_doc',
          title: 'Approve planning document',
        },
      }),
      interactionEvent(2, {
        interaction_id: 'approval-1',
        interaction_kind: 'approval_request',
        status: 'resolved',
        request_schema_version: 'helpin.v1',
        request_payload: {
          preview_panel_key: 'story_plan_doc',
          title: 'Approve planning document',
        },
        response_payload: {
          decision: 'request_changes',
          message: 'Split this into two smaller tasks.',
        },
        resolved_at: '2026-05-07T10:05:00Z',
        resolved_by: 'user-1',
      }),
    ]);

    expect(states.get('task_plan_doc')).toMatchObject({
      status: 'changes_requested',
      title: 'Approve planning document',
      previewPanelKey: 'task_plan_doc',
      resolvedAt: '2026-05-07T10:05:00Z',
      resolvedBy: 'user-1',
      note: 'Split this into two smaller tasks.',
    });
  });
});
