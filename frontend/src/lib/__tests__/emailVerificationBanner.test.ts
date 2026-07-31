import { describe, expect, it } from 'vitest';

import { shouldShowEmailVerificationBanner } from '../emailVerificationBanner';

describe('emailVerificationBanner', () => {
  it('hides the banner during first-workspace onboarding', () => {
    expect(shouldShowEmailVerificationBanner({
      emailVerified: false,
      pathname: '/onboarding',
      search: { step: 'details' },
    })).toBe(false);
  });

  it('keeps hiding the banner for the legacy workspace creation onboarding URL', () => {
    expect(shouldShowEmailVerificationBanner({
      emailVerified: false,
      pathname: '/workspaces',
      search: { create: true },
    })).toBe(false);
  });

  it('shows the banner for unverified users outside onboarding', () => {
    expect(shouldShowEmailVerificationBanner({
      emailVerified: false,
      pathname: '/w/acme/pm/my-work',
      search: {},
    })).toBe(true);
  });

  it('hides the banner for verified users', () => {
    expect(shouldShowEmailVerificationBanner({
      emailVerified: true,
      pathname: '/workspaces',
      search: {},
    })).toBe(false);
  });
});
