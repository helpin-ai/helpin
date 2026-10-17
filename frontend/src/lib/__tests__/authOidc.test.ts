import { describe, expect, it } from 'vitest';

import { API_BASE } from '@/lib/api';
import { buildOidcAuthStartUrl, loginErrorMessage } from '@/lib/authOidc';

describe('buildOidcAuthStartUrl', () => {
  it('points to the backend OIDC start route', () => {
    expect(buildOidcAuthStartUrl()).toBe(`${API_BASE}/auth/oidc/start`);
  });

  it('preserves the post-auth redirect when provided', () => {
    expect(buildOidcAuthStartUrl('/w/acme/pm/my-work')).toBe(
      `${API_BASE}/auth/oidc/start?redirect=%2Fw%2Facme%2Fpm%2Fmy-work`,
    );
  });
});

describe('loginErrorMessage', () => {
  it('shows nothing without an error', () => {
    expect(loginErrorMessage(null)).toBeNull();
    expect(loginErrorMessage('')).toBeNull();
  });

  it('explains a known reason', () => {
    expect(loginErrorMessage('email_in_use')).toContain('not linked');
  });

  it('falls back to a generic message for an unknown reason', () => {
    expect(loginErrorMessage('<script>')).toBe('Sign-in failed. Try again.');
  });
});
