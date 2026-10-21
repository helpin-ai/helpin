// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SupportConversation } from '@/lib/pmTypes';

const mocks = vi.hoisted(() => ({
  permissions: new Set<string>(),
  mutate: vi.fn(),
}));

vi.mock('@/hooks/queries/useSession', () => ({
  useWorkspaceAccess: () => ({ data: {} }),
  usePermissions: () => ({ has: (permission: string) => mocks.permissions.has(permission) }),
}));
vi.mock('@/hooks/queries/useSupport', () => ({
  useResendPortalConfirmation: () => ({ mutate: mocks.mutate, isPending: false }),
}));

import { SidebarPortalConfirmation } from '../SidebarPortalConfirmation';
import { isHiddenPortalIntake } from '../portalIntake';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

const hiddenIntake = { id: 'conv-1', channel: 'portal', source: 'portal', portal_visible: false } as SupportConversation;

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  mocks.permissions = new Set(['support.admin']);
  mocks.mutate.mockReset();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('SidebarPortalConfirmation', () => {
  it('only targets unconfirmed anonymous portal requests', () => {
    expect(isHiddenPortalIntake(hiddenIntake)).toBe(true);
    expect(isHiddenPortalIntake({ ...hiddenIntake, portal_visible: true })).toBe(false);
    expect(isHiddenPortalIntake({ ...hiddenIntake, portal_visibility_changed_at: '2026-09-27T00:00:00Z' })).toBe(false);
    expect(isHiddenPortalIntake({ ...hiddenIntake, channel: 'email', source: 'email' })).toBe(false);
    expect(isHiddenPortalIntake({ ...hiddenIntake, anonymized_at: '2026-09-27T00:00:00Z' })).toBe(false);
  });

  it('sends a confirmation for this request', async () => {
    await act(async () => root.render(<SidebarPortalConfirmation workspaceId="ws" conversation={hiddenIntake} />));
    const button = Array.from(container.querySelectorAll('button')).find((item) => item.textContent === 'Send portal confirmation');
    await act(async () => button?.click());
    expect(mocks.mutate).toHaveBeenCalledWith('conv-1');
  });

  it('is hidden from people who are not support admins', async () => {
    mocks.permissions = new Set(['support.edit']);
    await act(async () => root.render(<SidebarPortalConfirmation workspaceId="ws" conversation={hiddenIntake} />));
    expect(container.textContent).toBe('');
  });
});
