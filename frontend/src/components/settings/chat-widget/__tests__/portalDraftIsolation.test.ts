import { describe, expect, it } from 'vitest';
import type { SupportInboxSettings } from '@/lib/pmTypes';
import { buildSettingsDraftFromServer } from '../utils';

describe('chat widget settings draft', () => {
  it('leaves portal values out of its save payload', () => {
    const draft = buildSettingsDraftFromServer({
      portal_enabled: true,
      portal_requests_only: true,
      portal_intake_enabled: true,
      portal_anonymous_intake_enabled: true,
    } as SupportInboxSettings);

    expect(draft).not.toHaveProperty('portal_enabled');
    expect(draft).not.toHaveProperty('portal_requests_only');
    expect(draft).not.toHaveProperty('portal_intake_enabled');
    expect(draft).not.toHaveProperty('portal_anonymous_intake_enabled');
  });
});
