import { describe, expect, it } from 'vitest';
import { blankPlaybook } from '../crmPlaybookPresentation';
import { playbookSetupIssues } from '../crmPlaybookSetup';

describe('Playbook setup readiness', () => {
  it('does not mark untouched defaults as complete', () => {
    const issues = playbookSetupIssues(blankPlaybook(), false);
    expect(issues.purpose).not.toHaveLength(0);
    expect(issues.milestones).not.toHaveLength(0);
    expect(issues.team).not.toHaveLength(0);
    expect(issues.follow_up).toHaveLength(0);
  });
  it('accepts a manual playbook without any automation connection', () => {
    const d = blankPlaybook();
    d.objective = 'Agree a next step';
    d.milestones = [{ key: 'agreed', name: 'Next step agreed', success_criteria: 'Customer confirmation' }];
    d.responsibilities.escalation_member_id = 'member-1';
    expect(Object.values(playbookSetupIssues(d, true)).flat()).toEqual([]);
    expect(playbookSetupIssues(d, false).team).not.toHaveLength(0);
  });
  it('rejects incomplete milestones, invalid intervals, and absent stopping rules', () => {
    const d = blankPlaybook();
    d.milestones = [{ key: 'agreed', name: ' ', success_criteria: '' }];
    d.policy.check_after_hours = 1.5;
    d.policy.escalate_after_hours = 1;
    d.policy.stop_conditions = [];
    const issues = playbookSetupIssues(d, true);
    expect(issues.milestones).not.toHaveLength(0);
    expect(issues.follow_up).toHaveLength(3);
  });
});
