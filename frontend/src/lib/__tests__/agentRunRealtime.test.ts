import { describe, expect, it } from 'vitest';
import { createAgentRunUpdateTracker } from '../agentRunRealtime';

const event = (data: Record<string, unknown>, entity_id = 'run-1') => ({
  action: 'updated', entity_id,
  data: { status: 'paused', pause_reason: 'awaiting_user_message', change_kind: 'state', ...data },
});

describe('agent run change tracking', () => {
  it('suppresses repeated explicit state but delivers same-state content', () => {
    const track = createAgentRunUpdateTracker();
    expect(track(event({})).kind).toBe('lifecycle');
    expect(track(event({}))).toEqual({ kind: 'duplicate', attentionChanged: false });
    for (const change_kind of ['message', 'interaction', 'artifact']) {
      expect(track(event({ change_kind }))).toEqual({ kind: 'content', attentionChanged: false });
    }
  });

  it('invalidates attention on entry and exit, including resuming execution', () => {
    const track = createAgentRunUpdateTracker();
    track(event({}));
    expect(track(event({ pause_reason: 'human_approval' })).attentionChanged).toBe(true);
    expect(track(event({ pause_reason: 'human_approval', approval_state: 'approved' })).attentionChanged).toBe(false);
    expect(track(event({ status: 'running', pause_reason: 'none' }))).toEqual({ kind: 'lifecycle', attentionChanged: true });
  });

  it('keeps untyped legacy events conservative and scopes each run', () => {
    const track = createAgentRunUpdateTracker();
    track(event({}));
    expect(track(event({ change_kind: undefined })).kind).toBe('lifecycle');
    expect(track(event({}, 'run-2')).kind).toBe('lifecycle');
  });

  it('does not let pre-save content suppress the subsequent persisted state', () => {
    const track = createAgentRunUpdateTracker();
    track(event({}));
    expect(track(event({ change_kind: 'interaction', pause_reason: 'human_approval' })))
      .toEqual({ kind: 'content', attentionChanged: false });
    expect(track(event({ pause_reason: 'human_approval' })))
      .toEqual({ kind: 'lifecycle', attentionChanged: true });
  });

  it('delivers persisted row changes even when status stays the same', () => {
    const track = createAgentRunUpdateTracker();
    track(event({ state_revision: '2026-09-07T12:00:00Z' }));
    expect(track(event({ state_revision: '2026-09-07T12:00:01Z' })))
      .toEqual({ kind: 'lifecycle', attentionChanged: false });
  });

  it('discovers first running states and delivers both aliases in either order', () => {
    const track = createAgentRunUpdateTracker();
    for (const entity of ['coding_session', 'agent_run']) {
      const update = { ...event({ status: 'running', pause_reason: 'none' }), entity };
      expect(track(update).kind).toBe('lifecycle');
      expect(track(update).kind).toBe('duplicate');
    }
  });

  it('observes legacy transitions between typed snapshots without forgetting attention', () => {
    const track = createAgentRunUpdateTracker();
    track(event({ pause_reason: 'human_approval', approval_state: 'pending' }));
    expect(track(event({ change_kind: undefined, status: 'running', pause_reason: 'none' })))
      .toEqual({ kind: 'lifecycle', attentionChanged: true });
    expect(track(event({ pause_reason: 'human_approval', approval_state: 'pending' })))
      .toEqual({ kind: 'lifecycle', attentionChanged: true });
  });
});
