import { describe, expect, it } from 'vitest';
import {
  buildConversationListRequestFilters,
  buildSupportInboxViewFilters,
  defaultAssignmentForNav,
  defaultAIStatesForNav,
  defaultConversationListFiltersForNav,
  filterSupportConversations,
  hasConversationListChanges,
  parseSupportInboxViewFilters,
  supportInboxCountMailboxScope,
} from '../supportInboxFilters';
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
      aiStates: defaultConversationListFiltersForNav('inbox').aiStates,
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['human', 'ai-escalated', 'waiting']);
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
    }).map((conversation) => conversation.id)).toEqual(['resolved', 'ai-resolved']);

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

  it('treats an empty AI state selection as no AI state', () => {
    const conversations = [
      buildConversation({ id: 'human' }),
      buildConversation({ id: 'ai-pending', flow_state: 'ai_handling', ai_state: 'pending' }),
      buildConversation({ id: 'ai-resolved', flow_state: 'resolved_by_ai', ai_state: 'resolved' }),
      buildConversation({ id: 'ai-escalated', flow_state: 'waiting_for_human', ai_state: 'escalated' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'inbox',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
      aiStates: [],
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['human']);
  });

  it('can include AI handling in Inbox without removing normal conversations', () => {
    const conversations = [
      buildConversation({ id: 'human' }),
      buildConversation({ id: 'ai-pending', flow_state: 'ai_handling', ai_state: 'pending' }),
      buildConversation({ id: 'ai-resolved', flow_state: 'resolved_by_ai', ai_state: 'resolved' }),
      buildConversation({ id: 'ai-escalated', flow_state: 'waiting_for_human', ai_state: 'escalated' }),
    ];

    const result = filterSupportConversations(conversations, {
      navFilter: 'inbox',
      mailboxScope: 'all',
      userId: 'user-1',
      searchQuery: '',
      aiStates: ['handoff', 'handling'],
    });

    expect(result.map((conversation) => conversation.id)).toEqual(['human', 'ai-pending', 'ai-escalated']);
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
    expect(defaultAIStatesForNav('inbox')).toEqual(['handoff']);
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('inbox'),
    })).toEqual({ filter: 'inbox', mailbox_id: 'shared' });
  });

  it('adds assignment and sort refinements without changing the sidebar view', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'mailbox-billing',
      searchQuery: 'refund',
      listFilters: { ...defaultConversationListFiltersForNav('inbox'), assignment: ['unassigned'], sort: 'oldest' },
    })).toEqual({
      filter: 'inbox',
      mailbox_id: 'mailbox-billing',
      search: 'refund',
      assigned_to: 'unassigned',
      sort: 'oldest',
    });
  });

  it('sends multi-select assignment and team inbox filters as CSV lists', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: {
        ...defaultConversationListFiltersForNav('inbox'),
        assignment: ['me', 'unassigned'],
        mailboxIds: ['mailbox-billing', 'mailbox-sales'],
      },
    })).toEqual({
      filter: 'inbox',
      assigned_to: 'me,unassigned',
      mailbox_ids: 'mailbox-billing,mailbox-sales',
    });
  });

  it('broadens the Inbox request when non-default AI state filters need non-human-inbox rows', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: {
        ...defaultConversationListFiltersForNav('inbox'),
        tagIds: ['tag-billing', 'tag-vip'],
        aiStates: ['handoff', 'handling'],
      },
    })).toEqual({
      mailbox_id: 'shared',
      statuses: 'open,waiting_on_customer',
      tag_ids: 'tag-billing,tag-vip',
    });
  });

  it('uses explicit multi-state filters when the visible state selection changes', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('inbox'), states: ['open'] },
    })).toEqual({
      mailbox_id: 'shared',
      statuses: 'open',
    });
  });

  it('shows Mine globally as Open plus Waiting without changing the backend mine filter', () => {
    expect(defaultAssignmentForNav('mine')).toEqual(['me', 'mentioned_me', 'opened_by_me']);
    expect(buildConversationListRequestFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('mine'),
    })).toEqual({ filter: 'mine' });
  });

  it('only sends Mine assignment filters after they differ from the visible Mine defaults', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('mine'), assignment: ['mentioned_me'] },
    })).toEqual({ filter: 'mine', assigned_to: 'mentioned_me' });

    expect(buildConversationListRequestFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('mine'), assignment: [] },
    })).toEqual({ filter: 'mine' });
  });

  it('shows non-inbox sidebar state views across all inboxes by default', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'waiting',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('waiting'),
    })).toEqual({ status: 'waiting_on_customer' });

    expect(buildConversationListRequestFilters({
      navFilter: 'ai_active',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('ai_active'),
    })).toEqual({ ai: 'handling' });
  });

  it('uses the visible AI state selection for AI sidebar views', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'resolved_by_ai',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('resolved_by_ai'),
    })).toEqual({ ai: 'resolved' });

    expect(buildConversationListRequestFilters({
      navFilter: 'ai_active',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('ai_active'), aiStates: ['handoff', 'resolved'] },
    })).toEqual({ ai: 'handoff,resolved' });
  });

  it('allows Inbox to be explicitly expanded to all inboxes', () => {
    expect(buildConversationListRequestFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('inbox'), mailboxIds: ['all'] },
    })).toEqual({ filter: 'inbox' });
  });
});

describe('supportInboxCountMailboxScope', () => {
  it('uses the same mailbox scope as the default Inbox list', () => {
    expect(supportInboxCountMailboxScope('all')).toBe('shared');
    expect(supportInboxCountMailboxScope('mailbox-billing')).toBe('mailbox-billing');
  });
});

describe('hasConversationListChanges', () => {
  it('does not treat the selected sidebar team inbox as a filter change', () => {
    expect(hasConversationListChanges({
      navFilter: 'inbox',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('inbox'),
    })).toBe(false);
  });

  it('treats team inbox selections inside the filter popover as filter changes', () => {
    expect(hasConversationListChanges({
      navFilter: 'inbox',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('inbox'), mailboxIds: ['mailbox-sales'] },
    })).toBe(true);
  });
});

describe('buildSupportInboxViewFilters', () => {
  it('round-trips the default Inbox AI handoff baseline without persisting it', () => {
    const filters = buildSupportInboxViewFilters({
      navFilter: 'inbox',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('inbox'),
    });

    expect(filters).toEqual({
      nav_filter: 'inbox',
      states: 'open,waiting_on_customer',
    });
    expect(parseSupportInboxViewFilters(filters, 'inbox').listFilters.aiStates).toEqual(['handoff']);
  });

  it('does not persist default Mine assignment chips unless the user changes them', () => {
    expect(buildSupportInboxViewFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: defaultConversationListFiltersForNav('mine'),
    })).toEqual({
      nav_filter: 'mine',
      states: 'open,waiting_on_customer',
    });

    expect(buildSupportInboxViewFilters({
      navFilter: 'mine',
      selectedMailboxId: 'all',
      searchQuery: '',
      listFilters: { ...defaultConversationListFiltersForNav('mine'), assignment: ['me'] },
    })).toMatchObject({
      assignment: 'me',
    });
  });
});
