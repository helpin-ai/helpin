// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AuthConfig } from '@/lib/services/authService';

const navigate = vi.fn();
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: ReactNode; className?: string }) => <a href={to} className={className}>{children}</a>,
  useNavigate: () => navigate,
}));
vi.mock('@/components/layout/PublicPageShell', () => ({
  PublicPageShell: ({ children }: { children: ReactNode }) => <main>{children}</main>,
}));
vi.mock('@/components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: () => null,
}));
vi.mock('@/lib/services/passkeyService', () => ({
  passkeyService: {
    isSupported: () => true,
    isAutofillSupported: () => Promise.resolve(false),
    cancelPendingAuthentication: vi.fn(),
  },
}));
vi.mock('@/lib/services/workspacesService', () => ({ workspacesService: { list: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import Login from '../Login';
import { useAuthStore } from '@/stores/authStore';
import { API_BASE } from '@/lib/api';
import { authService } from '@/lib/services/authService';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
// The remember-me checkbox measures itself with ResizeObserver, which jsdom lacks.
globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} };

let container: HTMLDivElement | null = null;
let root: Root | null = null;
const originalLocation = window.location;
const assign = vi.fn();

beforeEach(() => {
  Object.defineProperty(window, 'location', { configurable: true, value: { ...originalLocation, search: '', assign } });
});

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
  assign.mockReset();
  Object.defineProperty(window, 'location', { configurable: true, value: originalLocation });
});

const baseConfig: AuthConfig = { email_verification_required: false, app_email_configured: true, google_login_enabled: false };

function render(configuration: AuthConfig, search = '') {
  window.location.search = search;
  useAuthStore.setState({ configuration });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<Login />));
  return container;
}

const buttonLabels = (page: HTMLElement) => Array.from(page.querySelectorAll('button')).map((button) => button.textContent?.trim());

describe('Login', () => {
  it('shows the password form and no SSO button on a server without OIDC', () => {
    const page = render(baseConfig);
    expect(page.querySelector('#password')).not.toBeNull();
    expect(buttonLabels(page)).toContain('Sign in with passkey');
    expect(buttonLabels(page).some((label) => label?.startsWith('Sign in with ') && label !== 'Sign in with passkey')).toBe(false);
  });

  it('offers the provider button next to the password form and carries the redirect path', () => {
    const page = render({ ...baseConfig, oidc_login_enabled: true, oidc_provider_name: 'Authelia' }, '?redirect=%2Foauth%2Fconsent%3Fid%3D1');
    expect(page.querySelector('#password')).not.toBeNull();
    const sso = Array.from(page.querySelectorAll('button')).find((button) => button.textContent === 'Sign in with Authelia');
    expect(sso).toBeDefined();

    act(() => sso!.click());

    expect(assign).toHaveBeenCalledWith(`${API_BASE}/auth/oidc/start?redirect=%2Foauth%2Fconsent%3Fid%3D1`);
  });

  it('shows only the provider button when local sign-in is off', () => {
    const page = render({ ...baseConfig, oidc_login_enabled: true, oidc_provider_name: 'Authelia', local_login_enabled: false, signup_mode: 'open' });
    expect(page.querySelector('#email')).toBeNull();
    expect(page.querySelector('#password')).toBeNull();
    expect(page.querySelector('a[href="/forgot-password"]')).toBeNull();
    expect(page.querySelector('a[href^="/register"]')).toBeNull();
    expect(buttonLabels(page)).toEqual(['Sign in with Authelia']);
  });

  it('goes straight to the provider when local sign-in is off, unless a sign-in error is shown', () => {
    const config = { ...baseConfig, oidc_login_enabled: true, local_login_enabled: false };
    render(config, '?redirect=%2Fboard');
    expect(assign).toHaveBeenCalledWith(`${API_BASE}/auth/oidc/start?redirect=%2Fboard`);

    act(() => root?.unmount());
    container?.remove();
    assign.mockReset();
    render(config, '?error=exchange_failed');
    expect(assign).not.toHaveBeenCalled();
  });

  it('does not redirect automatically while local sign-in is on', () => {
    render({ ...baseConfig, oidc_login_enabled: true });
    expect(assign).not.toHaveBeenCalled();
  });

  it('keeps the signup link and passkey button while local sign-in is on', () => {
    const page = render({ ...baseConfig, signup_mode: 'open' });
    expect(page.querySelector('a[href^="/register"]')).not.toBeNull();
    expect(buttonLabels(page)).toContain('Sign in with passkey');
  });

  it('explains a sign-in error returned in the query string', () => {
    const page = render({ ...baseConfig, oidc_login_enabled: true }, '?error=email_in_use');
    expect(page.querySelector('[role="alert"]')?.textContent).toContain('not linked to this sign-in provider');
  });

  it('shows neither the password form nor the passkey button until the config has loaded', async () => {
    let resolveConfig: (value: AuthConfig) => void = () => {};
    vi.spyOn(authService, 'config').mockReturnValue(new Promise((resolve) => {
      resolveConfig = (data) => resolve({ data, error: null } as Awaited<ReturnType<typeof authService.config>>);
    }));
    useAuthStore.setState({ configuration: null });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    act(() => root?.render(<Login />));
    expect(container.querySelector('#password')).toBeNull();
    expect(buttonLabels(container)).toEqual([]);

    await act(async () => resolveConfig({ ...baseConfig, oidc_login_enabled: true, local_login_enabled: false }));

    expect(container.querySelector('#password')).toBeNull();
    expect(buttonLabels(container)).toEqual(['Sign in with Single sign-on']);
  });

  it('falls back to the password form when the config cannot be loaded', async () => {
    vi.spyOn(authService, 'config').mockRejectedValue(new Error('offline'));
    useAuthStore.setState({ configuration: null });
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => root?.render(<Login />));
    expect(container.querySelector('#password')).not.toBeNull();
  });

  it('shows no alert without an error', () => {
    expect(render(baseConfig).querySelector('[role="alert"]')).toBeNull();
  });
});
