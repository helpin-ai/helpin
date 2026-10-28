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
vi.mock('@/hooks/queries', () => ({
  useDocsSpaces: () => ({ data: [
    { id: 'space-public', name: 'Guides', slug: 'guides', type: 'external_capable' },
    { id: 'space-internal', name: 'Runbooks', slug: 'runbooks', type: 'internal' },
    { id: 'space-private', name: 'Customer guides', slug: 'customer-guides', type: 'external_capable', helpcenter_audience: 'customers' },
  ], isLoading: false }),
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
  it('offers only private AI suggestions and keeps a retired customer-reply mode as suggestions', async () => {
    mocks.query.mockReturnValue({
      data: { settings: { portal_enabled: true, portal_ai_mode: 'ai_first', ai_agent_id: 'echo' } },
      isError: false,
      refetch: vi.fn(),
    });
    await act(async () => root.render(<CustomerPortalSettings workspaceId="ws" workspaceSlug="acme" editable />));
    const trigger = container.querySelector<HTMLElement>('[aria-label="Portal AI response mode"]');
    expect(trigger?.textContent).toContain('Private suggestions');
    expect(container.textContent).toContain('Your team answers portal requests');
    expect(container.textContent).toContain('Customers never see them');
    expect(container.textContent).not.toContain('Reply to customers');
  });

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
      portal_show_reply_by: false,
      portal_access_mode: 'approved_contacts',
      portal_ai_mode: 'off',
      portal_ai_agent_id: '',
      widget_help_space_ids: [],
    });
  });

  it('turns on showing when to expect a reply', async () => {
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable />));
    await act(async () => container.querySelector<HTMLButtonElement>('#portal-show-reply-by')?.click());
    await act(async () => { await vi.advanceTimersByTimeAsync(800); });
    expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ portal_show_reply_by: true }));
  });

  it('shares the help space selection with the chat widget and says when suggestions are off', async () => {
    await act(async () => root.render(<CustomerPortalSettings workspaceId="ws" workspaceSlug="acme" editable />));
    expect(container.textContent).toContain('article suggestions are off');
    expect(container.querySelector('[aria-label="Offer Runbooks to customers"]')).toBeNull()
    // Spaces for signed-in customers never reach the anonymous widget.
    expect(container.querySelector('[aria-label="Offer Customer guides to customers"]')).toBeNull()
    expect(container.textContent).toContain('One space is for signed-in customers only.');
    const toggle = container.querySelector<HTMLButtonElement>('[aria-label="Offer Guides to customers"]');
    await act(async () => toggle?.click());
    await act(async () => vi.advanceTimersByTimeAsync(2000));
    expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ widget_help_space_ids: ['space-public'] }));
    expect(container.textContent).not.toContain('article suggestions are off');
  });

  it('offers sign-in for company members only', async () => {
    mocks.query.mockReturnValue({ data: { settings: { portal_enabled: true, portal_access_mode: 'company_members' } }, isError: false, refetch: vi.fn() })
    await act(async () => root.render(<CustomerPortalSettings workspaceId="ws" workspaceSlug="acme" editable />))
    const radio = container.querySelector<HTMLInputElement>('#portal-access-company_members')
    expect(radio?.checked).toBe(true)
    expect(container.textContent).toContain('Only allowed contacts who are members of a company with company access turned on can sign in.')
  })

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
