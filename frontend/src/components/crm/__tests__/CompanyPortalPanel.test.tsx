// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CustomerPortalCompanyAccess, CustomerPortalCompanyContact } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({
  access: vi.fn(),
  refetch: vi.fn(),
  setCompanyRole: vi.fn(),
  allowAll: vi.fn(),
  setAccess: vi.fn(),
  setRole: vi.fn(),
}));

vi.mock('@/hooks/queries/useSupport', () => ({
  useCompanyPortalAccess: () => ({ ...mocks.access(), refetch: mocks.refetch }),
  useSetCompanyPortalRole: () => ({ mutate: mocks.setCompanyRole, isPending: false }),
  useAllowCompanyPortalContacts: () => ({ mutate: mocks.allowAll, isPending: false }),
  useSetContactPortalAccess: (_workspaceId: string, contactId: string) => ({ mutate: (value: unknown) => mocks.setAccess(contactId, value), isPending: false }),
  useSetContactPortalRole: (_workspaceId: string, contactId: string) => ({ mutate: (value: unknown) => mocks.setRole(contactId, value), isPending: false }),
}));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, className }: { children: React.ReactNode; className?: string }) => <a className={className} href="/settings">{children}</a>,
}));
// A native select stands in for the Radix select, which jsdom cannot open.
vi.mock('@/components/design-system/quiet-dropdown-select', () => {
  const SelectContext = React.createContext('');
  return {
    Select: ({ value, onValueChange, disabled, children }: { value: string; onValueChange: (value: string) => void; disabled?: boolean; children: React.ReactNode }) => {
      let label = '';
      React.Children.forEach(children, (child) => {
        if (React.isValidElement<{ 'aria-label'?: string }>(child) && child.props['aria-label']) label = child.props['aria-label'];
      });
      return (
        <SelectContext.Provider value={label}>
          <select aria-label={label} value={value} disabled={disabled} onChange={(event) => onValueChange(event.target.value)}>{children}</select>
        </SelectContext.Provider>
      );
    },
    SelectTrigger: () => null,
    SelectValue: () => null,
    SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    SelectItem: ({ value, children }: { value: string; children: React.ReactNode }) => <option value={value}>{children}</option>,
  };
});

import { CompanyPortalPanel } from '../CompanyPortalPanel';
import { allowableContacts, companyScopeSummary } from '../companyPortal';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let container: HTMLDivElement;

function contact(overrides: Partial<CustomerPortalCompanyContact>): CustomerPortalCompanyContact {
  return { id: 'c', first_name: 'Ada', last_name: 'Lovelace', email: 'ada@acme.test', portal_access: null, portal_role: null, shared_email_contacts: 0, ...overrides };
}

function companyAccess(overrides: Partial<CustomerPortalCompanyAccess> = {}): CustomerPortalCompanyAccess {
  return {
    default_role: 'member',
    access_mode: 'approved_contacts',
    portal_enabled: true,
    request_count: 4,
    contacts: [
      contact({ id: 'alice', first_name: 'Alice', email: 'alice@acme.test', portal_access: 'allowed', last_signed_in_at: new Date().toISOString() }),
      contact({ id: 'bob', first_name: 'Bob', email: 'bob@acme.test' }),
      contact({ id: 'blocked', first_name: 'Blocked', email: 'blocked@acme.test', portal_access: 'blocked' }),
      contact({ id: 'shared', first_name: 'Shared', email: 'shared@acme.test', shared_email_contacts: 1 }),
      contact({ id: 'none', first_name: 'No email', email: null }),
    ],
    ...overrides,
  };
}

beforeEach(() => {
  for (const mock of Object.values(mocks)) mock.mockReset();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function change(select: HTMLSelectElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set;
  setter?.call(select, value);
  select.dispatchEvent(new Event('change', { bubbles: true }));
}

const select = (label: string) => container.querySelector<HTMLSelectElement>(`select[aria-label="${label}"]`)!;
const render = () => act(async () => root.render(<CompanyPortalPanel workspaceId="ws" workspaceSlug="acme" companyId="co" />));

describe('allowableContacts', () => {
  it('matches the server rule: undecided contacts with an email of their own', () => {
    expect(allowableContacts(companyAccess().contacts).map((item) => item.id)).toEqual(['bob']);
  });
});

describe('companyScopeSummary', () => {
  it('explains each company setting', () => {
    expect(companyScopeSummary(companyAccess())).toContain('see and reply to all 4 requests');
    expect(companyScopeSummary(companyAccess({ default_role: 'requester', request_count: 1 }))).toContain('share all 1 request from');
    expect(companyScopeSummary(companyAccess({ default_role: null }))).toContain('would show members 4 requests');
  });
});

describe('CompanyPortalPanel', () => {
  it('changes the company setting and turns it off', async () => {
    mocks.access.mockReturnValue({ data: companyAccess(), isLoading: false, isError: false });
    await render();
    const company = select('Company requests in the portal');
    expect(company.value).toBe('member');
    await act(async () => change(company, 'requester'));
    expect(mocks.setCompanyRole).toHaveBeenLastCalledWith('requester');
    await act(async () => change(company, 'off'));
    expect(mocks.setCompanyRole).toHaveBeenLastCalledWith(null);
  });

  it('offers to allow only the contacts the server would allow', async () => {
    mocks.access.mockReturnValue({ data: companyAccess(), isLoading: false, isError: false });
    await render();
    const allow = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Allow 1 contact');
    expect(allow).toBeDefined();
    await act(async () => allow!.click());
    expect(mocks.allowAll).toHaveBeenCalledOnce();
  });

  it('manages each contact\'s sign-in and role', async () => {
    mocks.access.mockReturnValue({ data: companyAccess(), isLoading: false, isError: false });
    await render();
    await act(async () => change(select('Portal sign-in for Bob Lovelace'), 'allowed'));
    expect(mocks.setAccess).toHaveBeenLastCalledWith('bob', 'allowed');
    await act(async () => change(select('Portal sign-in for Alice Lovelace'), 'unset'));
    expect(mocks.setAccess).toHaveBeenLastCalledWith('alice', null);

    expect(select('Portal role for Bob Lovelace').disabled).toBe(true);
    const aliceRole = select('Portal role for Alice Lovelace');
    expect(aliceRole.disabled).toBe(false);
    expect(aliceRole.querySelector('option[value="default"]')?.textContent).toBe('Company default (Member)');
    await act(async () => change(aliceRole, 'requester'));
    expect(mocks.setRole).toHaveBeenLastCalledWith('alice', 'requester');
  });

  it('flags shared emails, missing emails and a disabled workspace portal', async () => {
    mocks.access.mockReturnValue({ data: companyAccess({ portal_enabled: false }), isLoading: false, isError: false });
    await render();
    expect(container.textContent).toContain('1 other contact shares this email');
    expect(container.textContent).toContain('No email, so this contact cannot sign in');
    expect(container.textContent).toContain('The customer portal is off for this workspace');
    expect(container.textContent).toContain('Never');
  });

  it('shows a working empty state without contacts', async () => {
    mocks.access.mockReturnValue({ data: companyAccess({ contacts: [] }), isLoading: false, isError: false });
    await render();
    expect(container.textContent).toContain('No contacts at this company');
    expect(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.startsWith('Allow'))).toBe(false);
  });

  it('retries after a load failure', async () => {
    mocks.access.mockReturnValue({ data: undefined, isLoading: false, isError: true });
    await render();
    const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Try again');
    await act(async () => retry!.click());
    expect(mocks.refetch).toHaveBeenCalledOnce();
  });
});
