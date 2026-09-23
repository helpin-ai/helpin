import { describe, expect, it } from 'vitest';

import { loginDestination, signupSuccessRedirect } from '../signupRedirect';

describe('signupRedirect', () => {
  it('opens first workspace creation when signup has no explicit redirect', () => {
    expect(signupSuccessRedirect(null)).toEqual({
      to: '/onboarding',
      search: { step: 'workspace' },
    });
  });

  it('preserves an explicit safe redirect', () => {
    expect(signupSuccessRedirect('/w/acme/pm/my-work')).toEqual({
      to: '/w/acme/pm/my-work',
    });
  });
});

describe('loginDestination', () => {
  it('sends a person without workspaces to the full onboarding flow', () => {
    expect(loginDestination({ workspaces: [] })).toEqual({ to: '/onboarding', search: { step: 'workspace' } });
    expect(loginDestination({ workspaces: null })).toEqual({ to: '/onboarding', search: { step: 'workspace' } });
  });

  it('prefers the default workspace, then the first one', () => {
    const workspaces = [{ id: 'a', slug: 'alpha' }, { id: 'b', slug: 'beta' }];
    expect(loginDestination({ workspaces, defaultWorkspaceId: 'b' })).toEqual({ to: '/w/$slug/pm/my-work', params: { slug: 'beta' } });
    expect(loginDestination({ workspaces, defaultWorkspaceId: 'missing' })).toEqual({ to: '/w/$slug/pm/my-work', params: { slug: 'alpha' } });
    expect(loginDestination({ workspaces })).toEqual({ to: '/w/$slug/pm/my-work', params: { slug: 'alpha' } });
  });
});
