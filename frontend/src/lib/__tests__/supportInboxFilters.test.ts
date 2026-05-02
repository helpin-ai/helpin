import { describe, expect, it } from 'vitest';
import { buildConversationListRequestFilters, defaultConversationListFiltersForNav, filterSupportConversations } from '../supportInboxFilters';
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
  it('keeps only active human work in Inbox', () => {
    const conversations = [
      buildConversation({ id: 'human' }),
      buildConversation({ id: 'ai-pending', ai_state: 'pending' }),
      buildConversation({ id: 'ai-resolved', ai_state: 'resolved' }),
      buildConversation({ id: 'ai-escalated', ai_state: 'escalated' }),
      buildConversation({ id: 'waiting', status: 'waiting_on_customer' }),
      buildConversation({ id: 'resolved', status: 'resolved' }),
      buildConversation({ id: 'spam', status: 'spam' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'inbox',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['human', 'ai-escalated']);
  });

  it('keeps Mine actionable and excludes normal AI-owned work', () => {
    const conversations = [
      buildConversation({ id: 'assigned-human', assigned_user_id: 'user-1' }),
      buildConversation({ id: 'opened-waiting', status: 'waiting_on_customer', opened_by_user_id: 'user-1' }),
      buildConversation({ id: 'assigned-ai', assigned_user_id: 'user-1', ai_state: 'pending' }),
      buildConversation({ id: 'assigned-resolved', status: 'resolved', assigned_user_id: 'user-1' }),
      buildConversation({ id: 'other', assigned_user_id: 'user-2' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'mine',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['assigned-human', 'opened-waiting']);
  });

  it('shows status views without mixing them into Inbox', () => {
    const conversations = [
      buildConversation({ id: 'open' }),
      buildConversation({ id: 'waiting', status: 'waiting_on_customer' }),
      buildConversation({ id: 'resolved', status: 'resolved', flow_state: 'resolved_by_human' }),
      buildConversation({ id: 'ai-resolved', status: 'resolved', flow_state: 'resolved_by_ai', ai_state: 'resolved' }),
      buildConversation({ id: 'spam', status: 'spam' }),
    ];

    expect(filterSupportConversations(conversations, {
      navFilter: 'waiting',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    }).map((conversation) => conversation.id)).toEqual(['waiting']);

    expect(filterSupportConversations(conversations, {
      navFilter: 'resolved',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    }).map((conversation) => conversation.id)).toEqual(['resolved']);

    expect(filterSupportConversations(conversations, {
      navFilter: 'spam',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    }).map((conversation) => conversation.id)).toEqual(['spam']);
  });

  it('keeps AI views separate', () => {
    const conversations = [
      buildConversation({ id: 'human' }),
      buildConversation({ id: 'ai-pending', flow_state: 'ai_handling', ai_state: 'pending' }),
      buildConversation({ id: 'ai-resolved', flow_state: 'resolved_by_ai', ai_state: 'resolved' }),
      buildConversation({ id: 'ai-escalated', flow_state: 'waiting_for_human', ai_state: 'escalated' }),
    ];

    expect(filterSupportConversations(conversations, {
      navFilter: 'ai_active',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    }).map((conversation) => conversation.id)).toEqual(['ai-pending']);

    expect(filterSupportConversations(conversations, {
      navFilter: 'resolved_by_ai',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
    }).map((conversation) => conversation.id)).toEqual(['ai-resolved']);
  });

  it('applies the selected mailbox scope in every view', () => {
    const conversations = [
      buildConversation({ id: 'shared-conv', mailbox_id: null }),
      buildConversation({ id: 'billing-conv', mailbox_id: 'mailbox-billing' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'inbox',
      mailboxScope: 'mailbox-billing',
      userId: 'user-1',
      searchQuery: '',
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['billing-conv']);
  });
});

describe('buildConversationListRequestFilters', () => {
  it('keeps sidebar filters as the default request shape', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('inbox'),
    })).toEqual({ filter: 'inbox' });
  });

  it('adds assignment and sort refinements without changing the sidebar view', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      searchQuery: 'refund',
      listFilters: { ...defaultConversationListFiltersForNav('inbox'), assignment: 'unassigned', sort: 'oldest' },
    })).toEqual({
      filter: 'inbox',
      mailbox_id: 'mailbox-billing',
      search: 'refund',
      assigned_to: 'unassigned',
      sort: 'oldest',
    });
  });

  it('lets explicit AI filters replace conflicting sidebar constraints', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'resolved',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('resolved'), ai: 'ai_handling' },
    })).toEqual({ flow_state: 'ai_handling' });
  });

  it('uses explicit multi-state filters when the visible state selection changes', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('inbox'), states: ['open', 'waiting_on_customer'] },
    })).toEqual({
      statuses: 'open,waiting_on_customer',
    });
  });

  it('shows Mine as Open plus Waiting without changing the backend mine filter', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('mine'),
    })).toEqual({ filter: 'mine' });
  });
});
