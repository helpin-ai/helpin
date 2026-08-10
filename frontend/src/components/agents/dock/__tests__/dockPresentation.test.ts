import { describe, expect, it } from 'vitest';
import type { DockRunSummary } from '@/lib/dockTypes';
import { dockRunContext, dockRunSubtitle, dockRunTitle, presentDockRun, relativeDockTime } from '../dockPresentation';

describe('dockPresentation', () => {
  it.each([
    ['paused', 'human_approval', 'approval', 'needs_you', 'Approve'],
    ['paused', 'authentication', 'authentication', 'needs_you', 'Sign in'],
    ['paused', 'human_input', 'input', 'needs_you', 'Needs you'],
    ['queued', 'none', undefined, 'running', 'Queued'],
    ['running', 'none', undefined, 'running', 'Running'],
    ['failed', 'none', undefined, 'recent', 'Failed'],
    ['completed', 'none', undefined, 'recent', 'Done'],
  ] as const)('maps %s / %s into the roster', (status, pauseReason, attention, group, label) => {
    const result = presentDockRun(status, pauseReason, attention);
    expect(result.group).toBe(group);
    expect(result.label).toBe(label);
  });

  it('builds a compact task identity and repository context', () => {
    const summary = {
      run: {
        target_type: 'task',
        status: 'running',
        pause_reason: 'none',
        target_info: { task_key: 'HLP-42', title: 'Polish the agent dock' },
        repo_full_name: 'helpin-ai/helpin',
        working_branch: 'feature/dock',
        execution_stage: 'Reviewing changes',
      },
      agent: { id: 'agent-1', name: 'Review Agent' },
    } as DockRunSummary;

    expect(dockRunTitle(summary)).toBe('HLP-42 · Polish the agent dock');
    expect(dockRunSubtitle(summary)).toBe('Review Agent · Reviewing changes');
    expect(dockRunContext(summary)).toBe('helpin-ai/helpin · feature/dock');
  });

  it('formats short relative timestamps without future negatives', () => {
    const now = Date.parse('2026-08-09T12:00:00Z');
    expect(relativeDockTime('2026-08-09T11:59:50Z', now)).toBe('now');
    expect(relativeDockTime('2026-08-09T11:42:00Z', now)).toBe('18m');
    expect(relativeDockTime('2026-08-09T09:00:00Z', now)).toBe('3h');
    expect(relativeDockTime('2026-08-07T12:00:00Z', now)).toBe('2d');
    expect(relativeDockTime('2026-08-10T12:00:00Z', now)).toBe('now');
  });
});
