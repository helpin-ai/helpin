import { describe, expect, it } from 'vitest';

import {
  buildCodingSessionStreamState,
  mergeCodingSessionStreamSnapshotSeed,
} from '../codingSessionStream';
import { isToolName } from '@/lib/toolNames';
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
  it('preserves the client correlation and pending delivery of a realtime user message', () => {
    const state = buildCodingSessionStreamState([buildEvent({
      id: 'msg:pending-followup', type: 'user.message.completed', sequence_no: 2,
      runtime_metadata: { source: 'agent_run_message' },
      payload: { message_id: 'pending-followup', role: 'user', content: 'Follow up',
        client_message_id: 'client-followup', delivery_status: 'pending', sequence_no: 2 },
    })]);
    expect(state.transcript_messages[0]).toMatchObject({
      client_message_id: 'client-followup', delivery_status: 'pending',
    });
  });

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

  it('appends provider assistant deltas verbatim, preserving spacing and mid-word token splits', () => {
    // The runtime emits verbatim deltas that already carry their own leading
    // whitespace; token boundaries fall mid-word. We must not guess spacing.
    const tokens = ['Bu', 'ffer', "'s", ' change', 'log', ' loaded', ' well', '.'];
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-start',
        type: 'assistant.message.started',
        sequence_no: 1,
        payload: { message_id: 'assistant-live-words' },
      }),
      ...tokens.map((token, index) => buildEvent({
        id: `assistant-delta-${index}`,
        type: 'assistant.message.delta',
        sequence_no: index + 2,
        payload: { message_id: 'assistant-live-words', text: token },
      })),
    ]);

    expect(state.live_assistant_message?.content).toBe("Buffer's changelog loaded well.");
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: { content: "Buffer's changelog loaded well." },
    });
  });

  it('does not duplicate the message when a completion follows verbatim streamed deltas', () => {
    const tokens = ['Analy', 'zing', ' the', ' change', 'logs', ' now', '.'];
    const state = buildCodingSessionStreamState([
      buildEvent({ id: 'a-start', type: 'assistant.message.started', sequence_no: 1, payload: { message_id: 'm1' } }),
      ...tokens.map((token, index) => buildEvent({
        id: `a-delta-${index}`,
        type: 'assistant.message.delta',
        sequence_no: index + 2,
        payload: { message_id: 'm1', text: token },
      })),
      buildEvent({
        id: 'a-completed',
        type: 'assistant.message.completed',
        sequence_no: 20,
        payload: { message_id: 'm1', text: 'Analyzing the changelogs now.' },
      }),
    ]);

    // Streamed content is an exact prefix of the completed content, so the
    // completion marks the single segment complete rather than re-appending.
    expect(state.live_assistant_message?.content).toBe('Analyzing the changelogs now.');
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: { content: 'Analyzing the changelogs now.', status: 'completed' },
    });
  });

  it('repairs a jumbled live assistant segment from completed text', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-start',
        type: 'assistant.message.started',
        sequence_no: 1,
        payload: { message_id: 'assistant-live-repair' },
      }),
      ...['inspect', 'the', 'Rust', 'crate'].map((token, index) => buildEvent({
        id: `assistant-delta-${index}`,
        type: 'assistant.message.delta',
        sequence_no: index + 2,
        payload: { message_id: 'assistant-live-repair', text: token },
      })),
      buildEvent({
        id: 'assistant-completed',
        type: 'assistant.message.completed',
        sequence_no: 10,
        payload: {
          message_id: 'assistant-live-repair',
          text: 'Inspect the Rust crate first, then build against the new API.',
        },
      }),
    ]);

    expect(state.live_assistant_message?.content).toBe(
      'Inspect the Rust crate first, then build against the new API.',
    );
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        content: 'Inspect the Rust crate first, then build against the new API.',
        status: 'completed',
      },
    });
  });

  it('replaces duplicated live assistant text with shorter completed text', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-delta-duplicate',
        type: 'assistant.message.delta',
        sequence_no: 1,
        payload: {
          message_id: 'assistant-live-duplicate',
          text: 'I found the Docker build arg issue. I found the Docker build arg issue.',
        },
      }),
      buildEvent({
        id: 'assistant-completed-duplicate',
        type: 'assistant.message.completed',
        sequence_no: 2,
        payload: {
          message_id: 'assistant-live-duplicate',
          text: 'I found the Docker build arg issue.',
        },
      }),
    ]);

    expect(state.live_assistant_message?.content).toBe('I found the Docker build arg issue.');
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: {
        content: 'I found the Docker build arg issue.',
        status: 'completed',
      },
    });
  });

  it('attaches persisted tool invocations to finalized assistant turns and drops duplicate live completions', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-persisted',
        type: 'assistant.message.completed',
        sequence_no: 2,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-live-1',
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
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: { message_id: 'assistant-live-1', content: 'Done' },
    });
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

  it('preserves failed persisted tool invocations after replay', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-failed-tool',
        type: 'assistant.message.completed',
        sequence_no: 2,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-failed-tool-1',
          content: '',
          role: 'assistant',
          tool_invocations: [
            {
              tool_call_id: 'tool-failed-1',
              tool_name: 'read_skill',
              input: { key: 'missing' },
              output_summary: 'Skill is unavailable',
              status: 'failed',
              error: 'skill is not available to this agent',
            },
          ],
        },
      }),
    ]);

    expect(state.transcript_messages[0]?.tool_calls?.[0]).toMatchObject({
      tool_call_id: 'tool-failed-1',
      parent_message_id: 'assistant-failed-tool-1',
      status: 'failed',
      result: { error: 'skill is not available to this agent' },
    });
  });

  it('keeps earlier Codex text and tool segments when the final message becomes persisted', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'final-persisted',
        type: 'assistant.message.completed',
        sequence_no: 1,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'msg-final',
          content: 'The repository is ready.',
          role: 'assistant',
        },
      }),
      buildEvent({
        id: 'preamble-started',
        type: 'assistant.message.started',
        sequence_no: 1_700_000_001,
        payload: { message_id: 'msg-preamble' },
      }),
      buildEvent({
        id: 'preamble-delta',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_002,
        payload: { message_id: 'msg-preamble', content: 'I will inspect the repository.' },
      }),
      buildEvent({
        id: 'preamble-completed',
        type: 'assistant.message.completed',
        sequence_no: 1_700_000_003,
        payload: { message_id: 'msg-preamble', content: 'I will inspect the repository.' },
      }),
      buildEvent({
        id: 'tool-started',
        type: 'tool.call.started',
        sequence_no: 1_700_000_004,
        payload: {
          tool_call_id: 'tool-list',
          tool_name: 'run_command',
          parent_message_id: 'msg-preamble',
          args_text: '{"command":"ls"}',
        },
      }),
      buildEvent({
        id: 'tool-completed',
        type: 'tool.call.completed',
        sequence_no: 1_700_000_005,
        payload: {
          tool_call_id: 'tool-list',
          tool_name: 'run_command',
          parent_message_id: 'msg-preamble',
          content: 'README.md',
        },
      }),
      buildEvent({
        id: 'final-started',
        type: 'assistant.message.started',
        sequence_no: 1_700_000_006,
        payload: { message_id: 'msg-final' },
      }),
      buildEvent({
        id: 'final-delta',
        type: 'assistant.message.delta',
        sequence_no: 1_700_000_007,
        payload: { message_id: 'msg-final', content: 'The repository is ready.' },
      }),
      buildEvent({
        id: 'final-completed',
        type: 'assistant.message.completed',
        sequence_no: 1_700_000_008,
        payload: { message_id: 'msg-final', content: 'The repository is ready.' },
      }),
    ]);

    expect(state.live_assistant_message).toBeNull();
    expect(state.live_turn_segments.map((segment) => segment.kind)).toEqual([
      'assistant_message',
      'tool_call',
      'assistant_message',
    ]);
    expect(state.live_turn_segments[0]).toMatchObject({
      assistant_message: {
        message_id: 'msg-preamble',
        content: 'I will inspect the repository.',
      },
    });
    expect(state.live_turn_segments[1]).toMatchObject({
      segment_id: 'tool-list',
      tool_call: { tool_call_id: 'tool-list', status: 'completed' },
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

  it('does not append resolved review checkpoints to the transcript after final assistant output', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'assistant-final',
        type: 'assistant.message.completed',
        sequence_no: 20,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-final-1',
          content: 'Implemented and committed the approved fix.',
          role: 'assistant',
          sequence_no: 20,
        },
      }),
      buildEvent({
        id: 'interaction-review-resolved',
        type: 'interaction.resolved',
        sequence_no: 21,
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
      role: 'assistant',
      content: 'Implemented and committed the approved fix.',
    });
    expect(state.transcript_messages.map((message) => message.message_type)).not.toContain('review_checkpoint_resolution');
  });

  it('keeps persisted review approval transcript messages in timeline order', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'msg-review-approval',
        type: 'user.message.completed',
        sequence_no: 14,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'message-review-approval-1',
          content: 'Approved review findings for implementation:\n\n- Missing regression coverage',
          role: 'user',
          message_type: 'approval',
          sequence_no: 14,
        },
      }),
      buildEvent({
        id: 'assistant-final',
        type: 'assistant.message.completed',
        sequence_no: 20,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'assistant-final-1',
          content: 'Implemented and committed the approved fix.',
          role: 'assistant',
          sequence_no: 20,
        },
      }),
      buildEvent({
        id: 'interaction-review-resolved',
        type: 'interaction.resolved',
        sequence_no: 21,
        payload: {
          interaction_id: 'interaction-1',
          interaction_kind: 'review_checkpoint',
          status: 'resolved',
          request_payload: {
            findings: [
              {
                id: 'finding_1',
                title: 'Missing regression coverage',
              },
            ],
          },
          response_payload: {
            decision: 'approve',
            selection_mode: 'all',
          },
        },
        runtime_metadata: { source: 'agent_run_interaction', interaction_kind: 'review_checkpoint' },
      }),
    ]);

    expect(state.transcript_messages.map((message) => message.content)).toEqual([
      'Approved review findings for implementation:\n\n- Missing regression coverage',
      'Implemented and committed the approved fix.',
    ]);
    expect(state.transcript_messages.map((message) => message.message_type)).toEqual([
      'approval',
      undefined,
    ]);
  });

  it('keeps resolved review checkpoints out of the transcript when selected scope has no ids', () => {
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

    expect(state.transcript_messages).toHaveLength(0);
  });

  it('does not render the same requested-changes note twice', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'msg-request-changes',
        type: 'user.message.completed',
        sequence_no: 12,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'message-request-changes-1',
          content: 'Members should be able to create and edit epics.',
          role: 'user',
          message_type: 'request_changes',
          sequence_no: 12,
        },
      }),
      buildEvent({
        id: 'interaction-approval-resolved',
        type: 'interaction.resolved',
        sequence_no: 13,
        payload: {
          interaction_id: 'interaction-approval-1',
          interaction_kind: 'approval_request',
          status: 'resolved',
          request_schema_version: 'helpin.v1',
          request_payload: {
            title: 'Task Planning Document: Fix Epic Editing for Team Members',
          },
          response_payload: {
            decision: 'request_changes',
            message: 'Members should be able to create and edit epics.',
          },
        },
        runtime_metadata: { source: 'agent_run_interaction', interaction_kind: 'approval_request' },
      }),
    ]);

    expect(state.transcript_messages.map((message) => message.message_type)).toEqual([
      'approval_request_resolution',
    ]);
    const renderedText = state.transcript_messages.map((message) => message.content).join('\n');
    expect(renderedText.match(/Members should be able to create and edit epics\./g)).toHaveLength(1);
  });

  it('dedupes the generated approval acknowledgment and shows what was approved', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'msg-approval-resume',
        type: 'user.message.completed',
        sequence_no: 12,
        runtime_metadata: { source: 'agent_run_message' },
        payload: {
          message_id: 'message-approval-resume-1',
          content: 'Approved. Continue.',
          role: 'user',
          message_type: 'approval',
          sequence_no: 12,
        },
      }),
      buildEvent({
        id: 'interaction-approval-resolved',
        type: 'interaction.resolved',
        sequence_no: 13,
        payload: {
          interaction_id: 'interaction-approval-1',
          interaction_kind: 'approval_request',
          status: 'resolved',
          request_schema_version: 'codex.v1',
          title: 'Approve tool call',
          summary: 'Command: rm -rf ./dist\nWorking directory: /repo\nReason: clean build',
          request_payload: {
            command: 'rm -rf ./dist',
          },
          response_payload: {
            decision: 'approve',
          },
        },
        runtime_metadata: { source: 'agent_run_interaction', interaction_kind: 'approval_request' },
      }),
    ]);

    // Only the synthesized resolution survives — no duplicate "Approved. Continue." bubble.
    expect(state.transcript_messages.map((message) => message.message_type)).toEqual([
      'approval_request_resolution',
    ]);
    const renderedText = state.transcript_messages.map((message) => message.content).join('\n');
    expect(renderedText).not.toContain('Approved. Continue.');
    // The surviving message describes WHAT was approved, not a bare "approval request".
    expect(renderedText).toContain('Command: rm -rf ./dist');
    expect(renderedText).not.toContain('approval request');
  });

  it('timestamps an approval decision at resolved_at instead of a later projection update', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'interaction:approval-1:resolved',
        type: 'interaction.resolved',
        sequence_no: 13,
        timestamp: '2026-08-21T19:53:28Z',
        payload: {
          interaction_id: 'approval-1',
          interaction_kind: 'approval_request',
          status: 'resolved',
          request_schema_version: 'helpin.v1',
          request_payload: { title: 'Approve task planning document' },
          response_payload: { decision: 'approve' },
          resolved_at: '2026-08-21T19:52:13Z',
        },
        runtime_metadata: { source: 'agent_run_interaction' },
      }),
    ]);

    expect(state.transcript_messages).toHaveLength(1);
    expect(state.transcript_messages[0].timestamp).toBe('2026-08-21T19:52:13Z');
  });

  it('keeps an approval decision before the assistant messages resumed by that decision', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'interaction:approval-1:resolved',
        type: 'interaction.resolved',
        sequence_no: 23,
        timestamp: '2026-08-21T19:52:13Z',
        payload: {
          interaction_id: 'approval-1',
          interaction_kind: 'approval_request',
          status: 'resolved',
          request_schema_version: 'helpin.v1',
          summary: 'Approve task planning document',
          response_payload: { decision: 'approve' },
          resolved_at: '2026-08-21T19:52:13Z',
        },
        runtime_metadata: { source: 'agent_run_interaction' },
      }),
      buildEvent({
        id: 'msg:approval-resume',
        type: 'user.message.completed',
        sequence_no: 24,
        timestamp: '2026-08-21T19:52:14Z',
        payload: {
          message_id: 'approval-resume',
          role: 'user',
          message_type: 'approval',
          sequence_no: 13,
          content: 'Approved task planning document.',
        },
        runtime_metadata: { source: 'agent_run_message' },
      }),
      buildEvent({
        id: 'msg:assistant-resumed',
        type: 'assistant.message.completed',
        sequence_no: 27,
        timestamp: '2026-08-21T19:52:34Z',
        payload: {
          message_id: 'assistant-resumed',
          role: 'assistant',
          sequence_no: 14,
          content: 'Approved. I will persist the plan now.',
        },
        runtime_metadata: { source: 'agent_run_message' },
      }),
      buildEvent({
        id: 'msg:assistant-final',
        type: 'assistant.message.completed',
        sequence_no: 33,
        timestamp: '2026-08-21T19:53:22Z',
        payload: {
          message_id: 'assistant-final',
          role: 'assistant',
          sequence_no: 16,
          content: 'Persisted the approved planning document.',
        },
        runtime_metadata: { source: 'agent_run_message' },
      }),
    ]);

    expect(state.transcript_messages.map((message) => [message.message_type, message.content])).toEqual([
      ['approval_request_resolution', 'Approved:\nApprove task planning document'],
      [undefined, 'Approved. I will persist the plan now.'],
      [undefined, 'Persisted the approved planning document.'],
    ]);
  });

  it('settles the active approval tool as soon as its interaction resolves', () => {
    const approvalTool = {
      tool_call_id: 'tool-approval-1',
      tool_name: 'mcp__helpin__request_approval',
      args_text: '{"title":"Approve task planning document"}',
      status: 'running' as const,
    };
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'interaction:approval-1:resolved',
        type: 'interaction.resolved',
        sequence_no: 13,
        timestamp: '2026-08-21T19:52:13Z',
        payload: {
          interaction_id: 'approval-1',
          interaction_kind: 'approval_request',
          status: 'resolved',
          request_schema_version: 'helpin.v1',
          request_payload: { title: 'Approve task planning document' },
          response_payload: { decision: 'approve' },
          resolved_at: '2026-08-21T19:52:13Z',
        },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-approval-1',
        content: '',
        status: 'streaming',
        tool_calls: [approvalTool],
      },
      live_turn_segments: [{
        segment_id: approvalTool.tool_call_id,
        kind: 'tool_call',
        tool_call: approvalTool,
      }],
    });

    expect(state.live_assistant_message?.tool_calls[0]).toMatchObject({
      tool_call_id: approvalTool.tool_call_id,
      status: 'completed',
      completed_at: '2026-08-21T19:52:13Z',
    });
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'tool_call',
      tool_call: {
        tool_call_id: approvalTool.tool_call_id,
        status: 'completed',
        completed_at: '2026-08-21T19:52:13Z',
      },
    });
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

  it('treats a refreshed stream snapshot as authoritative instead of unioning stale generations', () => {
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
      'Starting runtime.',
    ]);
    expect(merged?.live_assistant_message?.message_id).toBe('assistant-running');
  });

  it.each([null, undefined, {}])('clears stale live content when the refreshed snapshot is empty (%s)', (incoming) => {
    const current: CodingSessionStreamSnapshot = {
      live_assistant_message: {
        message_id: 'assistant-stale',
        content: 'This turn has already been persisted.',
        status: 'completed',
        tool_calls: [],
      },
      live_turn_segments: [{
        segment_id: 'assistant-stale:segment:1',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'assistant-stale',
          content: 'This turn has already been persisted.',
          status: 'completed',
          tool_calls: [],
        },
      }],
    };

    expect(mergeCodingSessionStreamSnapshotSeed(
      current,
      incoming as CodingSessionStreamSnapshot | null | undefined,
    )).toBeNull();
  });

  it('retains an unchanged plan without retaining obsolete live turns', () => {
    const current: CodingSessionStreamSnapshot = {
      current_plan: {
        plan: [{ step: 'Research competitors', status: 'in_progress' }],
      },
      live_turn_segments: [{
        segment_id: 'old-segment',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'old-message',
          content: 'Old snapshot generation.',
          status: 'completed',
          tool_calls: [],
        },
      }],
    };
    const incoming: CodingSessionStreamSnapshot = {
      live_turn_segments: [{
        segment_id: 'new-segment',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'new-message',
          content: 'Current snapshot generation.',
          status: 'streaming',
          tool_calls: [],
        },
      }],
    };

    const merged = mergeCodingSessionStreamSnapshotSeed(current, incoming);

    expect(merged?.current_plan).toEqual(current.current_plan);
    expect(merged?.live_turn_segments).toHaveLength(1);
    expect(merged?.live_turn_segments[0]).toMatchObject({ segment_id: 'new-segment' });
  });

  it('does not replay a websocket history that begins midway through snapshot text', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'delta-midword',
        type: 'assistant.message.delta',
        sequence_no: 100,
        payload: { message_id: 'assistant-buffer', content: 'ffer has launched' },
      }),
      buildEvent({
        id: 'delta-tail',
        type: 'assistant.message.delta',
        sequence_no: 101,
        payload: { message_id: 'assistant-buffer', content: ' a new feature.' },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-buffer',
        content: 'Buffer has launched a new feature.',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [{
        segment_id: 'assistant-buffer:segment:1',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'assistant-buffer',
          content: 'Buffer has launched a new feature.',
          status: 'streaming',
          tool_calls: [],
        },
      }],
    });

    expect(state.live_assistant_message?.content).toBe('Buffer has launched a new feature.');
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: { content: 'Buffer has launched a new feature.' },
    });
  });

  it('appends genuinely new deltas after consuming a mid-message snapshot overlap', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'delta-overlap',
        type: 'assistant.message.delta',
        sequence_no: 100,
        payload: { message_id: 'assistant-buffer', content: 'ffer' },
      }),
      buildEvent({
        id: 'delta-new',
        type: 'assistant.message.delta',
        sequence_no: 101,
        payload: { message_id: 'assistant-buffer', content: ' is publishing now.' },
      }),
    ], {
      live_assistant_message: {
        message_id: 'assistant-buffer',
        content: 'Buffer',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [{
        segment_id: 'assistant-buffer:segment:1',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'assistant-buffer',
          content: 'Buffer',
          status: 'streaming',
          tool_calls: [],
        },
      }],
    });

    expect(state.live_assistant_message?.content).toBe('Buffer is publishing now.');
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]).toMatchObject({
      kind: 'assistant_message',
      assistant_message: { content: 'Buffer is publishing now.' },
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
      segment.kind !== 'tool_call' || !isToolName(segment.tool_call.tool_name, 'update_plan')
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
    expect(state.activity_events).toHaveLength(0);
  });

  it('keeps persisted runtime tool-call events in the transcript stream', () => {
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'tool-start',
        type: 'tool.call.started',
        sequence_no: 10,
        payload: {
          parent_message_id: 'assistant-1',
          tool_call_id: 'tool-1',
          tool_name: 'fetch_url',
          args_text: '{"url":"https://example.com"}',
        },
      }),
      buildEvent({
        id: 'tool-complete',
        type: 'tool.call.completed',
        sequence_no: 11,
        payload: {
          parent_message_id: 'assistant-1',
          tool_call_id: 'tool-1',
          tool_name: 'fetch_url',
          output_summary: 'Fetched page',
          duration_ms: 42,
        },
      }),
    ]);

    const toolSegment = state.live_turn_segments.find((segment) => segment.kind === 'tool_call');
    expect(toolSegment?.kind).toBe('tool_call');
    if (toolSegment?.kind !== 'tool_call') return;
    expect(toolSegment.tool_call).toMatchObject({
      tool_call_id: 'tool-1',
      tool_name: 'fetch_url',
      status: 'completed',
      result: {
        output_summary: 'Fetched page',
      },
    });
  });

  it('uses the v2 snapshot watermark instead of replaying covered deltas', () => {
    const snapshot: CodingSessionStreamSnapshot = {
      through_sequence: 2,
      live_assistant_message: {
        message_id: 'assistant-1',
        content: 'Hello',
        status: 'streaming',
        tool_calls: [],
      },
      live_turn_segments: [{
        segment_id: 'assistant-1',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'assistant-1',
          content: 'Hello',
          status: 'streaming',
          tool_calls: [],
        },
      }],
    };
    const state = buildCodingSessionStreamState([
      buildEvent({
        id: 'covered-delta',
        type: 'assistant.message.delta',
        sequence_no: 2,
        runtime_metadata: { source: 'agent-runtime-v2' },
        payload: { message_id: 'assistant-1', content: 'Hello' },
      }),
      buildEvent({
        id: 'new-delta',
        type: 'assistant.message.delta',
        sequence_no: 3,
        runtime_metadata: { source: 'agent-runtime-v2' },
        payload: { message_id: 'assistant-1', content: ' world' },
      }),
    ], snapshot);

    expect(state.live_assistant_message?.content).toBe('Hello world');
    expect(state.live_turn_segments).toHaveLength(1);
    expect(state.live_turn_segments[0]?.kind === 'assistant_message'
      ? state.live_turn_segments[0].assistant_message.content
      : '').toBe('Hello world');
  });
});
