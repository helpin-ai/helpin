import { describe, expect, it } from 'vitest';

import { getVisibleTaskListGroupOptions } from '../taskListGrouping';

describe('getVisibleTaskListGroupOptions', () => {
  it('does not include team and labels owner grouping as members', () => {
    expect(
      getVisibleTaskListGroupOptions({
        task_type: true,
        priority: true,
        severity: true,
        epic: true,
        sprint: true,
      }).map((option) => option.label),
    ).toEqual([
      'None',
      'States',
      'Members',
      'Task Type',
      'Priority',
      'Severity',
      'Epic',
      'Sprint',
    ]);
  });

  it('hides advanced options when team field visibility disables them', () => {
    expect(
      getVisibleTaskListGroupOptions({
        task_type: false,
        priority: false,
        severity: true,
        epic: false,
        sprint: true,
      }).map((option) => option.value),
    ).toEqual([
      'none',
      'workflow_state',
      'owner',
      'severity',
      'sprint',
    ]);
  });
});
