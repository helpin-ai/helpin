// @vitest-environment jsdom
import { beforeEach, expect, it, vi } from 'vitest';

beforeEach(() => { sessionStorage.clear(); localStorage.clear(); vi.resetModules(); });

it('restores Ask agent text after a refresh, scopes it to its workspace, and clears sent drafts', async () => {
  const { useDockStore } = await import('../dockStore');
  useDockStore.getState().activateWorkspace('ws-1');
  useDockStore.getState().setDraft('chat:one', 'Unsent question');
  vi.resetModules();
  const { useDockStore: restored } = await import('../dockStore');
  restored.getState().activateWorkspace('ws-2');
  expect(restored.getState().drafts).toEqual({});
  restored.getState().activateWorkspace('ws-1');
  expect(restored.getState().drafts['chat:one']).toBe('Unsent question');
  restored.getState().clearDraft('chat:one');
  restored.getState().activateWorkspace('ws-2');
  restored.getState().activateWorkspace('ws-1');
  expect(restored.getState().drafts).toEqual({});
});

it('starts a new tab collapsed even when another tab saved an open dock', async () => {
  // Older versions stored visibility globally. Do not inherit that open state.
  localStorage.setItem('helpin:ask-agents-dock-collapsed', '0');
  const { useDockStore } = await import('../dockStore');
  expect(useDockStore.getState().collapsed).toBe(true);
  useDockStore.getState().activateWorkspace('ws-1');
  useDockStore.getState().setActiveChatId('chat-1');
  useDockStore.getState().setCollapsed(false);

  // A noopener link has its own session storage but shares local storage.
  sessionStorage.clear();
  vi.resetModules();
  const { useDockStore: newTab } = await import('../dockStore');
  newTab.getState().activateWorkspace('ws-1');
  expect(newTab.getState().collapsed).toBe(true);
  expect(newTab.getState().activeChatId).toBe('chat-1');
  expect(useDockStore.getState().collapsed).toBe(false);
});

it('restores dock visibility when refreshing the same tab', async () => {
  const { useDockStore } = await import('../dockStore');
  useDockStore.getState().setCollapsed(false);
  expect(sessionStorage.getItem('helpin:ask-agents-dock-collapsed')).toBe('0');
  expect(localStorage.getItem('helpin:ask-agents-dock-collapsed')).toBeNull();
  vi.resetModules();
  const { useDockStore: refreshed } = await import('../dockStore');
  expect(refreshed.getState().collapsed).toBe(false);
  refreshed.getState().setCollapsed(true);
  vi.resetModules();
  const { useDockStore: closed } = await import('../dockStore');
  expect(closed.getState().collapsed).toBe(true);
});
