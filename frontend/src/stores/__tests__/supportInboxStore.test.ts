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
    navFilter: 'inbox',
    selectedMailboxId: 'all',
    statusFilter: 'all',
    searchQuery: '',
    conversationListFilters: {
      states: ['open'],
      assignment: 'default',
      ai: 'default',
      sort: 'newest',
    },
    activeCustomViewId: null,
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

  it('returns to the global inbox view when switching queue filters', () => {
    useSupportInboxStore.setState({ selectedMailboxId: 'mailbox-billing' });

    useSupportInboxStore.getState().setNavFilter('ai_active');

    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('all');
  });

  it('opens a team inbox in the active inbox view', () => {
    useSupportInboxStore.setState({ navFilter: 'mine', statusFilter: 'waiting_on_customer' });

    useSupportInboxStore.getState().setSelectedMailboxId('mailbox-billing');

    expect(useSupportInboxStore.getState().navFilter).toBe('inbox');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
    expect(useSupportInboxStore.getState().statusFilter).toBe('all');
  });

  it('can refine the current sidebar view by mailbox without changing the view', () => {
    useSupportInboxStore.setState({ navFilter: 'waiting', selectedMailboxId: 'all' });

    useSupportInboxStore.getState().setMailboxFilter('mailbox-billing');

    expect(useSupportInboxStore.getState().navFilter).toBe('waiting');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
  });

  it('resets conversation list filters when switching sidebar views', () => {
    useSupportInboxStore.setState({
      conversationListFilters: {
        states: ['open'],
        assignment: 'unassigned',
        ai: 'ai_handling',
        sort: 'oldest',
      },
    });

    useSupportInboxStore.getState().setNavFilter('resolved');

    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['resolved'],
      assignment: 'default',
      ai: 'default',
      sort: 'newest',
    });
  });

  it('applies a custom support view to the list state', () => {
    useSupportInboxStore.getState().applyCustomView({
      id: 'view-1',
      filters: {
        nav_filter: 'waiting',
        states: 'open,waiting_on_customer',
        mailbox_id: 'mailbox-billing',
        assignment: 'unassigned',
        ai: 'needs_human',
        sort: 'oldest',
        search: 'refund',
      },
    });

    expect(useSupportInboxStore.getState().activeCustomViewId).toBe('view-1');
    expect(useSupportInboxStore.getState().navFilter).toBe('waiting');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
    expect(useSupportInboxStore.getState().searchQuery).toBe('refund');
    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['open', 'waiting_on_customer'],
      assignment: 'unassigned',
      ai: 'needs_human',
      sort: 'oldest',
    });
  });

  it('clears the active custom view when filters are manually changed', () => {
    useSupportInboxStore.setState({ activeCustomViewId: 'view-1' });

    useSupportInboxStore.getState().setConversationListFilter('assignment', 'me');

    expect(useSupportInboxStore.getState().activeCustomViewId).toBeNull();
  });

  it('syncs route state without view and inbox actions resetting each other', () => {
    useSupportInboxStore.getState().syncRouteState({
      navFilter: 'mine',
      selectedMailboxId: 'mailbox-billing',
      statusFilter: 'all',
      searchQuery: 'refund',
    });

    expect(useSupportInboxStore.getState().navFilter).toBe('mine');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
    expect(useSupportInboxStore.getState().searchQuery).toBe('refund');
  });
});
