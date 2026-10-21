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
