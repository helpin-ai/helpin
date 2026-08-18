import { describe, expect, it } from 'vitest';
import type { WorkflowWithStates } from '@/lib/pmTypes';
import { resolveMeetingActionStateId } from '@/lib/meetingActionTaskTarget';

const workflow = {
  workflow: { id: 'workflow-1', default_state_id: 'state-todo' },
  states: [
    { id: 'state-done', position: 3, is_default: false },
    { id: 'state-backlog', position: 0, is_default: false },
    { id: 'state-todo', position: 1, is_default: true },
  ],
} as WorkflowWithStates;

describe('resolveMeetingActionStateId', () => {
  it('keeps a selected state from the team workflow', () => {
    expect(resolveMeetingActionStateId(workflow, 'state-backlog')).toBe('state-backlog');
  });

  it('uses the workflow default when no state is selected', () => {
    expect(resolveMeetingActionStateId(workflow)).toBe('state-todo');
  });

  it('falls back to the first positioned state when the configured default is invalid', () => {
    const invalidDefault = {
      ...workflow,
      workflow: { ...workflow.workflow, default_state_id: 'missing' },
      states: workflow.states.map((state) => ({ ...state, is_default: false })),
    };
    expect(resolveMeetingActionStateId(invalidDefault)).toBe('state-backlog');
  });
});
