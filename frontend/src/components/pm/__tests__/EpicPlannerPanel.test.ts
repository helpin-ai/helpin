import { describe, expect, it } from 'vitest';

import {
  getEpicPlannerFeaturedAction,
  nextCompletedRunNotificationId,
  shouldReloadEpicPlannerRuns,
  shouldShowRunsLoading,
} from '../EpicPlannerPanel';
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
    status: 'queued',
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

describe('nextCompletedRunNotificationId', () => {
  it('notifies once for a newly completed run', () => {
    expect(
      nextCompletedRunNotificationId({ id: 'run-1', status: 'completed' }, null),
    ).toBe('run-1');
  });

  it('does not notify again for the same completed run id', () => {
    expect(
      nextCompletedRunNotificationId({ id: 'run-1', status: 'completed' }, 'run-1'),
    ).toBeNull();
  });

  it('does not notify for non-completed runs', () => {
    expect(
      nextCompletedRunNotificationId({ id: 'run-1', status: 'running' }, null),
    ).toBeNull();
  });
});

describe('shouldReloadEpicPlannerRuns', () => {
  it('reloads for queued and running events on the active epic', () => {
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'epic', parent_id: 'epic-1', data: { status: 'queued' } },
        'epic-1',
      ),
    ).toBe(true);
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'epic', parent_id: 'epic-1', data: { status: 'running' } },
        'epic-1',
      ),
    ).toBe(true);
  });

  it('ignores events for other targets', () => {
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'task', parent_id: 'epic-1' },
        'epic-1',
      ),
    ).toBe(false);
    expect(
      shouldReloadEpicPlannerRuns(
        { parent_type: 'epic', parent_id: 'epic-2' },
        'epic-1',
      ),
    ).toBe(false);
  });
});

describe('shouldShowRunsLoading', () => {
  it('only shows the visible loader for the initial empty load', () => {
    expect(shouldShowRunsLoading(true, false, [])).toBe(true);
    expect(shouldShowRunsLoading(true, true, [])).toBe(false);
    expect(shouldShowRunsLoading(true, false, [run({})])).toBe(false);
  });
});

describe('getEpicPlannerFeaturedAction', () => {
  it('uses pause-specific labels for interactive blockers', () => {
    expect(
      getEpicPlannerFeaturedAction(run({ status: 'paused', pause_reason: 'human_input' }), 'Atlas'),
    ).toEqual({ label: 'Reply to Atlas', runId: 'run-1', emphasis: 'prominent' });

    expect(
      getEpicPlannerFeaturedAction(run({ status: 'paused', pause_reason: 'human_approval' }), 'Atlas'),
    ).toEqual({ label: 'Review Atlas request', runId: 'run-1', emphasis: 'prominent' });

    expect(
      getEpicPlannerFeaturedAction(run({ status: 'paused', pause_reason: 'authentication' }), 'Atlas'),
    ).toEqual({ label: 'Complete Atlas sign-in', runId: 'run-1', emphasis: 'prominent' });
  });

  it('opens queued and running runs prominently', () => {
    expect(
      getEpicPlannerFeaturedAction(run({ id: 'run-active', status: 'running' }), 'Atlas'),
    ).toEqual({ label: 'Open run', runId: 'run-active', emphasis: 'prominent' });

    expect(
      getEpicPlannerFeaturedAction(run({ status: 'queued' }), 'Atlas').label,
    ).toBe('Open run');
  });

  it('views terminal runs quietly', () => {
    for (const status of ['completed', 'failed', 'cancelled'] as const) {
      expect(getEpicPlannerFeaturedAction(run({ status }), 'Atlas')).toEqual({
        label: 'View run',
        runId: 'run-1',
        emphasis: 'quiet',
      });
    }
  });

  it('falls back to a generic agent name', () => {
    expect(
      getEpicPlannerFeaturedAction(run({ status: 'paused', pause_reason: 'human_input' }), null).label,
    ).toBe('Reply to AI planner');
  });
});
