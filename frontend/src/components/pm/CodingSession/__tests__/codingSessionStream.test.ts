import { describe, expect, it } from 'vitest';

import {
  buildCodingSessionStreamState,
  mergeCodingSessionStreamSnapshotSeed,
} from '../codingSessionStream';
import type { CodingSessionEvent, CodingSessionStreamSnapshot } from '@/lib/pmTypes';

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
  it('keeps live run status messages in persisted transcript order', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'msg-user',
        type: 'user.message.completed',
        sequence_no: 2,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'persisted-user-1',
          content: 'Please start.',
          role: 'user',
          sequence_no: 2,
        },
      }),
      buildEvent({
        id: 'live-status',
        type: 'assistant.message.completed',
        sequence_no: 1_700_000_001,
        payload: {
          message_id: 'status-1',
          content: 'Preparing workspace and loading run context.',
          role: 'assistant',
          message_type: 'status',
          sequence_no: 1,
        },
      }),
    ]);

    expect(state.live_assistant_message).toBeNull();
    expect(state.live_turn_segments).toHaveLength(0);
    expect(state.transcript_messages.map((message) => message.content)).toEqual([
      'Preparing workspace and loading run context.',
      'Please start.',
    ]);
    expect(state.transcript_messages[0]?.message_type).toBe('status');
  });

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

  it('converts resolved review checkpoints into visible transcript entries with selected findings', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'interaction-review-resolved',
        type: 'interaction.resolved',
        sequence_no: 14,
        payload: {
          interaction_id: 'interaction-1',
          interaction_kind: 'review_checkpoint',
          status: 'resolved',
          request_schema_version: 'helpin.v1',
          request_payload: {
            findings: [
              {
                id: 'finding_1',
                title: 'Nil panic in retry path',
                code_location: 'server/internal/service/foo.go:42',
              },
              {
                id: 'finding_2',
                title: 'Missing regression coverage',
                code_location: 'server/internal/service/foo_test.go:10',
              },
            ],
          },
          response_payload: {
            decision: 'approve',
            selection_mode: 'selected',
            selected_finding_ids: ['finding_2'],
            message: 'Fix this one first.',
          },
        },
        runtime_metadata: { source: 'agent_run_interaction', interaction_kind: 'review_checkpoint' },
      }),
    ]);

    expect(state.transcript_messages).toHaveLength(1);
    expect(state.transcript_messages[0]).toMatchObject({
      role: 'user',
      message_type: 'review_checkpoint_resolution',
    });
    expect(state.transcript_messages[0]?.content).toContain('Approved selected review findings for implementation');
    expect(state.transcript_messages[0]?.content).toContain('Missing regression coverage');
    expect(state.transcript_messages[0]?.content).toContain('`server/internal/service/foo_test.go:10`');
    expect(state.transcript_messages[0]?.content).toContain('Note: Fix this one first.');
  });

  it('does not fall back to all findings when selected scope has no ids', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'interaction-review-resolved-empty-selected',
        type: 'interaction.resolved',
        sequence_no: 15,
        payload: {
          interaction_id: 'interaction-2',
          interaction_kind: 'review_checkpoint',
          status: 'resolved',
          request_schema_version: 'helpin.v1',
          request_payload: {
            findings: [
              {
                id: 'finding_1',
                title: 'Nil panic in retry path',
                code_location: 'server/internal/service/foo.go:42',
              },
              {
                id: 'finding_2',
                title: 'Missing regression coverage',
                code_location: 'server/internal/service/foo_test.go:10',
              },
            ],
          },
          response_payload: {
            decision: 'approve',
            selection_mode: 'selected',
            selected_finding_ids: [],
          },
        },
        runtime_metadata: { source: 'agent_run_interaction', interaction_kind: 'review_checkpoint' },
      }),
    ]);

    expect(state.transcript_messages).toHaveLength(1);
    expect(state.transcript_messages[0]?.content).toContain('Approved the review checkpoint.');
    expect(state.transcript_messages[0]?.content).not.toContain('Nil panic in retry path');
    expect(state.transcript_messages[0]?.content).not.toContain('Missing regression coverage');
  });

  it('falls back to tool_input for persisted historical tool segments', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-persisted-tool-input',
        type: 'assistant.message.completed',
        sequence_no: 6,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-persisted-3',
          content: 'Searching the codebase.',
          turn_segments: [
            {
              segment_id: 'tool-segment-1',
              kind: 'tool_call',
              tool_call: {
                tool_call_id: 'tool-legacy-1',
                tool_name: 'ripgrep',
                status: 'completed',
                tool_input: '{"pattern":"openShareModal","path":"frontend/src"}',
                result: {
                  content: 'frontend/src/components/ArticleEditorHeader.tsx',
                },
              },
            },
          ],
        },
      }),
    ]);

    expect(state.transcript_messages[0]?.turn_segments?.[0]).toMatchObject({
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-legacy-1',
        tool_name: 'ripgrep',
        args_text: '{"pattern":"openShareModal","path":"frontend/src"}',
      },
    });
  });

  it('stringifies object-shaped tool input for persisted historical tool segments', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-persisted-tool-object-input',
        type: 'assistant.message.completed',
        sequence_no: 7,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-persisted-4',
          content: 'Applying the patch.',
          turn_segments: [
            {
              segment_id: 'tool-segment-2',
              kind: 'tool_call',
              tool_call: {
                tool_call_id: 'tool-legacy-2',
                tool_name: 'apply_patch',
                status: 'completed',
                input: {
                  patch: [
                    '*** Begin Patch',
                    '*** Update File: frontend/src/App.tsx',
                    '@@',
                    '-old',
                    '+new',
                    '*** End Patch',
                  ].join('\n'),
                },
              },
            },
          ],
        },
      }),
    ]);

    expect(state.transcript_messages[0]?.turn_segments?.[0]).toMatchObject({
      kind: 'tool_call',
      tool_call: {
        tool_call_id: 'tool-legacy-2',
        tool_name: 'apply_patch',
        args_text: JSON.stringify({
          patch: [
            '*** Begin Patch',
            '*** Update File: frontend/src/App.tsx',
            '@@',
            '-old',
            '+new',
            '*** End Patch',
          ].join('\n'),
        }, null, 2),
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

  it('preserves hydrated queued content when the matching live assistant start arrives', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-start-late',
        type: 'assistant.message.started',
        sequence_no: 1_700_000_100,
        payload: {
          message_id: 'assistant-live-queued',
        },
      }),
      buildEvent({
        id: 'assistant-delta-late',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_101,
        payload: {
          message_id: 'assistant-live-queued',
          text: 'Starting runtime.',
        },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-live-queued',
        content: 'Queued. Preparing workspace.\n',
        started_at: '2026-03-31T10:00:00Z',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [
        {
          segment_id: 'assistant-live-queued:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-live-queued',
            content: 'Queued. Preparing workspace.\n',
            started_at: '2026-03-31T10:00:00Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
      ],
    });

    expect(state.live_assistant_message?.content).toBe('Queued. Preparing workspace.\nStarting runtime.');
    expect(state.live_turn_segments.map((segment) => (
      segment.kind === 'assistant_message' ? segment.assistant_message.content : segment.tool_call.tool_name
    ))).toEqual([
      'Queued. Preparing workspace.\nStarting runtime.',
    ]);
  });

  it('does not duplicate live deltas already covered by a refreshed stream snapshot', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-delta-duplicate-1',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_100,
        payload: {
          message_id: 'assistant-live-covered',
          text: 'Queued. Preparing workspace.\n',
        },
      }),
      buildEvent({
        id: 'assistant-delta-duplicate-2',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_101,
        payload: {
          message_id: 'assistant-live-covered',
          text: 'Starting runtime.',
        },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-live-covered',
        content: 'Queued. Preparing workspace.\nStarting runtime.',
        started_at: '2026-03-31T10:00:00Z',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [
        {
          segment_id: 'assistant-live-covered:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-live-covered',
            content: 'Queued. Preparing workspace.\n',
            started_at: '2026-03-31T10:00:00Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
        {
          segment_id: 'assistant-live-covered:segment:2',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-live-covered',
            content: 'Starting runtime.',
            started_at: '2026-03-31T10:00:01Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
      ],
    });

    expect(state.live_assistant_message?.content).toBe('Queued. Preparing workspace.\nStarting runtime.');
    expect(state.live_turn_segments).toHaveLength(2);
  });

  it('does not duplicate late queued deltas covered by preserved snapshot segments', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'queued-delta-late',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_100,
        payload: {
          message_id: 'assistant-queued',
          text: 'Queued. Preparing workspace.',
        },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-running',
        content: 'Starting runtime.',
        started_at: '2026-03-31T10:00:10Z',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [
        {
          segment_id: 'assistant-queued:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-queued',
            content: 'Queued. Preparing workspace.',
            started_at: '2026-03-31T10:00:00Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
        {
          segment_id: 'assistant-running:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-running',
            content: 'Starting runtime.',
            started_at: '2026-03-31T10:00:10Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
      ],
    });

    expect(state.live_turn_segments.map((segment) => (
      segment.kind === 'assistant_message' ? segment.assistant_message.content : segment.tool_call.tool_name
    ))).toEqual([
      'Queued. Preparing workspace.',
      'Starting runtime.',
    ]);
  });

  it('merges refreshed stream snapshots without dropping queued segments', () => {
    const current: CodingSessionStreamSnapshot = {
      live_assistant_message: {
        message_id: 'assistant-queued',
        content: 'Queued. Preparing workspace.',
        started_at: '2026-03-31T10:00:00Z',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [
        {
          segment_id: 'assistant-queued:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-queued',
            content: 'Queued. Preparing workspace.',
            started_at: '2026-03-31T10:00:00Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
      ],
    };
    const incoming: CodingSessionStreamSnapshot = {
      live_assistant_message: {
        message_id: 'assistant-running',
        content: 'Starting runtime.',
        started_at: '2026-03-31T10:00:10Z',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [
        {
          segment_id: 'assistant-running:segment:1',
          kind: 'assistant_message',
          assistant_message: {
            message_id: 'assistant-running',
            content: 'Starting runtime.',
            started_at: '2026-03-31T10:00:10Z',
            status: 'streaming',
            tool_calls: [],
          },
        },
      ],
    };

    const merged = mergeCodingSessionStreamSnapshotSeed(current, incoming);

    expect(merged?.live_turn_segments.map((segment) => (
      segment.kind === 'assistant_message' ? segment.assistant_message.content : segment.tool_call.tool_name
    ))).toEqual([
      'Queued. Preparing workspace.',
      'Starting runtime.',
    ]);
    expect(merged?.live_assistant_message?.message_id).toBe('assistant-running');
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
