// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CustomerPortalSettings } from '../CustomerPortalSettings';

const mocks = vi.hoisted(() => ({
  save: vi.fn().mockResolvedValue(undefined),
  query: vi.fn(),
}));

vi.mock('@/hooks/queries/useSupport', () => ({
  useChatSettings: () => mocks.query(),
  useUpdateCustomerPortalSettings: () => ({ mutateAsync: mocks.save }),
}));
vi.mock('../SettingsAutosaveGuard', () => ({ SettingsAutosaveGuard: () => null }));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let container: HTMLDivElement;

beforeEach(() => {
  vi.useFakeTimers();
  mocks.save.mockClear();
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
    });
  });

  it('shows the settings without allowing edits for a non-admin', async () => {
    await act(async () => root.render(<CustomerPortalSettings workspaceId="workspace-1" workspaceSlug="acme" editable={false} />));
    expect(container.textContent).toContain('A workspace support administrator can change these settings.');
    expect(container.querySelector<HTMLButtonElement>('#portal-enabled')?.disabled).toBe(true);
  });
});
