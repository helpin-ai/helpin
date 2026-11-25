// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest';
import { clearPendingSupportSetup, readPendingSupportSetup, savePendingSupportSetup } from '../pendingSupportSetup';

const setup = {
  detection: { website_url: 'https://acme.com', company_name: 'Acme', website_readable: true },
  companyName: 'Acme',
  aiMode: 'answer' as const,
};

afterEach(() => {
  localStorage.clear();
  vi.useRealTimers();
});

describe('pendingSupportSetup', () => {
  it('keeps the choices until they are cleared', () => {
    savePendingSupportSetup(setup);
    expect(readPendingSupportSetup()).toEqual({ ...setup, helpCenterUrl: undefined });

    clearPendingSupportSetup();
    expect(readPendingSupportSetup()).toBeNull();
  });

  it('drops choices older than a week', () => {
    vi.useFakeTimers();
    savePendingSupportSetup(setup);
    vi.setSystemTime(Date.now() + 8 * 24 * 60 * 60 * 1000);

    expect(readPendingSupportSetup()).toBeNull();
  });

  it('ignores anything that is not a saved setup', () => {
    localStorage.setItem('helpin:pending-support-setup', '{"companyName":1}');
    expect(readPendingSupportSetup()).toBeNull();
  });
});
