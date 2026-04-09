import { describe, expect, it } from 'vitest';

import { buildCodingSessionStreamState } from '../codingSessionStream';
import type { CodingSessionEvent } from '@/lib/pmTypes';

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

describe('buildCodingSessionStreamState', () => {
  it('builds a live assistant turn with attached tool execution and sidecar activity', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'msg-user',
        type: 'user.message.completed',
        sequence_no: 1,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'persisted-user-1',
          content: 'Please update the file.',
          role: 'user',
        },
      }),
      buildEvent({
        id: 'assistant-start',
        type: 'assistant.message.started',
        sequence_no: 1_700_000_001,
        payload: { message_id: 'assistant-live-1' },
      }),
      buildEvent({
        id: 'assistant-delta-1',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_002,
        payload: { message_id: 'assistant-live-1', text: 'Inspecting the workspace.\n' },
      }),
      buildEvent({
        id: 'tool-start',
        type: 'tool.call.started',
        sequence_no: 1_700_000_003,
        payload: {
          parent_message_id: 'assistant-live-1',
          tool_call_id: 'tool-1',
          tool_name: 'read_file',
          args_text: '{"path":"a.go"}',
        },
      }),
      buildEvent({
        id: 'tool-result',
        type: 'tool.call.result',
        sequence_no: 1_700_000_004,
        payload: {
          parent_message_id: 'assistant-live-1',
          tool_call_id: 'tool-1',
          result_message_id: 'tool-result-1',
          content: 'package main',
          output_summary: 'package main',
        },
      }),
      buildEvent({
        id: 'tool-complete',
        type: 'tool.call.completed',
        sequence_no: 1_700_000_005,
        payload: {
          parent_message_id: 'assistant-live-1',
          tool_call_id: 'tool-1',
          duration_ms: 25,
          content: 'package main',
          output_summary: 'package main',
        },
      }),
      buildEvent({
        id: 'assistant-delta-2',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_006,
        payload: { message_id: 'assistant-live-1', text: 'Patched the call site.' },
      }),
      buildEvent({
        id: 'interaction-requested',
        type: 'interaction.requested',
        sequence_no: 1_700_000_007,
        payload: {
          interaction_id: 'interaction-1',
          interaction_kind: 'request_user_input',
          status: 'pending',
        },
      }),
    ]);

    expect(state.transcript_messages).toHaveLength(1);
    expect(state.transcript_messages[0]?.role).toBe('user');
    expect(state.live_assistant_message?.message_id).toBe('assistant-live-1');
    expect(state.live_assistant_message?.content).toBe('Inspecting the workspace.\nPatched the call site.');
    expect(state.live_assistant_message?.tool_calls).toHaveLength(1);
    expect(state.live_assistant_message?.tool_calls[0]).toMatchObject({
      tool_call_id: 'tool-1',
      tool_name: 'read_file',
      status: 'completed',
      duration_ms: 25,
      result: {
        content: 'package main',
      },
    });
    expect(state.live_turn_segments).toHaveLength(3);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        message_id: 'assistant-live-1',
        content: 'Inspecting the workspace.\n',
      },
    });
    expect(state.live_turn_segments[1]).toMatchObject({
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-1',
        tool_name: 'read_file',
      },
    });
    expect(state.live_turn_segments[2]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        message_id: 'assistant-live-1',
        content: 'Patched the call site.',
      },
    });
    expect(state.activity_events.map((event) => event.type)).toEqual(['interaction.requested']);
  });

  it('attaches persisted tool invocations to finalized assistant turns and drops duplicate live completions', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-persisted',
        type: 'assistant.message.completed',
        sequence_no: 2,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-persisted-1',
          content: 'Done',
          role: 'assistant',
          tool_invocations: [
            {
              tool_name: 'write_file',
              input: { path: 'a.go' },
              output_summary: 'Updated a.go',
              duration_ms: 14,
            },
          ],
        },
      }),
      buildEvent({
        id: 'assistant-start',
        type: 'assistant.message.started',
        sequence_no: 1_700_000_001,
        payload: { message_id: 'assistant-live-1' },
      }),
      buildEvent({
        id: 'assistant-delta',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_002,
        payload: { message_id: 'assistant-live-1', text: 'Done' },
      }),
      buildEvent({
        id: 'assistant-complete',
        type: 'assistant.message.completed',
        sequence_no: 1_700_000_003,
        payload: { message_id: 'assistant-live-1', content: 'Done' },
      }),
      buildEvent({
        id: 'tool-message-persisted',
        type: 'tool.call.completed',
        sequence_no: 3,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'tool-msg-1',
          role: 'tool',
          content: 'Updated a.go',
        },
      }),
    ]);

    expect(state.live_assistant_message).toBeNull();
    expect(state.live_turn_segments).toHaveLength(0);
    expect(state.transcript_messages).toHaveLength(1);
    expect(state.transcript_messages[0]?.tool_calls).toHaveLength(1);
    expect(state.transcript_messages[0]?.tool_calls?.[0]).toMatchObject({
      tool_name: 'write_file',
      status: 'completed',
      result: {
        content: 'Updated a.go',
      },
    });
  });

  it('parses persisted assistant turn segments for historical transcript reconstruction', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-persisted-segments',
        type: 'assistant.message.completed',
        sequence_no: 5,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-persisted-2',
          content: 'Inspecting files.\nPatched the call site.',
          turn_segments: [
            {
              segment_id: 'assistant-persisted-2:segment:1',
              kind: 'assistant_message',
              assistant_message: {
                message_id: 'assistant-live-2',
                content: 'Inspecting files.\n',
                status: 'completed',
              },
            },
            {
              segment_id: 'tool-1',
              kind: 'tool_call',
              tool_call: {
                tool_call_id: 'tool-1',
                tool_name: 'read_file',
                status: 'completed',
                args_text: '{"path":"a.go"}',
                result: {
                  content: 'package main',
                },
              },
            },
            {
              segment_id: 'assistant-persisted-2:segment:2',
              kind: 'assistant_message',
              assistant_message: {
                message_id: 'assistant-live-2',
                content: 'Patched the call site.',
                status: 'completed',
              },
            },
          ],
        },
      }),
    ]);

    expect(state.transcript_messages).toHaveLength(1);
    expect(state.transcript_messages[0]?.turn_segments).toHaveLength(3);
    expect(state.transcript_messages[0]?.turn_segments?.[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        content: 'Inspecting files.\n',
      },
    });
    expect(state.transcript_messages[0]?.turn_segments?.[1]).toMatchObject({
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-1',
        tool_name: 'read_file',
      },
    });
    expect(state.transcript_messages[0]?.turn_segments?.[2]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        content: 'Patched the call site.',
      },
    });
  });

  it('hydrates a live turn from the session snapshot and applies future deltas on top', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-delta-later',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_100,
        payload: {
          message_id: 'assistant-live-2',
          text: ' world',
        },
      }),
      buildEvent({
        id: 'tool-complete-later',
        type: 'tool.call.completed',
        sequence_no: 1_700_000_101,
        payload: {
          parent_message_id: 'assistant-live-2',
          tool_call_id: 'tool-2',
          tool_name: 'read_file',
          duration_ms: 12,
          output_summary: 'Read app.tsx',
        },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-live-2',
        content: 'Hello',
        started_at: '2026-03-31T10:00:00Z',
        status: 'streaming',
        tool_calls: [{
          tool_call_id: 'tool-2',
          parent_message_id: 'assistant-live-2',
          tool_name: 'read_file',
          args_text: '{"path":"app.tsx"}',
          status: 'running',
          started_at: '2026-03-31T10:00:01Z',
        }],
      },
      live_turn_segments: [
        {
          segment_id: 'assistant-live-2:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-live-2',
            content: 'Hello',
            started_at: '2026-03-31T10:00:00Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
        {
          segment_id: 'tool-2',
          kind: 'tool_call',
          tool_call: {
            tool_call_id: 'tool-2',
            parent_message_id: 'assistant-live-2',
            tool_name: 'read_file',
            args_text: '{"path":"app.tsx"}',
            status: 'running',
            started_at: '2026-03-31T10:00:01Z',
          },
        },
      ],
    });

    expect(state.live_assistant_message?.content).toBe('Hello world');
    expect(state.live_assistant_message?.tool_calls[0]).toMatchObject({
      tool_call_id: 'tool-2',
      status: 'completed',
      duration_ms: 12,
      result: {
        content: 'Read app.tsx',
      },
    });
    expect(state.live_turn_segments).toHaveLength(3);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        content: 'Hello',
      },
    });
    expect(state.live_turn_segments[1]).toMatchObject({
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-2',
        status: 'completed',
      },
    });
    expect(state.live_turn_segments[2]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        content: ' world',
      },
    });
  });

  it('reconciles task-plan document steps from completed publish and review actions', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-plan',
        type: 'assistant.message.completed',
        sequence_no: 1,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-plan-1',
          role: 'assistant',
          content: 'Drafted the plan doc.',
          tool_invocations: [
            {
              tool_name: 'update_plan',
              input: {
                plan: [
                  { step: 'Explore repo structure and identify root cause', status: 'completed' },
                  { step: 'Draft task planning document', status: 'in_progress' },
                  { step: 'Publish and request review', status: 'pending' },
                ],
              },
            },
            {
              tool_name: 'publish_task_plan_doc',
              input: {
                content: '# Plan',
              },
              output_summary: 'Published task planning document',
            },
          ],
        },
      }),
      buildEvent({
        id: 'review-requested',
        type: 'interaction.requested',
        sequence_no: 2,
        payload: {
          interaction_id: 'review-1',
          interaction_kind: 'review_checkpoint',
          status: 'pending',
        },
      }),
    ]);

    expect(state.current_plan?.plan).toEqual([
      { step: 'Explore repo structure and identify root cause', status: 'completed' },
      { step: 'Draft task planning document', status: 'completed' },
      { step: 'Publish and request review', status: 'completed' },
    ]);
  });

  it('hydrates the current plan from the session snapshot and keeps later live updates', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'live-plan-update',
        type: 'plan.updated',
        sequence_no: 1_700_000_102,
        payload: {
          content: JSON.stringify({
            plan: [
              { step: 'Inspect repo context', status: 'completed' },
              { step: 'Patch the handler', status: 'completed' },
              { step: 'Run targeted tests', status: 'in_progress' },
            ],
          }),
        },
      }),
    ], {
      current_plan: {
        plan: [
          { step: 'Inspect repo context', status: 'completed' },
          { step: 'Patch the handler', status: 'in_progress' },
          { step: 'Run targeted tests', status: 'pending' },
        ],
      },
    });

    expect(state.current_plan).toEqual({
      plan: [
        { step: 'Inspect repo context', status: 'completed' },
        { step: 'Patch the handler', status: 'completed' },
        { step: 'Run targeted tests', status: 'in_progress' },
      ],
    });
  });

  it('keeps the latest plan visible across assistant rounds by reading update_plan tool segments', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-start-1',
        type: 'assistant.message.started',
        sequence_no: 1,
        payload: { message_id: 'assistant-1' },
      }),
      buildEvent({
        id: 'plan-tool-start',
        type: 'tool.call.started',
        sequence_no: 2,
        payload: {
          parent_message_id: 'assistant-1',
          tool_call_id: 'plan-tool-1',
          tool_name: 'update_plan',
          args_text: JSON.stringify({
            plan: [
              { step: 'Inspect repo context', status: 'completed' },
              { step: 'Draft task plan doc', status: 'in_progress' },
            ],
          }),
        },
      }),
      buildEvent({
        id: 'plan-tool-complete',
        type: 'tool.call.completed',
        sequence_no: 3,
        payload: {
          parent_message_id: 'assistant-1',
          tool_call_id: 'plan-tool-1',
          tool_name: 'update_plan',
          output_summary: 'plan updated',
        },
      }),
      buildEvent({
        id: 'assistant-complete-1',
        type: 'assistant.message.completed',
        sequence_no: 4,
        payload: { message_id: 'assistant-1', content: 'Repo inspected.' },
      }),
      buildEvent({
        id: 'assistant-start-2',
        type: 'assistant.message.started',
        sequence_no: 5,
        payload: { message_id: 'assistant-2' },
      }),
      buildEvent({
        id: 'assistant-delta-2',
        type: 'assistant.message.delta',
        sequence_no: 6,
        payload: { message_id: 'assistant-2', text: 'Writing the planning draft.' },
      }),
    ]);

    expect(state.current_plan).toEqual({
      plan: [
        { step: 'Inspect repo context', status: 'completed' },
        { step: 'Draft task plan doc', status: 'in_progress' },
      ],
    });
    expect(state.live_turn_segments.every((segment) => (
      segment.kind !== 'tool_call' || segment.tool_call.tool_name !== 'update_plan'
    ))).toBe(true);
  });

  it('hydrates the current plan from persisted activity updates', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'artifact-plan',
        type: 'activity.updated',
        sequence_no: 10,
        runtime_metadata: { source: 'agent_run_artifact', artifact_type: 'run_plan' },
        payload: {
          artifact_type: 'run_plan',
          content: {
            note: 'Keep the scope tight.',
            plan: [
              { step: 'Review PRD draft', status: 'completed' },
              { step: 'Refine implementation tasks', status: 'in_progress' },
            ],
          },
        },
      }),
    ]);

    expect(state.current_plan).toEqual({
      note: 'Keep the scope tight.',
      plan: [
        { step: 'Review PRD draft', status: 'completed' },
        { step: 'Refine implementation tasks', status: 'in_progress' },
      ],
    });
  });
});
