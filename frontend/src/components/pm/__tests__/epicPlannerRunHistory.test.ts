import { describe, expect, it } from 'vitest';

import {
  compactRelativeAge,
  groupHistoryRuns,
  historyGroupTimeLabel,
  isCommandBarRun,
  isDecayedRun,
  runResultSummary,
} from '../epicPlannerRunHistory';
import type { AgentRun } from '@/lib/pmTypes';

function run(overrides: Partial<AgentRun>): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    target_type: 'epic',
    target_id: 'epic-1',
    runtime_kind: 'native_sdk',
    invocation_mode: 'interactive',
    approval_state: 'not_required',
    pause_reason: 'none',
    status: 'completed',
    input: {},
    output_summary: {},
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-04-29T00:00:00Z',
    updated_at: '2026-04-29T00:00:00Z',
    ...overrides,
  };
}

describe('isCommandBarRun', () => {
  it('detects runs dispatched from the command bar', () => {
    expect(isCommandBarRun(run({ input: { trigger: { source: 'command_bar' } } }))).toBe(true);
  });

  it('treats planner / manual runs as non-command-bar', () => {
    expect(isCommandBarRun(run({ input: {} }))).toBe(false);
    expect(isCommandBarRun(run({ input: { trigger: { source: 'manual' } } }))).toBe(false);
    expect(isCommandBarRun(run({ input: { trigger: {} } }))).toBe(false);
  });
});

describe('groupHistoryRuns', () => {
  it('returns no groups for an empty list', () => {
    expect(groupHistoryRuns([])).toEqual([]);
  });

  it('returns a single group for one run', () => {
    const groups = groupHistoryRuns([run({ id: 'a' })]);
    expect(groups).toHaveLength(1);
    expect(groups[0]).toMatchObject({ kind: 'single', agentId: 'agent-1' });
  });

  it('collapses consecutive terminal runs by the same agent', () => {
    const groups = groupHistoryRuns([
      run({ id: 'a', status: 'failed' }),
      run({ id: 'b', status: 'failed' }),
      run({ id: 'c', status: 'cancelled' }),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0].kind).toBe('collapsed');
    expect(groups[0].runs.map((r) => r.id)).toEqual(['a', 'b', 'c']);
  });

  it('breaks groups when the agent changes', () => {
    const groups = groupHistoryRuns([
      run({ id: 'a', agent_id: 'atlas' }),
      run({ id: 'b', agent_id: 'atlas' }),
      run({ id: 'c', agent_id: 'scribe' }),
      run({ id: 'd', agent_id: 'atlas' }),
    ]);
    expect(groups.map((g) => ({ kind: g.kind, agentId: g.agentId }))).toEqual([
      { kind: 'collapsed', agentId: 'atlas' },
      { kind: 'single', agentId: 'scribe' },
      { kind: 'single', agentId: 'atlas' },
    ]);
  });

  it('keeps non-terminal runs as their own single row and breaks adjacency', () => {
    const groups = groupHistoryRuns([
      run({ id: 'a', status: 'completed' }),
      run({ id: 'b', status: 'queued' }),
      run({ id: 'c', status: 'completed' }),
    ]);
    expect(groups.map((g) => ({ kind: g.kind, id: g.runs[0].id }))).toEqual([
      { kind: 'single', id: 'a' },
      { kind: 'single', id: 'b' },
      { kind: 'single', id: 'c' },
    ]);
  });

  it('preserves newest-first order across groups', () => {
    const groups = groupHistoryRuns([
      run({ id: 'a', agent_id: 'scribe' }),
      run({ id: 'b', agent_id: 'atlas' }),
      run({ id: 'c', agent_id: 'atlas' }),
    ]);
    expect(groups.flatMap((g) => g.runs.map((r) => r.id))).toEqual(['a', 'b', 'c']);
  });
});

describe('isDecayedRun', () => {
  const now = new Date('2026-05-01T00:00:00Z');

  it('decays failed runs older than 48 hours', () => {
    expect(isDecayedRun(run({ status: 'failed', created_at: '2026-04-28T23:00:00Z' }), now)).toBe(true);
    expect(isDecayedRun(run({ status: 'failed', created_at: '2026-04-29T01:00:00Z' }), now)).toBe(false);
  });

  it('decays old cancelled runs', () => {
    expect(isDecayedRun(run({ status: 'cancelled', created_at: '2026-04-28T00:00:00Z' }), now)).toBe(true);
  });

  it('never decays completed or active runs', () => {
    expect(isDecayedRun(run({ status: 'completed', created_at: '2026-04-26T00:00:00Z' }), now)).toBe(false);
    expect(isDecayedRun(run({ status: 'running', created_at: '2026-04-26T00:00:00Z' }), now)).toBe(false);
  });
});

describe('runResultSummary', () => {
  it('prefers the error message for failed runs', () => {
    expect(
      runResultSummary(run({ status: 'failed', error_message: ' boom ', output_summary: { summary: 'partial' } })),
    ).toBe('boom');
  });

  it('falls back to output summary for failed runs without an error message', () => {
    expect(
      runResultSummary(run({ status: 'failed', error_message: '  ', output_summary: { summary: 'partial' } })),
    ).toBe('partial');
  });

  it('reads summary, message, or result keys from output_summary', () => {
    expect(runResultSummary(run({ output_summary: { summary: '9 tasks drafted' } }))).toBe('9 tasks drafted');
    expect(runResultSummary(run({ output_summary: { message: 'done' } }))).toBe('done');
    expect(runResultSummary(run({ output_summary: { result: 'ok' } }))).toBe('ok');
  });

  it('returns empty for runs without a usable summary', () => {
    expect(runResultSummary(run({ output_summary: {} }))).toBe('');
    expect(runResultSummary(run({ output_summary: { count: 3 } }))).toBe('');
  });
});

describe('compactRelativeAge', () => {
  const now = new Date('2026-05-01T12:00:00Z');

  it('formats minutes, hours, and days', () => {
    expect(compactRelativeAge('2026-05-01T11:55:00Z', now)).toBe('5m');
    expect(compactRelativeAge('2026-05-01T11:00:30Z', now)).toBe('59m');
    expect(compactRelativeAge('2026-04-30T23:00:00Z', now)).toBe('13h');
    expect(compactRelativeAge('2026-04-29T13:00:00Z', now)).toBe('47h');
    expect(compactRelativeAge('2026-04-29T11:00:00Z', now)).toBe('2d');
    expect(compactRelativeAge('2026-04-26T12:00:00Z', now)).toBe('5d');
  });

  it('returns empty for invalid timestamps', () => {
    expect(compactRelativeAge('not-a-date', now)).toBe('');
  });
});

describe('historyGroupTimeLabel', () => {
  const now = new Date('2026-05-01T12:00:00Z');

  it('uses a relative phrase for single rows', () => {
    const group = groupHistoryRuns([run({ created_at: '2026-04-30T23:00:00Z' })])[0];
    expect(historyGroupTimeLabel(group, now)).toBe('about 13 hours ago');
  });

  it('uses a compact range for collapsed groups', () => {
    const group = groupHistoryRuns([
      run({ id: 'a', status: 'failed', created_at: '2026-04-30T23:00:00Z' }),
      run({ id: 'b', status: 'failed', created_at: '2026-04-30T21:00:00Z' }),
    ])[0];
    expect(historyGroupTimeLabel(group, now)).toBe('13h–15h ago');
  });

  it('collapses equal endpoints to one value', () => {
    const group = groupHistoryRuns([
      run({ id: 'a', status: 'failed', created_at: '2026-04-30T23:00:00Z' }),
      run({ id: 'b', status: 'failed', created_at: '2026-04-30T22:55:00Z' }),
    ])[0];
    expect(historyGroupTimeLabel(group, now)).toBe('13h ago');
  });
});
