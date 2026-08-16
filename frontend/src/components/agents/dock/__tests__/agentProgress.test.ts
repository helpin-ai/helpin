import { describe, expect, it } from 'vitest';
import type { AgentRun, CodingSessionStreamState, RunPlanArtifact } from '@/lib/pmTypes';
import { formatAgentElapsed, resolveAgentLiveProgress } from '../agentProgress';

function run(overrides: Partial<AgentRun> = {}): AgentRun {
  return {
    id: 'run-1', workspace_id: 'ws-1', agent_id: 'agent-1', target_type: 'workspace', target_id: 'ws-1',
    runtime_kind: 'native_sdk', invocation_mode: 'interactive', status: 'running', pause_reason: 'none',
    approval_state: 'not_required', input: {}, output_summary: {}, cached_input_tokens: 0, input_tokens: 0,
    output_tokens: 0, tokens_used: 0, started_at: '2026-08-14T10:00:00Z', created_at: '2026-08-14T10:00:00Z',
    updated_at: '2026-08-14T10:00:00Z', ...overrides,
  };
}

function stream(overrides: Partial<CodingSessionStreamState> = {}): CodingSessionStreamState {
  return {
    transcript_messages: [], live_assistant_message: null, live_reasoning_message: null,
    live_turn_segments: [], activity_events: [], current_plan: null, completed_tool_calls: [], ...overrides,
  };
}

describe('resolveAgentLiveProgress', () => {
  it('uses one friendly status for a running tool', () => {
    const result = resolveAgentLiveProgress({
      run: run(), currentPlan: null, sending: false,
      stream: stream({ live_turn_segments: [{
        segment_id: 'tool-1', kind: 'tool_call',
        tool_call: { tool_call_id: 'tool-1', parent_message_id: 'message-1', tool_name: 'mcp__helpin__list_tasks', args_text: '{}', status: 'running' },
      }] }),
    });
    expect(result?.label).toBe('List Tasks…');
  });

  it('prefers an active plan step over generic working', () => {
    const currentPlan = { plan: [{ step: 'Compare the billing options', status: 'in_progress' }] } as RunPlanArtifact;
    expect(resolveAgentLiveProgress({ run: run(), stream: stream(), currentPlan, sending: false })?.label)
      .toBe('Compare the billing options');
  });

  it('identifies an active delegated agent', () => {
    expect(resolveAgentLiveProgress({
      run: run(), stream: stream(), currentPlan: null, activeSubAgentName: 'Research Agent', sending: false,
    })?.label).toBe('Working with Research Agent…');
  });

  it('shows collaboration waits without adding transcript messages', () => {
    expect(resolveAgentLiveProgress({
      run: run({ status: 'paused', pause_reason: 'human_approval' }), stream: stream(), currentPlan: null, sending: false,
    })?.label).toBe('Waiting for approval');
  });

  it('only labels an explicit human-input pause as waiting for a reply', () => {
    expect(resolveAgentLiveProgress({
      run: run({ status: 'paused', pause_reason: 'human_input' }), stream: stream(), currentPlan: null, sending: false,
    })?.label).toBe('Waiting for your reply');
    expect(resolveAgentLiveProgress({
      run: run({ status: 'paused', pause_reason: 'awaiting_user_message' }), stream: stream(), currentPlan: null, sending: false,
    })).toBeNull();
  });

  it('shows starting immediately when a new message is sent against an old paused run', () => {
    expect(resolveAgentLiveProgress({
      run: run({ status: 'paused', pause_reason: 'awaiting_user_message' }),
      stream: stream({ live_turn_segments: [{
        segment_id: 'previous-answer', kind: 'assistant_message',
        assistant_message: { message_id: 'previous-answer', content: 'Previous answer', status: 'completed', tool_calls: [] },
      }] }),
      currentPlan: null,
      sending: true,
    })?.label).toBe('Starting…');
  });

  it('describes a streaming answer without hiding progress', () => {
    expect(resolveAgentLiveProgress({
      run: run(), currentPlan: null, sending: false,
      stream: stream({ live_turn_segments: [{
        segment_id: 'answer-1', kind: 'assistant_message',
        assistant_message: { message_id: 'answer-1', content: 'Here is the plan', status: 'streaming', tool_calls: [] },
      }] }),
    })?.label).toBe('Preparing a response…');
  });

  it('shows finishing when the answer exists before the run becomes terminal', () => {
    expect(resolveAgentLiveProgress({
      run: run(), currentPlan: null, sending: false,
      stream: stream({ live_turn_segments: [{
        segment_id: 'answer-1', kind: 'assistant_message',
        assistant_message: { message_id: 'answer-1', content: 'Here is the plan', status: 'completed', tool_calls: [] },
      }] }),
    })?.label).toBe('Finishing…');
  });

  it('does not reuse the previous answer finishing state after a follow-up user message', () => {
    expect(resolveAgentLiveProgress({
      run: run(), currentPlan: null, sending: false,
      stream: stream({
        transcript_messages: [{
          event_id: 'user-2', message_id: 'user-2', role: 'user', content: 'Follow up',
          message_type: 'message', timestamp: '2026-08-14T10:01:00Z', sequence_no: 2,
        }],
        live_turn_segments: [{
          segment_id: 'previous-answer', kind: 'assistant_message',
          assistant_message: { message_id: 'previous-answer', content: 'Previous answer', status: 'completed', tool_calls: [] },
        }],
      }),
    })?.label).toBe('Starting…');
  });
});

describe('formatAgentElapsed', () => {
  it('formats seconds, minutes, and hours as elapsed duration', () => {
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T10:00:08Z'))).toBe('8s');
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T10:01:04Z'))).toBe('1m 04s');
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T11:06:02Z'))).toBe('1h 06m');
  });
});
