// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CustomerPortalSettings } from '../CustomerPortalSettings';

const mocks = vi.hoisted(() => ({
  save: vi.fn().mockResolvedValue(undefined),
  query: vi.fn(),
  summary: vi.fn(),
}));

vi.mock('@/hooks/queries/useSupport', () => ({
  useChatSettings: () => mocks.query(),
  useUpdateCustomerPortalSettings: () => ({ mutateAsync: mocks.save }),
  useCustomerPortalAccessSummary: (_workspaceId: string, enabled: boolean) => (enabled ? mocks.summary() : { isLoading: false, isError: false }),
  useSupportAgents: () => ({ data: [{ id: 'echo', name: 'Echo' }] }),
}));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, search, className }: { children: React.ReactNode; search?: Record<string, string>; className?: string }) => (
    <a className={className} data-search={JSON.stringify(search ?? {})}>{children}</a>
  ),
}));
vi.mock('../SettingsAutosaveGuard', () => ({ SettingsAutosaveGuard: () => null }));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  vi.useFakeTimers();
  mocks.save.mockClear();
  mocks.summary.mockClear();
  mocks.query.mockReturnValue({
    data: { settings: {
      portal_enabled: false,
      portal_intake_enabled: true,
      portal_anonymous_intake_enabled: false,
      widget_name: 'A separate widget setting',
    } },
    isError: false,
    refetch: vi.fn(),
  });
  mocks.summary.mockReturnValue({
    data: { access_mode: 'approved_contacts', allowed_contacts: 3, conflicts: 0, conflict_emails: [], anonymous_intake_delivery_available: true },
    isLoading: false,
    isError: false,
  });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.useRealTimers();
});

describe('CustomerPortalSettings', () => {
  it('saves only portal fields and keeps the current portal address visible', async () => {
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable />));
    const link = container.querySelector<HTMLAnchorElement>('a[href$="/portal/acme"]');
    expect(link?.textContent).toContain('Open portal');
    expect(mocks.save).not.toHaveBeenCalled();

    const toggle = container.querySelector<HTMLButtonElement>('#portal-enabled');
    expect(toggle).not.toBeNull();
    await act(async () => toggle?.click());
    await act(async () => { await vi.advanceTimersByTimeAsync(800); });

    expect(mocks.save).toHaveBeenCalledTimes(1);
    expect(mocks.save).toHaveBeenCalledWith({
      portal_enabled: true,
      portal_intake_enabled: true,
      portal_anonymous_intake_enabled: false,
      portal_access_mode: 'approved_contacts',
      portal_ai_mode: 'off',
      portal_ai_agent_id: '',
    });
  });

  it('defaults to approved contacts and saves a changed access mode', async () => {
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable />));
    expect(container.querySelector<HTMLInputElement>('#portal-access-approved_contacts')?.checked).toBe(true);
    await act(async () => container.querySelector<HTMLInputElement>('#portal-access-any_verified_email')?.click());
    await act(async () => { await vi.advanceTimersByTimeAsync(800); });
    expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ portal_access_mode: 'any_verified_email' }));
  });

  it('shows allowed contacts and only real conflicts', async () => {
    mocks.summary.mockReturnValue({
      data: { access_mode: 'approved_contacts', allowed_contacts: 3, conflicts: 1, conflict_emails: ['dup@example.com'], anonymous_intake_delivery_available: true },
      isLoading: false,
      isError: false,
    });
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable />));
    expect(container.textContent).toContain('3 contacts are allowed to sign in.');
    const allowedLink = Array.from(container.querySelectorAll('a')).find((link) => link.textContent === 'View allowed contacts');
    expect(allowedLink?.dataset.search).toContain('portal_access');
    expect(container.textContent).toContain('1 email needs attention');
    const conflictLink = Array.from(container.querySelectorAll('a')).find((link) => link.textContent === 'dup@example.com');
    expect(JSON.parse(conflictLink?.dataset.search ?? '{}')).toEqual({ search: 'dup@example.com' });
  });

  it('keeps requests without sign-in off when agents cannot reply by email', async () => {
    mocks.summary.mockReturnValue({
      data: { access_mode: 'approved_contacts', allowed_contacts: 0, conflicts: 0, conflict_emails: [], anonymous_intake_delivery_available: false },
      isLoading: false,
      isError: false,
    });
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable />));
    expect(container.querySelector<HTMLButtonElement>('#portal-anonymous-intake')?.disabled).toBe(true);
    expect(container.textContent).toContain('Requests without sign-in need outbound email');
  });

  it('shows the settings without allowing edits for a non-admin', async () => {
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable={false} />));
    expect(container.textContent).toContain('A workspace support administrator can change these settings.');
    expect(container.querySelector<HTMLButtonElement>('#portal-enabled')?.disabled).toBe(true);
    expect(container.querySelector<HTMLInputElement>('#portal-access-approved_contacts')?.closest('fieldset')?.disabled).toBe(true);
    expect(mocks.summary).not.toHaveBeenCalled();
  });
});
