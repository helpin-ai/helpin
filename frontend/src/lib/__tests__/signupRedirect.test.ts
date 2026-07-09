import { describe, expect, it } from 'vitest';

import { signupSuccessRedirect } from '../signupRedirect';

describe('signupRedirect', () => {
  it('opens first workspace creation when signup has no explicit redirect', () => {
    expect(signupSuccessRedirect(null)).toEqual({
      to: '/onboarding',
      search: { step: 'details' },
    });
  });

  it('preserves an explicit safe redirect', () => {
    expect(signupSuccessRedirect('/w/acme/pm/my-work')).toEqual({
      to: '/w/acme/pm/my-work',
    });
  });
});
