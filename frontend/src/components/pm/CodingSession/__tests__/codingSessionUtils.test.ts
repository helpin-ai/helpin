import { describe, expect, it } from 'vitest';

import {
  codingSessionApprovalStatesByPreviewKey,
  isPersistedCodingSessionEvent,
  maxPersistedCodingSessionSequence,
  sortCodingSessionEvents,
  upsertCodingSessionEvents,
} from '../codingSessionUtils';
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

describe('persisted coding-session event cursors', () => {
  function event(
    id: string,
    sequenceNo: number,
    source?: string,
  ): CodingSessionEvent {
    return {
      id,
      session_id: 'session-1',
      run_id: 'run-1',
      sequence_no: sequenceNo,
      timestamp: '2026-07-10T10:00:00Z',
      type: 'assistant.message.delta',
      runtime_kind: 'native_sdk',
      payload: {},
      runtime_metadata: source ? { source } : undefined,
    };
  }

  it.each([
    ['agent_run_message', 'message persistence'],
    ['agent_run_artifact', 'artifact persistence'],
    ['agent_run_interaction', 'interaction persistence'],
    ['agent_run', 'run persistence'],
  ])('recognizes the %s REST projection source (%s)', (source) => {
    expect(isPersistedCodingSessionEvent(event('custom-id', 12, source))).toBe(true);
  });

  it.each([
    'agent_runtime',
    'native_sdk',
    'temporal_worker',
    'websocket',
  ])('does not treat live source %s as a REST pagination cursor', (source) => {
    expect(isPersistedCodingSessionEvent(event('live-event', 1_700_000_000, source))).toBe(false);
  });

  it.each(['msg:1', 'artifact:1', 'interaction:1', 'run:1'])('keeps persisted id-prefix compatibility for %s', (id) => {
    expect(isPersistedCodingSessionEvent(event(id, 7))).toBe(true);
  });

  it('ignores a much larger runtime sequence when advancing the persisted cursor', () => {
    expect(maxPersistedCodingSessionSequence([
      event('msg:1', 14, 'agent_run_message'),
      event('runtime-delta', 1_700_000_123, 'agent_runtime'),
      event('artifact:1', 15, 'agent_run_artifact'),
    ])).toBe(15);
  });

  it('does not advance the REST cursor for a canonical realtime interaction ID', () => {
    const realtime = event(
      'interaction:approval-1:resolved',
      1_787_341_992_000,
      'agent_run_interaction_realtime',
    );
    realtime.type = 'interaction.resolved';
    realtime.payload = { interaction_id: 'approval-1', status: 'resolved' };

    expect(isPersistedCodingSessionEvent(realtime)).toBe(false);
    expect(maxPersistedCodingSessionSequence([
      event('msg:1', 32, 'agent_run_message'),
      realtime,
    ])).toBe(32);
  });

  it('does not treat a realtime message projection as a REST pagination cursor', () => {
    const realtime = event('msg:live-assistant', 14, 'agent_run_message_realtime');
    realtime.type = 'assistant.message.completed';
    realtime.payload = {
      message_id: 'live-assistant',
      role: 'assistant',
      sequence_no: 14,
    };

    expect(isPersistedCodingSessionEvent(realtime)).toBe(false);
  });
});

describe('coding-session event reconciliation', () => {
  it('uses the canonical projection sequence across persisted messages and interactions', () => {
    const approval = interactionEvent(23, {
      interaction_id: 'approval-1',
      interaction_kind: 'approval_request',
      status: 'resolved',
    });
    approval.runtime_metadata = { source: 'agent_run_interaction' };

    const resumedAssistant: CodingSessionEvent = {
      id: 'msg:assistant-resumed',
      session_id: 'session-1',
      run_id: 'run-1',
      sequence_no: 27,
      timestamp: '2026-08-21T19:52:34Z',
      type: 'assistant.message.completed',
      runtime_kind: 'codex',
      payload: {
        message_id: 'assistant-resumed',
        role: 'assistant',
        sequence_no: 14,
      },
      runtime_metadata: { source: 'agent_run_message' },
    };

    expect(sortCodingSessionEvents([resumedAssistant, approval])).toEqual([
      approval,
      resumedAssistant,
    ]);
  });

  it('retains payload sequence ordering for legacy message events without a projection source', () => {
    const persistedUser: CodingSessionEvent = {
      id: 'msg:user',
      session_id: 'session-1',
      run_id: 'run-1',
      sequence_no: 2,
      timestamp: '2026-05-08T07:00:01Z',
      type: 'user.message.completed',
      runtime_kind: 'codex',
      payload: { message_id: 'user', role: 'user', sequence_no: 2 },
      runtime_metadata: { source: 'agent_run_message' },
    };
    const legacyStatus: CodingSessionEvent = {
      ...persistedUser,
      id: 'legacy-status',
      sequence_no: 1_700_000_001,
      type: 'assistant.message.completed',
      payload: { message_id: 'status', role: 'assistant', sequence_no: 1 },
      runtime_metadata: undefined,
    };

    expect(sortCodingSessionEvents([persistedUser, legacyStatus])).toEqual([
      legacyStatus,
      persistedUser,
    ]);
  });

  it('uses timestamps when realtime interaction and message sequences come from different streams', () => {
    const resolution = interactionEvent(1_787_341_933_639, {
      interaction_id: 'approval-1',
      interaction_kind: 'approval_request',
      status: 'resolved',
    });
    resolution.timestamp = '2026-08-21T19:52:13.639Z';
    resolution.runtime_metadata = { source: 'agent_run_interaction_realtime' };

    const resumedAssistant: CodingSessionEvent = {
      id: 'msg:assistant-resumed',
      session_id: 'session-1',
      run_id: 'run-1',
      sequence_no: 14,
      timestamp: '2026-08-21T19:52:14.008Z',
      type: 'assistant.message.completed',
      runtime_kind: 'codex',
      payload: {
        message_id: 'assistant-resumed',
        role: 'assistant',
        sequence_no: 14,
      },
      runtime_metadata: { source: 'agent_run_message_realtime' },
    };

    expect(sortCodingSessionEvents([resumedAssistant, resolution])).toEqual([
      resolution,
      resumedAssistant,
    ]);
  });

  it('replaces a transient interaction resolution with its persisted projection', () => {
    const payload = {
      interaction_id: 'approval-1',
      interaction_kind: 'approval_request',
      status: 'resolved',
      request_schema_version: 'helpin.v1',
      request_payload: { title: 'Approve task planning document' },
      response_payload: { decision: 'approve' },
      resolved_at: '2026-08-21T19:52:13Z',
    };
    const transient: CodingSessionEvent = {
      id: 'run-1:1787341992000000000',
      session_id: 'run-1',
      run_id: 'run-1',
      sequence_no: 1_787_341_992_000,
      timestamp: '2026-08-21T19:52:13Z',
      type: 'interaction.resolved',
      runtime_kind: 'native_sdk',
      payload,
      runtime_metadata: { source: 'websocket' },
    };
    const persisted: CodingSessionEvent = {
      ...transient,
      id: 'interaction:approval-1:resolved',
      sequence_no: 33,
      runtime_metadata: { source: 'agent_run_interaction' },
    };

    const reconciled = upsertCodingSessionEvents([transient], [persisted]);

    expect(reconciled).toEqual([persisted]);
  });

  it('does not let a delayed realtime copy replace a persisted interaction event', () => {
    const persisted = interactionEvent(4, {
      interaction_id: 'approval-2',
      interaction_kind: 'approval_request',
      status: 'resolved',
      request_schema_version: 'helpin.v1',
      request_payload: { title: 'Approve delivery' },
      response_payload: { decision: 'approve' },
    });
    persisted.id = 'interaction:approval-2:resolved';
    persisted.runtime_metadata = { source: 'agent_run_interaction' };
    const delayedRealtime = {
      ...persisted,
      sequence_no: 1_787_341_992_000,
      runtime_metadata: { source: 'agent_run_interaction_realtime' },
    };

    expect(upsertCodingSessionEvents([persisted], [delayedRealtime])).toEqual([persisted]);
  });
});
