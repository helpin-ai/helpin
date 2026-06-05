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
      assignment: [],
      mailboxIds: [],
      tagIds: [],
      aiStates: [],
      sort: 'newest',
    },
    activeCustomViewId: null,
    customViewDirty: false,
    builtinViewFilters: {},
    selectedConversationId: null,
    conversationHandoff: null,
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
        assignment: ['unassigned'],
        mailboxIds: [],
        tagIds: ['tag-billing'],
        aiStates: ['handoff'],
        sort: 'oldest',
      },
    });

    useSupportInboxStore.getState().setNavFilter('resolved');

    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['resolved'],
      assignment: [],
      mailboxIds: [],
      tagIds: [],
      aiStates: [],
      sort: 'newest',
    });
  });

  it('applies saved sidebar view filters when switching to that sidebar item', () => {
    useSupportInboxStore.getState().setBuiltinViewFilter('nav:waiting', {
      nav_filter: 'waiting',
      states: 'open,waiting_on_customer',
      search: 'refund',
      assignment: 'unassigned',
      tag_ids: 'tag-billing',
      ai: 'handoff',
      sort: 'oldest',
    });

    useSupportInboxStore.getState().setNavFilter('waiting');

    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('all');
    expect(useSupportInboxStore.getState().searchQuery).toBe('refund');
    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['open', 'waiting_on_customer'],
      assignment: ['unassigned'],
      mailboxIds: [],
      tagIds: ['tag-billing'],
      aiStates: ['handoff'],
      sort: 'oldest',
    });
  });

  it('applies saved team inbox filters when opening that team inbox', () => {
    useSupportInboxStore.getState().setBuiltinViewFilter('team:mailbox-billing', {
      nav_filter: 'inbox',
      states: 'open,waiting_on_customer',
      search: 'invoice',
      mailbox_id: 'mailbox-billing',
      mailbox_ids: 'all',
      ai: 'handoff',
      sort: 'oldest',
    });

    useSupportInboxStore.getState().setSelectedMailboxId('mailbox-billing');

    expect(useSupportInboxStore.getState().navFilter).toBe('inbox');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
    expect(useSupportInboxStore.getState().searchQuery).toBe('invoice');
    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['open', 'waiting_on_customer'],
      assignment: ['me', 'mentioned_me', 'opened_by_me', 'unassigned', 'others'],
      mailboxIds: ['all'],
      tagIds: [],
      aiStates: ['handoff'],
      sort: 'oldest',
    });
  });

  it('keeps the matching team inbox active when reopening a selected resolved conversation', () => {
    useSupportInboxStore.setState({
      navFilter: 'resolved',
      selectedMailboxId: 'mailbox-billing',
      selectedConversationId: 'conv-1',
      activePanel: 'thread',
    });

    useSupportInboxStore.getState().showReopenedConversationInInbox('conv-1', 'mailbox-billing');

    expect(useSupportInboxStore.getState().navFilter).toBe('inbox');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1');
    expect(useSupportInboxStore.getState().conversationListFilters.states).toEqual(['open', 'waiting_on_customer']);
  });

  it('falls back to all inboxes when reopening a selected conversation from another team inbox', () => {
    useSupportInboxStore.setState({
      navFilter: 'resolved',
      selectedMailboxId: 'mailbox-billing',
      selectedConversationId: 'conv-1',
      activePanel: 'thread',
    });

    useSupportInboxStore.getState().showReopenedConversationInInbox('conv-1', 'mailbox-sales');

    expect(useSupportInboxStore.getState().navFilter).toBe('inbox');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('all');
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1');
  });

  it('tracks a conversation handoff while resolving and clears it after completion', () => {
    useSupportInboxStore.setState({ selectedConversationId: 'conv-1', activePanel: 'thread' });

    useSupportInboxStore.getState().startConversationHandoff('conv-1', 'conv-2');

    expect(useSupportInboxStore.getState().conversationHandoff).toEqual({
      fromConversationId: 'conv-1',
      toConversationId: 'conv-2',
    });
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-1');

    useSupportInboxStore.getState().finishConversationHandoff('conv-2');

    expect(useSupportInboxStore.getState().conversationHandoff).toBeNull();
    expect(useSupportInboxStore.getState().selectedConversationId).toBe('conv-2');
    expect(useSupportInboxStore.getState().activePanel).toBe('thread');
  });

  it('uses visible Assignment defaults for built-in sidebar items', () => {
    useSupportInboxStore.getState().setNavFilter('inbox');

    expect(useSupportInboxStore.getState().conversationListFilters.assignment).toEqual([
      'me',
      'mentioned_me',
      'opened_by_me',
      'unassigned',
      'others',
    ]);

    useSupportInboxStore.getState().setNavFilter('mine');

    expect(useSupportInboxStore.getState().conversationListFilters.assignment).toEqual([
      'me',
      'mentioned_me',
      'opened_by_me',
    ]);

    useSupportInboxStore.getState().setNavFilter('waiting');

    expect(useSupportInboxStore.getState().conversationListFilters.assignment).toEqual([]);
  });

  it('applies a custom support view to the list state', () => {
    useSupportInboxStore.getState().applyCustomView({
      id: 'view-1',
      filters: {
        nav_filter: 'waiting',
        states: 'open,waiting_on_customer',
        mailbox_id: 'mailbox-billing',
        assignment: 'unassigned',
        tag_ids: 'tag-billing,tag-vip',
        ai: 'handoff',
        sort: 'oldest',
        search: 'refund',
      },
    });

    expect(useSupportInboxStore.getState().activeCustomViewId).toBe('view-1');
    expect(useSupportInboxStore.getState().customViewDirty).toBe(false);
    expect(useSupportInboxStore.getState().navFilter).toBe('waiting');
    expect(useSupportInboxStore.getState().selectedMailboxId).toBe('mailbox-billing');
    expect(useSupportInboxStore.getState().searchQuery).toBe('refund');
    expect(useSupportInboxStore.getState().conversationListFilters).toEqual({
      states: ['open', 'waiting_on_customer'],
      assignment: ['unassigned'],
      mailboxIds: [],
      tagIds: ['tag-billing', 'tag-vip'],
      aiStates: ['handoff'],
      sort: 'oldest',
    });
  });

  it('marks the active custom view dirty when filters are manually changed', () => {
    useSupportInboxStore.setState({ activeCustomViewId: 'view-1' });

    useSupportInboxStore.getState().setConversationListFilter('assignment', ['me']);

    expect(useSupportInboxStore.getState().activeCustomViewId).toBe('view-1');
    expect(useSupportInboxStore.getState().customViewDirty).toBe(true);
  });

  it('can mark an updated custom view clean', () => {
    useSupportInboxStore.setState({ activeCustomViewId: 'view-1', customViewDirty: true });

    useSupportInboxStore.getState().markCustomViewClean();

    expect(useSupportInboxStore.getState().activeCustomViewId).toBe('view-1');
    expect(useSupportInboxStore.getState().customViewDirty).toBe(false);
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
