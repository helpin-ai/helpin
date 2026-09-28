// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  access: vi.fn(),
  mutate: vi.fn(),
}));

vi.mock('@/hooks/queries/useSupport', () => ({
  useContactPortalAccess: (_workspaceId: string, _contactId: string, enabled: boolean) =>
    (enabled ? mocks.access() : { data: undefined, isLoading: false }),
  useSetContactPortalAccess: () => ({ mutate: mocks.mutate, isPending: false }),
}));
// A native select stands in for the Radix select, which jsdom cannot open.
vi.mock('@/components/design-system/quiet-dropdown-select', () => ({
  Select: ({ value, onValueChange, disabled, children }: { value: string; onValueChange: (value: string) => void; disabled?: boolean; children: React.ReactNode }) => (
    <select aria-label="Customer portal access" value={value} disabled={disabled} onChange={(event) => onValueChange(event.target.value)}>{children}</select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ value, children }: { value: string; children: React.ReactNode }) => <option value={value}>{children}</option>,
}));

import { ContactPortalAccessControl } from '../ContactPortalAccessControl';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  mocks.access.mockReset();
  mocks.mutate.mockReset();
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

describe('ContactPortalAccessControl', () => {
  it('is read-only and never calls the admin API for non-admins', async () => {
    await act(async () => root.render(<ContactPortalAccessControl workspaceId="ws" contactId="c1" currentAccess="blocked" canManage={false} />));
    expect(container.textContent).toBe('Blocked');
    expect(container.querySelector('select')).toBeNull();
    expect(mocks.access).not.toHaveBeenCalled();
  });

  it('lets support admins change access and clears it with Not set', async () => {
    mocks.access.mockReturnValue({ data: { portal_access: 'allowed', shared_email_contacts: 0 }, isLoading: false });
    await act(async () => root.render(<ContactPortalAccessControl workspaceId="ws" contactId="c1" canManage />));
    const select = container.querySelector<HTMLSelectElement>('select');
    expect(select?.value).toBe('allowed');
    await act(async () => change(select!, 'blocked'));
    expect(mocks.mutate).toHaveBeenLastCalledWith('blocked');
    await act(async () => change(select!, 'unset'));
    expect(mocks.mutate).toHaveBeenLastCalledWith(null);
  });

  it('warns when other contacts share the email', async () => {
    mocks.access.mockReturnValue({ data: { portal_access: 'allowed', shared_email_contacts: 2 }, isLoading: false });
    await act(async () => root.render(<ContactPortalAccessControl workspaceId="ws" contactId="c1" canManage />));
    expect(container.textContent).toContain('2 other contacts share this email');
  });
});
