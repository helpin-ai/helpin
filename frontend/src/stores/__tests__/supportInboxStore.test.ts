import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { useSupportInboxStore } from '../supportInboxStore';

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value; }),
    removeItem: vi.fn((key: string) => { delete store[key]; }),
    clear: vi.fn(() => { store = {}; }),
    get _store() { return store; },
  };
})();

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock });

function resetStore() {
  useSupportInboxStore.setState({
    drafts: {},
    navFilter: 'all',
    statusFilter: 'open',
    searchQuery: '',
    selectedConversationId: null,
    replyMode: 'reply',
    createDialogOpen: false,
    activePanel: 'list',
  });
}

describe('supportInboxStore', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorageMock.clear();
    resetStore();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // ── setDraft / clearDraft ──────────────────────────────────

  describe('setDraft / clearDraft', () => {
    it('stores a draft for a conversation', () => {
      const { setDraft } = useSupportInboxStore.getState();
      setDraft('conv-1', 'Hello there');
      expect(useSupportInboxStore.getState().drafts['conv-1']).toBe('Hello there');
    });

    it('removes draft when content is empty', () => {
      const { setDraft } = useSupportInboxStore.getState();
      setDraft('conv-1', 'something');
      setDraft('conv-1', '');
      expect(useSupportInboxStore.getState().drafts['conv-1']).toBeUndefined();
    });

    it('clearDraft removes a specific conversation draft', () => {
      const { setDraft, clearDraft } = useSupportInboxStore.getState();
      setDraft('conv-1', 'draft A');
      setDraft('conv-2', 'draft B');
      clearDraft('conv-1');
      const drafts = useSupportInboxStore.getState().drafts;
      expect(drafts['conv-1']).toBeUndefined();
      expect(drafts['conv-2']).toBe('draft B');
    });

    it('clearDraft is a no-op when draft does not exist', () => {
      const before = useSupportInboxStore.getState().drafts;
      useSupportInboxStore.getState().clearDraft('nonexistent');
      const after = useSupportInboxStore.getState().drafts;
      expect(after).toBe(before); // reference equality — no state change
    });

    it('persists drafts to localStorage after debounce', () => {
      useSupportInboxStore.getState().setDraft('conv-1', 'persisted text');
      // Before debounce fires, localStorage should not have been updated yet by saveDraftsDebounced
      vi.advanceTimersByTime(600);
      expect(localStorageMock.setItem).toHaveBeenCalledWith(
        'support_inbox_drafts',
        expect.stringContaining('persisted text')
      );
    });

    it('clearDraft removes from localStorage immediately', () => {
      // Seed localStorage
      localStorageMock.setItem('support_inbox_drafts', JSON.stringify({ 'conv-1': 'text' }));
      useSupportInboxStore.setState({ drafts: { 'conv-1': 'text' } });

      useSupportInboxStore.getState().clearDraft('conv-1');
      // removeDraftFromStorage is synchronous
      expect(localStorageMock.removeItem).toHaveBeenCalledWith('support_inbox_drafts');
    });
  });
});
