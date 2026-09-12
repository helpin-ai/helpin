import { describe, expect, it } from 'vitest';
import {
  extractConversationListConversations,
  getNextConversationIdAfterRemoval,
  isSupportConversationListQueryKey,
  moveConversationToTopForMessageActivity,
  patchConversationStatusInCache,
  updateConversationListUnreadCount,
  updateConversationUnreadCount,
} from '@/lib/supportQueryCache';
import { filterSupportConversations } from '@/lib/supportInboxFilters';
import type { ConversationListResponse, SupportConversation } from '@/lib/pmTypes';

describe('supportQueryCache', () => {
  it('moves an Open thread in and out of Waiting for public replies, ignoring notes and activity', () => {
    let current = { data: [{ id: 'thread', workspace_id: 'ws-1', display_id: 1, subject: 'Thread', status: 'open', priority: 'medium', source: 'widget', created_at: '2026-09-11T09:00:00Z', updated_at: '2026-09-11T09:00:00Z' }], total: 1, page: 1, per_page: 50, total_pages: 1 } as ConversationListResponse;
    const waiting = () => filterSupportConversations(current.data, { navFilter: 'waiting', mailboxScope: 'all', searchQuery: '' }).length;
    for (const [index, [sender, internal, expected]] of ([
      ['customer', false, 0], ['user', false, 1], ['user', true, 1],
      ['customer', false, 0], ['user', true, 0], ['ai', false, 0], ['user', false, 1],
    ] as const).entries()) {
      current = moveConversationToTopForMessageActivity(current, {
        conversationId: 'thread', messageId: `message-${index}`, timestamp: `2026-09-11T09:0${index + 1}:00Z`,
        message: { sender_type: sender, message_type: 'reply', content: 'Message', is_internal: internal },
      }) as ConversationListResponse;
      expect(waiting()).toBe(expected);
      expect(current.data[0].status).toBe('open');
    }
    current = moveConversationToTopForMessageActivity(current, { conversationId: 'thread', timestamp: '2026-09-11T09:08:00Z', message: { sender_type: 'user', message_type: 'activity', system_event_type: 'assignment_changed' } }) as ConversationListResponse;
    expect(waiting()).toBe(1);
  });

  it('applies delayed public replies independently of newer internal notes', () => {
    const current = { data: [{ id: 'thread', status: 'open', last_message: 'Note: Investigating', list_last_message_at: '2026-09-11T09:03:00Z', list_last_activity_at: '2026-09-11T09:03:00Z', last_public_message_at: '2026-09-11T09:01:00Z', last_public_sender_type: 'user', customer_awaiting_response: false }], total: 1 } as ConversationListResponse;
    const updated = moveConversationToTopForMessageActivity(current, { conversationId: 'thread', messageId: 'customer-reply', timestamp: '2026-09-11T09:02:00Z', message: { sender_type: 'customer', message_type: 'reply', content: 'More details' } }) as ConversationListResponse;
    expect(updated.data[0]).toEqual(expect.objectContaining({ last_public_sender_type: 'customer', customer_awaiting_response: true, last_message: 'Note: Investigating', list_last_message_at: '2026-09-11T09:03:00Z' }));
    expect(filterSupportConversations(updated.data, { navFilter: 'waiting', mailboxScope: 'all', searchQuery: '' })).toEqual([]);
  });

  it('matches support conversation list query keys but not detail or message keys', () => {
    expect(
      isSupportConversationListQueryKey(
        ['support', 'ws-1', 'conversations', { mailbox_id: 'shared' }],
        'ws-1',
      ),
    ).toBe(true);
    expect(
      isSupportConversationListQueryKey(
        ['support', 'ws-1', 'conversations', undefined],
        'ws-1',
      ),
    ).toBe(true);
    expect(
      isSupportConversationListQueryKey(
        ['support', 'ws-1', 'conversations', 'infinite', { mailbox_id: 'shared' }],
        'ws-1',
      ),
    ).toBe(true);
    expect(
      isSupportConversationListQueryKey(
        ['support', 'ws-1', 'conversations', 'conv-1'],
        'ws-1',
      ),
    ).toBe(false);
    expect(
      isSupportConversationListQueryKey(
        ['support', 'ws-1', 'conversations', 'conv-1', 'messages'],
        'ws-1',
      ),
    ).toBe(false);
  });

  it('updates unread counts on list payloads and ignores malformed cache entries', () => {
    const current: ConversationListResponse = {
      data: [
        {
          id: 'conv-1',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Subject',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 0,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
      meta: {
        unread: {
          total: 0,
          my_inbox: 0,
          unassigned: 0,
          ai_active: 0,
        },
      },
    };

    expect(updateConversationListUnreadCount(current, 'conv-1', 1)?.data[0]?.unread_count).toBe(1);
    expect(
      updateConversationListUnreadCount({ foo: 'bar' } as ConversationListResponse, 'conv-1', 1),
    ).toEqual({ foo: 'bar' });
  });

  it('updates unread counts on infinite list payloads', () => {
    const current = {
      pages: [
        {
          data: [
            {
              id: 'conv-1',
              workspace_id: 'ws-1',
              display_id: 1,
              subject: 'Subject',
              status: 'open',
              priority: 'medium',
              source: 'widget',
              unread_count: 0,
              created_at: '2026-04-08T00:00:00Z',
              updated_at: '2026-04-08T00:00:00Z',
            },
          ],
          total: 1,
          page: 1,
          per_page: 50,
          total_pages: 1,
          meta: {
            unread: {
              total: 0,
              my_inbox: 0,
              unassigned: 0,
              ai_active: 0,
            },
          },
        },
      ],
      pageParams: [1],
    };

    const updated = updateConversationListUnreadCount(current, 'conv-1', 2);
    expect(extractConversationListConversations(updated)[0]?.unread_count).toBe(2);
  });

  it('updates detail unread counts without changing unrelated entries', () => {
    const current: SupportConversation = {
      id: 'conv-1',
      workspace_id: 'ws-1',
      display_id: 1,
      subject: 'Subject',
      status: 'open',
      priority: 'medium',
      source: 'widget',
      unread_count: 2,
      created_at: '2026-04-08T00:00:00Z',
      updated_at: '2026-04-08T00:00:00Z',
    };

    expect(updateConversationUnreadCount(current, 0)?.unread_count).toBe(0);
    expect(updateConversationUnreadCount(undefined, 0)).toBeUndefined();
  });

  it('moves public agent message activity to the top without changing unread count', () => {
    const current: ConversationListResponse = {
      data: [
        {
          id: 'conv-old',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Older',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 0,
          last_message: 'Older message',
          awaiting_reply: true,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
        {
          id: 'conv-activity',
          workspace_id: 'ws-1',
          display_id: 2,
          subject: 'Activity',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 3,
          last_message: 'Customer question',
          awaiting_reply: true,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      per_page: 50,
      total_pages: 1,
    };

    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-activity',
      timestamp: '2026-04-08T10:00:00Z',
      message: {
        content: 'We will check this.',
        sender_type: 'user',
        message_type: 'reply',
      },
    }) as ConversationListResponse;

    expect(updated.data.map((conversation) => conversation.id)).toEqual(['conv-activity', 'conv-old']);
    expect(updated.data[0]).toEqual(expect.objectContaining({
      unread_count: 3,
      awaiting_reply: false,
      last_message: 'We will check this.',
      last_message_sender_type: 'user',
      updated_at: '2026-04-08T10:00:00Z',
    }));
  });

  it('moves customer activity and sets shared response state without guessing personal unread', () => {
    const current: ConversationListResponse = {
      data: [
        {
          id: 'conv-1',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Subject',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 1,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
    };

    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-1',
      timestamp: '2026-04-08T10:00:00Z',
      message: {
        content: 'I still need help.',
        sender_type: 'customer',
        message_type: 'reply',
      },
    }) as ConversationListResponse;

    expect(updated.data[0]).toEqual(expect.objectContaining({
      unread_count: 1,
      awaiting_reply: true,
      customer_awaiting_response: true,
      last_message: 'I still need help.',
      last_message_sender_type: 'customer',
    }));
  });

  it('marks customer activity as human work after an explicit AI takeover', () => {
    const current: ConversationListResponse = {
      data: [{
        id: 'conv-handoff',
        workspace_id: 'ws-1',
        display_id: 1,
        subject: 'Handoff',
        status: 'open',
        flow_state: 'ai_handling',
        human_takeover: true,
        priority: 'medium',
        source: 'widget',
        unread_count: 0,
        created_at: '2026-04-08T00:00:00Z',
        updated_at: '2026-04-08T00:00:00Z',
      }],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
    };

    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-handoff',
      timestamp: '2026-04-08T10:00:00Z',
      message: {
        content: 'A human still needs to answer.',
        sender_type: 'customer',
        message_type: 'reply',
      },
    }) as ConversationListResponse;

    expect(updated.data[0]).toEqual(expect.objectContaining({
      customer_awaiting_response: true,
      needs_human_reply: true,
    }));
  });

  it('moves internal note activity to the top without changing unread or public preview', () => {
    const current: ConversationListResponse = {
      data: [
        {
          id: 'conv-old',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Older',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 0,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
        {
          id: 'conv-note',
          workspace_id: 'ws-1',
          display_id: 2,
          subject: 'Note',
          status: 'open',
          priority: 'medium',
          source: 'widget',
          unread_count: 4,
          last_message: 'Customer question',
          awaiting_reply: true,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      per_page: 50,
      total_pages: 1,
    };

    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-note',
      timestamp: '2026-04-08T10:00:00Z',
    }) as ConversationListResponse;

    expect(updated.data.map((conversation) => conversation.id)).toEqual(['conv-note', 'conv-old']);
    expect(updated.data[0]).toEqual(expect.objectContaining({
      unread_count: 4,
      awaiting_reply: true,
      last_message: 'Customer question',
      updated_at: '2026-04-08T10:00:00Z',
    }));
  });

  it('advances note timestamps without changing public response or unread state', () => {
    const current: ConversationListResponse = {
      data: [{ id: 'conv-note', workspace_id: 'ws-1', display_id: 1, subject: 'Note',
        status: 'open', priority: 'medium', source: 'widget',
        created_at: '2026-09-07T00:00:00Z', updated_at: '2026-09-07T10:00:00Z',
        list_last_message_at: '2026-09-07T10:00:00Z',
        last_message: 'Customer question', unread_count: 2, awaiting_reply: true,
      }], total: 1, page: 1, per_page: 50, total_pages: 1,
    };
    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-note', messageId: 'note', timestamp: '2026-09-07T12:00:00Z',
      message: { sender_type: 'user', message_type: 'reply', is_internal: true, content: 'Investigating this' },
    }) as ConversationListResponse;
    expect(updated.data[0]).toEqual(expect.objectContaining({
      list_last_activity_at: '2026-09-07T12:00:00Z',
      list_last_message_id: 'note', list_last_message_at: '2026-09-07T12:00:00Z',
      unread_count: 2, awaiting_reply: true,
    }));
  });

  it.each([false, true])('keeps system activity separate from message state (infinite: %s)', (infinite) => {
    const conversation = {
      id: 'conv-1', workspace_id: 'ws-1', display_id: 1, subject: 'Subject',
      status: 'open' as const, priority: 'medium' as const, source: 'widget' as const,
      created_at: '2026-09-07T00:00:00Z', updated_at: '2026-09-07T10:00:00Z',
      list_last_message_id: 'reply', list_last_message_at: '2026-09-07T10:00:00Z',
      last_message: 'Customer question', unread_count: 2, awaiting_reply: true,
    };
    const page = { data: [conversation], total: 1, page: 1, per_page: 50, total_pages: 1 };
    const current = infinite ? { pages: [page], pageParams: [1] } : page;
    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-1', messageId: 'status-event', timestamp: '2026-09-07T12:00:00Z',
      message: { sender_type: 'system', message_type: 'system', system_event_type: 'resolved' },
    })!;
    const row = 'pages' in updated ? updated.pages[0].data[0] : updated.data[0];
    expect(row).toEqual(expect.objectContaining({
      list_last_activity_at: '2026-09-07T12:00:00Z',
      list_last_message_id: 'reply', list_last_message_at: '2026-09-07T10:00:00Z',
      last_message: 'Customer question', unread_count: 2, awaiting_reply: true,
    }));
    const delayed = moveConversationToTopForMessageActivity(updated, {
      conversationId: 'conv-1', messageId: 'old-status', timestamp: '2026-09-07T11:00:00Z',
      message: { sender_type: 'system', message_type: 'system' },
    });
    expect(delayed).toBe(updated);
  });

  it('ignores a delayed message older than the cached list projection', () => {
    const current: ConversationListResponse = {
      data: [{
        id: 'conv-newer',
        workspace_id: 'ws-1',
        display_id: 1,
        subject: 'Newer',
        status: 'open',
        priority: 'medium',
        source: 'widget',
        list_last_message_id: '00000000-0000-0000-0000-000000000020',
        list_last_message_at: '2026-04-08T11:00:00Z',
        last_message: 'Newest customer reply',
        created_at: '2026-04-08T00:00:00Z',
        updated_at: '2026-04-08T11:00:00Z',
      }],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
    };

    const updated = moveConversationToTopForMessageActivity(current, {
      conversationId: 'conv-newer',
      messageId: '00000000-0000-0000-0000-000000000010',
      timestamp: '2026-04-08T10:00:00Z',
      message: {
        content: 'Delayed reply',
        sender_type: 'customer',
        message_type: 'reply',
      },
    });

    expect(updated).toBe(current);
  });

  it('patches status changes without changing unread count or preview', () => {
    const current: ConversationListResponse = {
      data: [
        {
          id: 'conv-reopen',
          workspace_id: 'ws-1',
          display_id: 1,
          subject: 'Reopened',
          status: 'resolved',
          flow_state: 'resolved_by_human',
          priority: 'medium',
          source: 'widget',
          unread_count: 0,
          last_message: 'Previous reply',
          awaiting_reply: false,
          created_at: '2026-04-08T00:00:00Z',
          updated_at: '2026-04-08T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      per_page: 50,
      total_pages: 1,
    };

    const updated = patchConversationStatusInCache(current, {
      conversationId: 'conv-reopen',
      status: 'open',
      oldStatus: 'resolved',
      flowState: 'assigned_to_human',
      updatedAt: '2026-04-08T11:00:00Z',
      mailboxId: 'mailbox-billing',
    }) as ConversationListResponse;

    expect(updated.data[0]).toEqual(expect.objectContaining({
      status: 'open',
      flow_state: 'assigned_to_human',
      mailbox_id: 'mailbox-billing',
      unread_count: 0,
      last_message: 'Previous reply',
      awaiting_reply: false,
      updated_at: '2026-04-08T11:00:00Z',
    }));
  });

  it('selects the next row after removing a conversation from a list', () => {
    const conversations = [{ id: 'conv-1' }, { id: 'conv-2' }, { id: 'conv-3' }];

    expect(getNextConversationIdAfterRemoval(conversations, 'conv-1')).toBe('conv-2');
    expect(getNextConversationIdAfterRemoval(conversations, 'conv-2')).toBe('conv-3');
    expect(getNextConversationIdAfterRemoval(conversations, 'conv-3')).toBe('conv-2');
    expect(getNextConversationIdAfterRemoval([], 'conv-1')).toBeNull();
  });
});
