import { beforeEach, expect, it } from 'vitest';
import { useSupportPresenceStore } from '../support-presence-store';

beforeEach(() => useSupportPresenceStore.setState(useSupportPresenceStore.getInitialState()));

it('does not consider a connected socket proof of an offline visitor', () => {
  useSupportPresenceStore.getState().setWsConnected(true);
  expect(useSupportPresenceStore.getState().hasOnlineVisitorsSnapshot).toBe(false);
  useSupportPresenceStore.getState().setOnlineVisitors([]);
  expect(useSupportPresenceStore.getState().hasOnlineVisitorsSnapshot).toBe(true);
});

it('requires a fresh visitor snapshot after disconnecting', () => {
  useSupportPresenceStore.getState().setOnlineVisitors(['visitor-1']);
  useSupportPresenceStore.getState().setWsConnected(false);
  expect(useSupportPresenceStore.getState().hasOnlineVisitorsSnapshot).toBe(false);
  useSupportPresenceStore.getState().setWsConnected(true);
  expect(useSupportPresenceStore.getState().hasOnlineVisitorsSnapshot).toBe(false);
  useSupportPresenceStore.getState().setOnlineVisitors([]);
  expect(useSupportPresenceStore.getState().hasOnlineVisitorsSnapshot).toBe(true);
});
