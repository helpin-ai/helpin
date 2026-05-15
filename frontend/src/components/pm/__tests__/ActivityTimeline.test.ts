import { describe, expect, it } from 'vitest';

import { formatActivityAction, getActivityActorLabel } from '@/components/pm/ActivityTimeline';

describe('ActivityTimeline formatting', () => {
  it('describes generic updates with the entity when no field is available', () => {
    expect(formatActivityAction('updated', 'epic')).toBe('updated this epic');
  });

  it('describes generic updates with the field when one is available', () => {
    expect(formatActivityAction('updated', 'task', 'owner_member_ids')).toBe('updated owners');
  });

  it('renders automation activity as automation instead of system', () => {
    expect(getActivityActorLabel('auto-started by automation')).toBe('Automation');
    expect(formatActivityAction('auto-started by automation', 'epic')).toBe('started this epic');
  });
});
