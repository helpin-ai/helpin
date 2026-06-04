import { describe, expect, it } from 'vitest';

import {
  getAgentAutoRunStateChangeMessage,
  getAgentAutoRunStateChangeToastId,
  shouldNotifyAgentAutoRunStateChange,
} from '../agentAutoRunNotification';

describe('agent auto-run state change notification', () => {
  it('notifies when a task moves into an automated state', () => {
    expect(shouldNotifyAgentAutoRunStateChange({
      fromStateId: 'todo',
      toStateId: 'in-progress',
      automatedStateIds: new Set(['in-progress']),
    })).toBe(true);
  });

  it('does not notify for same-state reorders or non-automated destinations', () => {
    expect(shouldNotifyAgentAutoRunStateChange({
      fromStateId: 'in-progress',
      toStateId: 'in-progress',
      automatedStateIds: new Set(['in-progress']),
    })).toBe(false);

    expect(shouldNotifyAgentAutoRunStateChange({
      fromStateId: 'todo',
      toStateId: 'review',
      automatedStateIds: new Set(['in-progress']),
    })).toBe(false);
  });

  it('uses the destination state name in the toast copy', () => {
    expect(getAgentAutoRunStateChangeMessage('In Progress')).toBe(
      'Agent will run automatically because this task moved to In Progress.',
    );
  });

  it('builds a stable toast id from the destination state', () => {
    expect(getAgentAutoRunStateChangeToastId('in-progress')).toBe(
      'agent-auto-run-state-change-in-progress',
    );
  });
});
