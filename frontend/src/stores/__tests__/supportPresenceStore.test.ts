import { describe, it, expect, beforeEach } from 'vitest';
import { useSupportPresenceStore } from '../supportPresenceStore';

function resetStore() {
  useSupportPresenceStore.setState({
    typingIndicators: {},
    agentTyping: {},
    viewingAgents: {},
    wsSend: null,
    wsConnected: false,
  });
}

describe('supportPresenceStore', () => {
  beforeEach(() => {
    resetStore();
  });

  // ── setTyping (customer typing) ────────────────────────────

  describe('setTyping', () => {
    it('sets typing with content', () => {
      useSupportPresenceStore.getState().setTyping('conv-1', true, 'Hello');
      expect(useSupportPresenceStore.getState().typingIndicators['conv-1']).toBe('Hello');
    });

    it('sets typing with empty content (still truthy string)', () => {
      useSupportPresenceStore.getState().setTyping('conv-1', true);
      expect(useSupportPresenceStore.getState().typingIndicators['conv-1']).toBe('');
    });

    it('clears typing', () => {
      useSupportPresenceStore.getState().setTyping('conv-1', true, 'Hello');
      useSupportPresenceStore.getState().setTyping('conv-1', false);
      expect(useSupportPresenceStore.getState().typingIndicators['conv-1']).toBe(false);
    });

    it('handles multiple conversations independently', () => {
      const { setTyping } = useSupportPresenceStore.getState();
      setTyping('conv-1', true, 'typing in 1');
      setTyping('conv-2', true, 'typing in 2');
      const indicators = useSupportPresenceStore.getState().typingIndicators;
      expect(indicators['conv-1']).toBe('typing in 1');
      expect(indicators['conv-2']).toBe('typing in 2');
    });
  });

  // ── setAgentTyping (multi-agent) ───────────────────────────

  describe('setAgentTyping', () => {
    it('adds an agent typing entry', () => {
      useSupportPresenceStore.getState().setAgentTyping('conv-1', 'agent-a', 'drafting reply');
      const map = useSupportPresenceStore.getState().agentTyping['conv-1'];
      expect(map).toEqual({ 'agent-a': 'drafting reply' });
    });

    it('supports multiple agents typing in the same conversation', () => {
      const { setAgentTyping } = useSupportPresenceStore.getState();
      setAgentTyping('conv-1', 'agent-a', 'reply A');
      setAgentTyping('conv-1', 'agent-b', 'reply B');
      const map = useSupportPresenceStore.getState().agentTyping['conv-1'];
      expect(map).toEqual({ 'agent-a': 'reply A', 'agent-b': 'reply B' });
    });

    it('clears all agents when actorId is null', () => {
      const { setAgentTyping } = useSupportPresenceStore.getState();
      setAgentTyping('conv-1', 'agent-a', 'text');
      setAgentTyping('conv-1', 'agent-b', 'text');
      setAgentTyping('conv-1', null);
      expect(useSupportPresenceStore.getState().agentTyping['conv-1']).toEqual({});
    });

    it('clearAll is no-op when already empty', () => {
      const before = useSupportPresenceStore.getState().agentTyping;
      useSupportPresenceStore.getState().setAgentTyping('conv-1', null);
      const after = useSupportPresenceStore.getState().agentTyping;
      expect(after).toBe(before);
    });

    it('defaults content to empty string when omitted', () => {
      useSupportPresenceStore.getState().setAgentTyping('conv-1', 'agent-a');
      expect(useSupportPresenceStore.getState().agentTyping['conv-1']['agent-a']).toBe('');
    });
  });

  // ── clearOneAgentTyping ────────────────────────────────────

  describe('clearOneAgentTyping', () => {
    it('removes a single agent from the typing map', () => {
      const { setAgentTyping, clearOneAgentTyping } = useSupportPresenceStore.getState();
      setAgentTyping('conv-1', 'agent-a', 'A');
      setAgentTyping('conv-1', 'agent-b', 'B');
      clearOneAgentTyping('conv-1', 'agent-a');
      const map = useSupportPresenceStore.getState().agentTyping['conv-1'];
      expect(map).toEqual({ 'agent-b': 'B' });
    });

    it('is a no-op when the agent is not in the map', () => {
      useSupportPresenceStore.getState().setAgentTyping('conv-1', 'agent-a', 'text');
      const before = useSupportPresenceStore.getState().agentTyping;
      useSupportPresenceStore.getState().clearOneAgentTyping('conv-1', 'nonexistent');
      const after = useSupportPresenceStore.getState().agentTyping;
      expect(after).toBe(before);
    });

    it('is a no-op when conversation has no typing state', () => {
      const before = useSupportPresenceStore.getState().agentTyping;
      useSupportPresenceStore.getState().clearOneAgentTyping('no-conv', 'agent-a');
      const after = useSupportPresenceStore.getState().agentTyping;
      expect(after).toBe(before);
    });
  });

  // ── setViewingAgent (add/remove/no-op) ─────────────────────

  describe('setViewingAgent', () => {
    it('adds an agent to the viewing list', () => {
      useSupportPresenceStore.getState().setViewingAgent('conv-1', 'agent-a', true);
      expect(useSupportPresenceStore.getState().viewingAgents['conv-1']).toEqual(['agent-a']);
    });

    it('does not duplicate when adding an already-viewing agent', () => {
      const { setViewingAgent } = useSupportPresenceStore.getState();
      setViewingAgent('conv-1', 'agent-a', true);
      const before = useSupportPresenceStore.getState().viewingAgents;
      setViewingAgent('conv-1', 'agent-a', true);
      const after = useSupportPresenceStore.getState().viewingAgents;
      expect(after).toBe(before); // reference equality — no state change
      expect(after['conv-1']).toEqual(['agent-a']);
    });

    it('removes an agent from the viewing list', () => {
      const { setViewingAgent } = useSupportPresenceStore.getState();
      setViewingAgent('conv-1', 'agent-a', true);
      setViewingAgent('conv-1', 'agent-b', true);
      setViewingAgent('conv-1', 'agent-a', false);
      expect(useSupportPresenceStore.getState().viewingAgents['conv-1']).toEqual(['agent-b']);
    });

    it('is a no-op when removing an agent that is not viewing', () => {
      const before = useSupportPresenceStore.getState().viewingAgents;
      useSupportPresenceStore.getState().setViewingAgent('conv-1', 'agent-x', false);
      const after = useSupportPresenceStore.getState().viewingAgents;
      expect(after).toBe(before);
    });

    it('supports multiple conversations independently', () => {
      const { setViewingAgent } = useSupportPresenceStore.getState();
      setViewingAgent('conv-1', 'agent-a', true);
      setViewingAgent('conv-2', 'agent-b', true);
      const viewing = useSupportPresenceStore.getState().viewingAgents;
      expect(viewing['conv-1']).toEqual(['agent-a']);
      expect(viewing['conv-2']).toEqual(['agent-b']);
    });
  });
});
