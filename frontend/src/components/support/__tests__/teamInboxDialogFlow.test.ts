import { describe, expect, it } from 'vitest';

import { getTeamInboxDialogSteps } from '../teamInboxDialogFlow';

describe('team inbox dialog flow', () => {
  it('keeps team and assignment setup in the members step', () => {
    const steps = getTeamInboxDialogSteps();

    expect(steps.map((step) => step.label)).toEqual(['Details', 'Members & Assignment', 'Routing']);
    expect(steps[0].fields).toEqual(['identity', 'description', 'reply_expectations']);
    expect(steps[1].fields).toEqual(['linked_team', 'additional_members', 'assignment_mode']);
    expect(steps[2].fields).toEqual(['automated_routing_notice', 'manual_routing_rules', 'ai_routing_prompt']);
    expect(steps[1].description).toContain('linked team');
    expect(steps[1].description).toContain('additional members');
    expect(steps[2].description).toContain('Automated routing');
  });
});
