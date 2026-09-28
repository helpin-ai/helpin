import { describe, expect, it } from 'vitest';
import type { AgentRun, CodingSessionStreamState, CommandBarPlanSummary, RunPlanArtifact } from '@/lib/pmTypes';
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

function delegatedPlan(child: Partial<AgentRun> = {}): CommandBarPlanSummary {
  return {
    id: 'plan-1', status: 'running', prompt: 'Research',
    page_context: { entity_type: 'workspace', entity_id: 'ws-1', display_title: 'Workspace' },
    steps: [{ agent_id: 'research', agent_name: 'Research Agent', instructions: 'Research',
      target: { entity_type: 'workspace', entity_id: 'ws-1', display_title: 'Workspace' } }],
    run_ids_by_step: { 0: 'child-1' }, current_step_index: 0, run_count: 1,
    created_at: '2026-08-14T10:00:00Z', updated_at: '2026-08-14T10:00:00Z',
    runs: [run({ id: 'child-1', ...child })],
  };
}

describe('resolveAgentLiveProgress', () => {
  it('keeps the working status steady while the transcript shows the running tool', () => {
    const result = resolveAgentLiveProgress({
      run: run(), currentPlan: null, sending: false,
      stream: stream({ live_turn_segments: [{
        segment_id: 'tool-1', kind: 'tool_call',
        tool_call: { tool_call_id: 'tool-1', parent_message_id: 'message-1', tool_name: 'mcp__helpin__list_tasks', args_text: '{}', status: 'running' },
      }] }),
    });
    expect(result?.label).toBe('Working…');
  });

  it('keeps plan details out of the working status', () => {
    const currentPlan = { plan: [{ step: 'Compare the billing options', status: 'in_progress' }] } as RunPlanArtifact;
    expect(resolveAgentLiveProgress({ run: run(), stream: stream(), currentPlan, sending: false })?.label)
      .toBe('Working…');
  });

  it('identifies an active delegated agent', () => {
    expect(resolveAgentLiveProgress({
      run: run(), stream: stream(), currentPlan: null, delegatedPlans: [delegatedPlan()], sending: false,
    })?.label).toBe('Research Agent is running');
  });

  it.each(['paused', 'completed'] as const)('keeps delegated progress visible after the parent is %s and has answered', (status) => {
    expect(resolveAgentLiveProgress({
      run: run({ status, pause_reason: 'awaiting_user_message' }),
      stream: stream({ transcript_messages: [{
        event_id: 'handoff', role: 'assistant', content: 'I started the reviewer.',
        message_type: 'assistant_final', timestamp: '2026-08-14T10:00:40Z', sequence_no: 2,
      }] }),
      currentPlan: null, delegatedPlans: [delegatedPlan()], sending: false,
    })).toMatchObject({ label: 'Research Agent is running · Ask Agent is waiting', tone: 'working' });
  });

  it.each([
    ['human_approval', 'needs approval'], ['human_input', 'needs your input'],
    ['authentication', 'needs sign-in'], ['manual', 'is paused'], ['awaiting_user_message', 'is waiting'],
  ] as const)('preserves delegated %s waits', (pause_reason, label) => {
    expect(resolveAgentLiveProgress({
      run: run({ status: 'completed' }), stream: stream(), currentPlan: null, sending: false,
      delegatedPlans: [delegatedPlan({ status: 'paused', pause_reason })],
    })).toMatchObject({ label: `Research Agent ${label} · Ask Agent is waiting`, tone: 'waiting', delegated: true });
  });

  it('shows queued work and pending launches', () => {
    const input = { run: run(), stream: stream(), currentPlan: null, sending: false };
    expect(resolveAgentLiveProgress({ ...input, delegatedPlans: [delegatedPlan({ status: 'queued' })] })?.label)
      .toBe('Research Agent is queued');
    expect(resolveAgentLiveProgress({ ...input, delegatedPlans: [{ ...delegatedPlan(), runs: [], run_ids_by_step: {} }] })?.label)
      .toBe('Starting sub-agent…');
  });

  it('summarizes multiple children without losing an approval blocker', () => {
    const second = delegatedPlan({ id: 'child-2', status: 'paused', pause_reason: 'human_approval' });
    second.id = 'plan-2';
    second.run_ids_by_step = { 0: 'child-2' };
    expect(resolveAgentLiveProgress({ run: run(), stream: stream(), currentPlan: null, sending: false,
      delegatedPlans: [delegatedPlan(), second],
    })).toMatchObject({ label: '2 sub-agents active · Approval needed', tone: 'working' });
  });

  it('returns to the parent status after children finish, even before the plan settles', () => {
    expect(resolveAgentLiveProgress({ run: run(), stream: stream(), currentPlan: null, sending: false,
      delegatedPlans: [delegatedPlan({ status: 'completed' })],
    })).toMatchObject({ label: 'Working…' });
  });

  it('moves to working as soon as the runtime is running, before its first activity arrives', () => {
    expect(resolveAgentLiveProgress({
      run: run(), stream: stream(), currentPlan: null, sending: false,
    })?.label).toBe('Working…');
  });

  it('shows collaboration waits without adding transcript messages', () => {
    expect(resolveAgentLiveProgress({
      run: run({ status: 'paused', pause_reason: 'human_approval' }), stream: stream(), currentPlan: null, sending: false,
    })?.label).toBe('Waiting for approval');
  });

  it('shows the pending pause before execution has stopped', () => {
    expect(resolveAgentLiveProgress({
      run: run({ execution_stage: 'pausing' }), stream: stream(), currentPlan: null, sending: false,
    })?.label).toBe('Pausing…');
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

  it('starts a follow-up timer from the newly submitted message', () => {
    const result = resolveAgentLiveProgress({
      run: run({ started_at: '2026-08-14T10:00:00Z' }),
      stream: stream(),
      currentPlan: null,
      sending: true,
      localStartedAt: '2026-08-14T10:05:00Z',
    });
    expect(result?.startedAt).toBe('2026-08-14T10:05:00Z');
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

  it('keeps working after an ordinary provider message finishes', () => {
    expect(resolveAgentLiveProgress({
      run: run(), currentPlan: null, sending: false,
      stream: stream({ live_turn_segments: [{
        segment_id: 'answer-1', kind: 'assistant_message',
        assistant_message: { message_id: 'answer-1', content: 'Here is the plan', status: 'completed', tool_calls: [] },
      }] }),
    })?.label).toBe('Working…');
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
    })?.label).toBe('Working…');
  });

  it('keeps a completed run status available for the worked-duration label', () => {
    expect(resolveAgentLiveProgress({
      run: run({ status: 'completed' }), stream: stream(), currentPlan: null, sending: false,
    })).toMatchObject({ label: 'Worked', completed: true });
  });
});

describe('formatAgentElapsed', () => {
  it('formats seconds, minutes, and hours as elapsed duration', () => {
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T10:00:08Z'))).toBe('8s');
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T10:00:08Z'), 3_000)).toBe('5s');
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T10:01:04Z'))).toBe('1m 04s');
    expect(formatAgentElapsed('2026-08-14T10:00:00Z', Date.parse('2026-08-14T11:06:02Z'))).toBe('1h 06m');
  });
});
