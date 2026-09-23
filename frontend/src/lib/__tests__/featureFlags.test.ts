import { afterEach, describe, expect, it, vi } from 'vitest';

import { isSetupSuccessEnabled } from '../featureFlags';

describe('isSetupSuccessEnabled', () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('follows the API when it reports the Setup guide as enabled', () => {
    vi.stubEnv('VITE_SETUP_SUCCESS_ENABLED', 'false');
    expect(isSetupSuccessEnabled({ setup_guide_enabled: true })).toBe(true);
  });

  it('never exposes the Setup guide the API has disabled', () => {
    vi.stubEnv('VITE_SETUP_SUCCESS_ENABLED', 'true');
    expect(isSetupSuccessEnabled({ setup_guide_enabled: false })).toBe(false);
  });

  it('falls back to the build flag when the API does not report the field', () => {
    vi.stubEnv('VITE_SETUP_SUCCESS_ENABLED', 'true');
    expect(isSetupSuccessEnabled({})).toBe(true);
    expect(isSetupSuccessEnabled(null)).toBe(true);
  });

  it('stays hidden without API configuration or build flag', () => {
    vi.stubEnv('VITE_SETUP_SUCCESS_ENABLED', '');
    expect(isSetupSuccessEnabled(null)).toBe(false);
    expect(isSetupSuccessEnabled()).toBe(false);
  });
});
