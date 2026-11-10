// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';

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

  it('resolves a relative API base against the page origin', async () => {
    vi.resetModules();
    vi.doMock('@/lib/api', () => ({ API_BASE: '/api' }));
    const { buildOidcAuthStartUrl: build } = await import('@/lib/authOidc');
    expect(build()).toBe(`${window.location.origin}/api/auth/oidc/start`);
    vi.doUnmock('@/lib/api');
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
