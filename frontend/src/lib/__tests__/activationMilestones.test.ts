import { describe, expect, it } from 'vitest';

import {
  ACTIVATION_MODULES,
  MODULE_ACTIVATION_MILESTONES,
  type ActivationModule,
} from '../activationMilestones';

describe('activation milestones', () => {
  it('defines first and repeat value milestones for every supported module', () => {
    expect(ACTIVATION_MODULES).toEqual(['pm', 'docs', 'support', 'crm', 'automation']);

    for (const module of ACTIVATION_MODULES) {
      const milestones = MODULE_ACTIVATION_MILESTONES[module];
      expect(milestones.firstValue.length).toBeGreaterThan(0);
      expect(milestones.repeatValue.length).toBeGreaterThan(0);
    }
  });

  it('keeps milestone definitions module-specific', () => {
    const firstValues = Object.entries(MODULE_ACTIVATION_MILESTONES).map(([module, milestones]) => [
      module,
      milestones.firstValue,
    ] as const);

    expect(firstValues).toContainEqual(['support', ['first_reply_sent']]);
    expect(firstValues).toContainEqual(['crm', ['first_contact_created', 'first_deal_created']]);
    expect(Object.keys(MODULE_ACTIVATION_MILESTONES) as ActivationModule[]).toHaveLength(5);
  });
});
