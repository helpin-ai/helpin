// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CustomerPortalCompanyAccess, CustomerPortalCompanyContact } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({
  access: vi.fn(),
  refetch: vi.fn(),
  setCompanyRole: vi.fn(),
  addDomain: vi.fn(),
  setMember: vi.fn(),
  setAccess: vi.fn(),
  setRole: vi.fn(),
  setTeamSettings: vi.fn(),
  setCompanyRole2: vi.fn(),
}));

vi.mock('@/hooks/queries/useSupport', () => ({
  useCompanyPortalAccess: () => ({ ...mocks.access(), refetch: mocks.refetch }),
  useSetCompanyPortalRole: () => ({ mutate: mocks.setCompanyRole, isPending: false }),
  useAddCompanyPortalDomainMembers: () => ({ mutate: mocks.addDomain, isPending: false }),
  useSetCompanyPortalMember: () => ({ mutate: mocks.setMember, isPending: false }),
  useSetCompanyTeamSettings: () => ({ mutate: mocks.setTeamSettings, isPending: false }),
  useSetCompanyMemberRole: () => ({ mutate: mocks.setCompanyRole2, isPending: false }),
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
import { companyScopeSummary, domainAddableContacts, emailOnDomain } from '../companyPortal';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let container: HTMLDivElement;

function contact(overrides: Partial<CustomerPortalCompanyContact>): CustomerPortalCompanyContact {
  return { id: 'c', first_name: 'Ada', last_name: 'Lovelace', email: 'ada@acme.test', portal_access: null, portal_role: null, member: false, shared_email_contacts: 0, ...overrides };
}

function companyAccess(overrides: Partial<CustomerPortalCompanyAccess> = {}): CustomerPortalCompanyAccess {
  return {
    default_role: 'member',
    domain: 'https://www.acme.test',
    access_mode: 'approved_contacts',
    portal_enabled: true,
    request_count: 4,
    contacts: [
      contact({ id: 'alice', first_name: 'Alice', email: 'alice@acme.test', portal_access: 'allowed', member: true, last_signed_in_at: new Date().toISOString() }),
      contact({ id: 'bob', first_name: 'Bob', email: 'bob@acme.test' }),
      contact({ id: 'eve', first_name: 'Eve', email: 'eve@attacker.test' }),
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

describe('domainAddableContacts', () => {
  it('matches the server rule: non-members on the company domain with an email of their own', () => {
    expect(domainAddableContacts(companyAccess().contacts, 'https://www.acme.test').map((item) => item.id)).toEqual(['bob']);
    expect(domainAddableContacts(companyAccess().contacts, null)).toEqual([]);
  });

  it('accepts the domain and its subdomains only', () => {
    expect(emailOnDomain('ops@eu.acme.test', 'acme.test')).toBe(true);
    expect(emailOnDomain('ALICE@Acme.test', 'acme.test/')).toBe(true);
    expect(emailOnDomain('eve@notacme.test', 'acme.test')).toBe(false);
    expect(emailOnDomain('eve@acme.test.evil', 'acme.test')).toBe(false);
    expect(emailOnDomain('no-at-sign', 'acme.test')).toBe(false);
  });
});

describe('companyScopeSummary', () => {
  it('explains each company setting', () => {
    expect(companyScopeSummary(companyAccess())).toContain('see and reply to every request from this company (4 requests so far)');
    expect(companyScopeSummary(companyAccess({ default_role: 'requester', request_count: 1 }))).toContain('(1 request so far)');
    expect(companyScopeSummary(companyAccess({ default_role: null }))).toContain('would show members every request from this company (4 requests so far)');
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

  it('offers to add only the contacts on the company domain', async () => {
    mocks.access.mockReturnValue({ data: companyAccess(), isLoading: false, isError: false });
    await render();
    const add = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Add 1 contact from acme.test');
    expect(add).toBeDefined();
    await act(async () => add!.click());
    expect(mocks.addDomain).toHaveBeenCalledOnce();
  });

  it('asks for a domain before adding in bulk', async () => {
    mocks.access.mockReturnValue({ data: companyAccess({ domain: null }), isLoading: false, isError: false });
    await render();
    expect(container.textContent).toContain('Add a domain to this company');
    expect(Array.from(container.querySelectorAll('button')).some((button) => button.textContent?.startsWith('Add '))).toBe(false);
  });

  it('gives and removes company access per contact', async () => {
    mocks.access.mockReturnValue({ data: companyAccess(), isLoading: false, isError: false });
    await render();
    const bob = container.querySelector<HTMLButtonElement>('[aria-label="Company access for Bob Lovelace"]')!;
    expect(bob.getAttribute('aria-checked')).toBe('false');
    await act(async () => bob.click());
    expect(mocks.setMember).toHaveBeenLastCalledWith({ contactId: 'bob', member: true });
    const alice = container.querySelector<HTMLButtonElement>('[aria-label="Company access for Alice Lovelace"]')!;
    await act(async () => alice.click());
    expect(mocks.setMember).toHaveBeenLastCalledWith({ contactId: 'alice', member: false });
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
    expect(aliceRole.querySelector('option[value="default"]')?.textContent).toBe('Default (Member)');
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

describe('CompanyPortalPanel company admins', () => {
  it('turns admin invitations on and lists what admins changed', async () => {
    mocks.access.mockReturnValue({
      data: companyAccess({
        admin_invites: false,
        invite_any_domain: false,
        activity: [{ id: 'e1', event_type: 'invited', subject_email: 'dan@acme.test', after: 'member', actor_name: 'Alice', occurred_at: new Date().toISOString() }],
      }),
      isLoading: false,
      isError: false,
    });
    await render();
    const anyDomain = container.querySelector<HTMLButtonElement>('[aria-label="Company admins can invite any address"]')!;
    expect(anyDomain.disabled).toBe(true);
    act(() => container.querySelector<HTMLButtonElement>('[aria-label="Company admins can invite colleagues"]')!.click());
    expect(mocks.setTeamSettings).toHaveBeenCalledWith({ admin_invites: true, invite_any_domain: false });
    expect(container.textContent).toContain('Alice invited dan@acme.test as Member');
    expect(container.textContent).toContain('Off: only addresses on acme.test.');
  });
});
