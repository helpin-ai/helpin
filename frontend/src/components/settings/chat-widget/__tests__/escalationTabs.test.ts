import { describe, expect, it } from 'vitest';

import { CHAT_WIDGET_ESCALATION_TABS } from '../escalationTabs';

describe('CHAT_WIDGET_ESCALATION_TABS', () => {
  it('defines the escalation message tabs in display order', () => {
    expect(CHAT_WIDGET_ESCALATION_TABS).toEqual([
      { value: 'default', label: 'Default' },
      { value: 'busy', label: 'Team busy' },
      { value: 'after_hours', label: 'After hours' },
      { value: 'email', label: 'Email' },
      { value: 'delayed_team_reply', label: 'Delayed team reply' },
    ]);
  });
});
