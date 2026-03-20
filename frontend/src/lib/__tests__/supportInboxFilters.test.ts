import { describe, expect, it } from 'vitest';
import { filterSupportConversations } from '../supportInboxFilters';
import type { SupportConversation } from '../pmTypes';

function buildConversation(overrides: Partial<SupportConversation>): SupportConversation {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    workspace_id: overrides.workspace_id ?? 'ws-1',
    display_id: overrides.display_id ?? 1,
    subject: overrides.subject ?? 'Conversation',
    status: overrides.status ?? 'open',
    priority: overrides.priority ?? 'medium',
    channel: overrides.channel ?? 'widget',
    source: overrides.source ?? 'widget',
    created_at: overrides.created_at ?? '2026-03-19T12:00:00Z',
    updated_at: overrides.updated_at ?? '2026-03-19T12:00:00Z',
    ...overrides,
  };
}

describe('filterSupportConversations', () => {
  it('keeps AI-managed conversations out of the human all view', () => {
    const conversations = [
      buildConversation({ id: 'human', subject: 'Human conversation' }),
      buildConversation({ id: 'ai-pending', subject: 'AI pending', ai_state: 'pending' }),
      buildConversation({ id: 'ai-resolved', subject: 'AI resolved', ai_state: 'resolved' }),
      buildConversation({ id: 'ai-escalated', subject: 'AI escalated', ai_state: 'escalated' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'all',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['human', 'ai-escalated']);
  });

  it('keeps AI-managed conversations out of my inbox but preserves escalated ones', () => {
    const conversations = [
      buildConversation({ id: 'human', opened_by_user_id: 'user-1' }),
      buildConversation({ id: 'ai-pending', opened_by_user_id: 'user-1', ai_state: 'pending' }),
      buildConversation({ id: 'ai-escalated', opened_by_user_id: 'user-1', ai_state: 'escalated' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'my_inbox',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['human', 'ai-escalated']);
  });

  it('shows only AI conversations in the Helpin AI view', () => {
    const conversations = [
      buildConversation({ id: 'human' }),
      buildConversation({ id: 'ai-pending', ai_state: 'pending' }),
      buildConversation({ id: 'ai-resolved', ai_state: 'resolved' }),
      buildConversation({ id: 'ai-escalated', ai_state: 'escalated' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'ai_all',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual([
      'ai-pending',
      'ai-resolved',
      'ai-escalated',
    ]);
  });

  it('treats escalated AI conversations as human-unassigned work', () => {
    const conversations = [
      buildConversation({ id: 'human-unassigned' }),
      buildConversation({ id: 'ai-pending', ai_state: 'pending' }),
      buildConversation({ id: 'ai-escalated', ai_state: 'escalated' }),
      buildConversation({ id: 'assigned-human', assigned_agent_id: 'agent-1' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'unassigned',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual([
      'human-unassigned',
      'ai-escalated',
    ]);
  });
});
