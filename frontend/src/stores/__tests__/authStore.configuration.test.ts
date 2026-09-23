import { beforeEach, describe, expect, it, vi } from 'vitest';

const config = vi.hoisted(() => vi.fn());

vi.mock('@/lib/services/authService', () => ({
  authService: { config },
}));

import { ensureAuthConfiguration, useAuthStore } from '../authStore';

const reported = {
  email_verification_required: false,
  app_email_configured: false,
  google_login_enabled: false,
  setup_guide_enabled: true,
};

describe('ensureAuthConfiguration', () => {
  beforeEach(() => {
    config.mockReset();
    useAuthStore.setState({ configuration: null });
  });

  it('fetches the configuration once for concurrent callers and stores it', async () => {
    config.mockResolvedValue({ data: reported, error: null });

    const [first, second] = await Promise.all([ensureAuthConfiguration(), ensureAuthConfiguration()]);

    expect(config).toHaveBeenCalledTimes(1);
    expect(first).toEqual(reported);
    expect(second).toEqual(reported);
    expect(useAuthStore.getState().configuration).toEqual(reported);
  });

  it('reuses a loaded configuration without another request', async () => {
    useAuthStore.setState({ configuration: reported });

    await expect(ensureAuthConfiguration()).resolves.toEqual(reported);
    expect(config).not.toHaveBeenCalled();
  });

  it('resolves null when the request fails and retries on the next call', async () => {
    config.mockRejectedValueOnce(new Error('offline'));
    await expect(ensureAuthConfiguration()).resolves.toBeNull();

    config.mockResolvedValueOnce({ data: reported, error: null });
    await expect(ensureAuthConfiguration()).resolves.toEqual(reported);
    expect(config).toHaveBeenCalledTimes(2);
  });
});
