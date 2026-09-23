// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AuthConfig } from '@/lib/services/authService';

const navigate = vi.fn();
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: ReactNode; className?: string }) => <a href={to} className={className}>{children}</a>,
  useNavigate: () => navigate,
}));
vi.mock('@/components/layout/PublicPageShell', () => ({
  PublicPageShell: ({ children }: { children: ReactNode }) => <main>{children}</main>,
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import Register from '../Register';
import { useAuthStore } from '@/stores/authStore';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
  navigate.mockReset();
});

const baseConfig: AuthConfig = { email_verification_required: false, app_email_configured: true, google_login_enabled: false };

function render(configuration: AuthConfig, signUp = vi.fn().mockResolvedValue({ error: null })) {
  useAuthStore.setState({ configuration, signUp });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<Register />));
  return { page: container, signUp };
}

async function submit(page: HTMLElement) {
  await act(async () => {
    page.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  });
}

describe('Register', () => {
  it('shows the signup form when signup is open', () => {
    const { page } = render({ ...baseConfig, signup_mode: 'open' });
    expect(page.querySelector('h1, [role="heading"]')?.textContent).toBe('Create your account');
    expect(page.querySelector('form')).not.toBeNull();
  });

  it('explains invite-only signup instead of showing a form', () => {
    const { page } = render({ ...baseConfig, signup_mode: 'invite_only', signup_first_user: false });
    expect(page.querySelector('form')).toBeNull();
    expect(page.textContent).toContain('Signup on this server is by invitation. Ask your admin for an invite.');
    expect(page.querySelector('a[href="/login"]')).not.toBeNull();
  });

  it('lets the first account sign up on a fresh invite-only server', () => {
    const { page } = render({ ...baseConfig, signup_mode: 'invite_only', signup_first_user: true });
    expect(page.querySelector('form')).not.toBeNull();
    expect(page.textContent).toContain('Create the admin account');
  });

  it('names the allowed domains and waits for email confirmation', async () => {
    const signUp = vi.fn().mockResolvedValue({ error: null, verificationRequired: true, email: 'dev@acme.com' });
    const { page } = render({ ...baseConfig, signup_mode: 'domains', signup_allowed_domains: ['acme.com', 'beta.io'] }, signUp);
    expect(page.querySelector('#signup-domains-hint')?.textContent).toContain('acme.com or beta.io');

    await submit(page);

    expect(signUp).toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
    expect(page.textContent).toContain('Check your email');
    expect(page.textContent).toContain('dev@acme.com');
  });

  it('keeps older servers without a signup policy open', () => {
    const { page } = render(baseConfig);
    expect(page.querySelector('form')).not.toBeNull();
  });
});
