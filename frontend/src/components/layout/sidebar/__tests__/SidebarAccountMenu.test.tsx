// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SidebarAccountMenu } from '../SidebarAccountMenu';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('SidebarAccountMenu', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    document.body.innerHTML = '';
  });

  it('opens Helpin from Get Help immediately above Sign out', async () => {
    const onGetHelp = vi.fn();
    await act(async () => {
      root.render(
        <SidebarAccountMenu
          user={{ full_name: 'Ada Lovelace', email: 'ada@example.com' }}
          initials="AL"
          presence={null}
          selectedPresenceMode="auto"
          onPresenceChange={vi.fn()}
          onProfile={vi.fn()}
          onSettings={vi.fn()}
          onWorkspaces={vi.fn()}
          onGetHelp={onGetHelp}
          onSignOut={vi.fn()}
        />,
      );
    });

    const trigger = container.querySelector<HTMLButtonElement>('[aria-label="Account menu"]');
    await act(async () => {
      trigger?.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, ctrlKey: false }));
    });

    const items = Array.from(document.body.querySelectorAll<HTMLElement>('[role="menuitem"]'));
    const getHelpIndex = items.findIndex((item) => item.textContent?.includes('Get Help'));
    const signOutIndex = items.findIndex((item) => item.textContent?.includes('Sign out'));
    expect(getHelpIndex).toBeGreaterThanOrEqual(0);
    expect(signOutIndex).toBe(getHelpIndex + 1);

    await act(async () => items[getHelpIndex]?.click());
    expect(onGetHelp).toHaveBeenCalledOnce();
  });
});
