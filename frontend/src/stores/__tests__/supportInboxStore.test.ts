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
    typingIndicators: {},
    agentTyping: {},
    viewingAgents: {},
    drafts: {},
    navFilter: 'all',
    statusFilter: 'open',
    searchQuery: '',
    selectedConversationId: null,
    replyMode: 'reply',
    createDialogOpen: false,
    activePanel: 'list',
    wsSend: null,
    wsConnected: false,
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

  // ── setTyping (customer typing) ────────────────────────────

  describe('setTyping', () => {
    it('sets typing with content', () => {
      useSupportInboxStore.getState().setTyping('conv-1', true, 'Hello');
      expect(useSupportInboxStore.getState().typingIndicators['conv-1']).toBe('Hello');
    });

    it('sets typing with empty content (still truthy string)', () => {
      useSupportInboxStore.getState().setTyping('conv-1', true);
      expect(useSupportInboxStore.getState().typingIndicators['conv-1']).toBe('');
    });

    it('clears typing', () => {
      useSupportInboxStore.getState().setTyping('conv-1', true, 'Hello');
      useSupportInboxStore.getState().setTyping('conv-1', false);
      expect(useSupportInboxStore.getState().typingIndicators['conv-1']).toBe(false);
    });

    it('handles multiple conversations independently', () => {
      const { setTyping } = useSupportInboxStore.getState();
      setTyping('conv-1', true, 'typing in 1');
      setTyping('conv-2', true, 'typing in 2');
      const indicators = useSupportInboxStore.getState().typingIndicators;
      expect(indicators['conv-1']).toBe('typing in 1');
      expect(indicators['conv-2']).toBe('typing in 2');
    });
  });

  // ── setAgentTyping (multi-agent) ───────────────────────────

  describe('setAgentTyping', () => {
    it('adds an agent typing entry', () => {
      useSupportInboxStore.getState().setAgentTyping('conv-1', 'agent-a', 'drafting reply');
      const map = useSupportInboxStore.getState().agentTyping['conv-1'];
      expect(map).toEqual({ 'agent-a': 'drafting reply' });
    });

    it('supports multiple agents typing in the same conversation', () => {
      const { setAgentTyping } = useSupportInboxStore.getState();
      setAgentTyping('conv-1', 'agent-a', 'reply A');
      setAgentTyping('conv-1', 'agent-b', 'reply B');
      const map = useSupportInboxStore.getState().agentTyping['conv-1'];
      expect(map).toEqual({ 'agent-a': 'reply A', 'agent-b': 'reply B' });
    });

    it('clears all agents when actorId is null', () => {
      const { setAgentTyping } = useSupportInboxStore.getState();
      setAgentTyping('conv-1', 'agent-a', 'text');
      setAgentTyping('conv-1', 'agent-b', 'text');
      setAgentTyping('conv-1', null);
      expect(useSupportInboxStore.getState().agentTyping['conv-1']).toEqual({});
    });

    it('clearAll is no-op when already empty', () => {
      const before = useSupportInboxStore.getState().agentTyping;
      useSupportInboxStore.getState().setAgentTyping('conv-1', null);
      const after = useSupportInboxStore.getState().agentTyping;
      expect(after).toBe(before);
    });

    it('defaults content to empty string when omitted', () => {
      useSupportInboxStore.getState().setAgentTyping('conv-1', 'agent-a');
      expect(useSupportInboxStore.getState().agentTyping['conv-1']['agent-a']).toBe('');
    });
  });

  // ── clearOneAgentTyping ────────────────────────────────────

  describe('clearOneAgentTyping', () => {
    it('removes a single agent from the typing map', () => {
      const { setAgentTyping, clearOneAgentTyping } = useSupportInboxStore.getState();
      setAgentTyping('conv-1', 'agent-a', 'A');
      setAgentTyping('conv-1', 'agent-b', 'B');
      clearOneAgentTyping('conv-1', 'agent-a');
      const map = useSupportInboxStore.getState().agentTyping['conv-1'];
      expect(map).toEqual({ 'agent-b': 'B' });
    });

    it('is a no-op when the agent is not in the map', () => {
      useSupportInboxStore.getState().setAgentTyping('conv-1', 'agent-a', 'text');
      const before = useSupportInboxStore.getState().agentTyping;
      useSupportInboxStore.getState().clearOneAgentTyping('conv-1', 'nonexistent');
      const after = useSupportInboxStore.getState().agentTyping;
      expect(after).toBe(before);
    });

    it('is a no-op when conversation has no typing state', () => {
      const before = useSupportInboxStore.getState().agentTyping;
      useSupportInboxStore.getState().clearOneAgentTyping('no-conv', 'agent-a');
      const after = useSupportInboxStore.getState().agentTyping;
      expect(after).toBe(before);
    });
  });

  // ── setViewingAgent (add/remove/no-op) ─────────────────────

  describe('setViewingAgent', () => {
    it('adds an agent to the viewing list', () => {
      useSupportInboxStore.getState().setViewingAgent('conv-1', 'agent-a', true);
      expect(useSupportInboxStore.getState().viewingAgents['conv-1']).toEqual(['agent-a']);
    });

    it('does not duplicate when adding an already-viewing agent', () => {
      const { setViewingAgent } = useSupportInboxStore.getState();
      setViewingAgent('conv-1', 'agent-a', true);
      const before = useSupportInboxStore.getState().viewingAgents;
      setViewingAgent('conv-1', 'agent-a', true);
      const after = useSupportInboxStore.getState().viewingAgents;
      expect(after).toBe(before); // reference equality — no state change
      expect(after['conv-1']).toEqual(['agent-a']);
    });

    it('removes an agent from the viewing list', () => {
      const { setViewingAgent } = useSupportInboxStore.getState();
      setViewingAgent('conv-1', 'agent-a', true);
      setViewingAgent('conv-1', 'agent-b', true);
      setViewingAgent('conv-1', 'agent-a', false);
      expect(useSupportInboxStore.getState().viewingAgents['conv-1']).toEqual(['agent-b']);
    });

    it('is a no-op when removing an agent that is not viewing', () => {
      const before = useSupportInboxStore.getState().viewingAgents;
      useSupportInboxStore.getState().setViewingAgent('conv-1', 'agent-x', false);
      const after = useSupportInboxStore.getState().viewingAgents;
      expect(after).toBe(before);
    });

    it('supports multiple conversations independently', () => {
      const { setViewingAgent } = useSupportInboxStore.getState();
      setViewingAgent('conv-1', 'agent-a', true);
      setViewingAgent('conv-2', 'agent-b', true);
      const viewing = useSupportInboxStore.getState().viewingAgents;
      expect(viewing['conv-1']).toEqual(['agent-a']);
      expect(viewing['conv-2']).toEqual(['agent-b']);
    });
  });
});
