import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const ensureAuthConfiguration = vi.hoisted(() => vi.fn());

vi.mock('@tanstack/react-router', () => ({
  createFileRoute: () => (options: unknown) => options,
  redirect: (target: unknown) => ({ redirect: target }),
  // The router plugin code-splits route components in tests as well.
  lazyRouteComponent: () => () => null,
  lazyFn: () => () => null,
}));
vi.mock('@/pages/SetupSuccessPage', () => ({ SetupSuccessPage: () => null }));
vi.mock('@/stores/authStore', () => ({ ensureAuthConfiguration }));

import { Route } from '../_authenticated/w/$slug/setup';

type Guard = { beforeLoad: (context: { params: { slug: string } }) => Promise<void> };
const guard = Route as unknown as Guard;

describe('workspace Setup route guard', () => {
  beforeEach(() => {
    ensureAuthConfiguration.mockReset();
    vi.stubEnv('VITE_SETUP_SUCCESS_ENABLED', '');
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('waits for the API configuration and opens the Setup guide when enabled', async () => {
    ensureAuthConfiguration.mockResolvedValue({ setup_guide_enabled: true });

    await expect(guard.beforeLoad({ params: { slug: 'acme' } })).resolves.toBeUndefined();
    expect(ensureAuthConfiguration).toHaveBeenCalledTimes(1);
  });

  it('redirects to My Work when the API has the Setup guide disabled', async () => {
    ensureAuthConfiguration.mockResolvedValue({ setup_guide_enabled: false });

    await expect(guard.beforeLoad({ params: { slug: 'acme' } })).rejects.toEqual({
      redirect: { to: '/w/$slug/pm/my-work', params: { slug: 'acme' } },
    });
  });
});
