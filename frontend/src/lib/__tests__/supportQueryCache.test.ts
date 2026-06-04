import { describe, expect, it } from 'vitest';
import {
  extractConversationListConversations,
  getNextConversationIdAfterRemoval,
  isSupportConversationListQueryKey,
  moveConversationToTopForMessageActivity,
  updateConversationListUnreadCount,
  updateConversationUnreadCount,
} from '@/lib/supportQueryCache';
import type { ConversationListResponse, SupportConversation } from '@/lib/pmTypes';

describe('supportQueryCache', () => {
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

  it('moves customer message activity to the top and increments unread count', () => {
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
      unread_count: 2,
      awaiting_reply: true,
      last_message: 'I still need help.',
      last_message_sender_type: 'customer',
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

  it('selects the next row after removing a conversation from a list', () => {
    const conversations = [{ id: 'conv-1' }, { id: 'conv-2' }, { id: 'conv-3' }];

    expect(getNextConversationIdAfterRemoval(conversations, 'conv-1')).toBe('conv-2');
    expect(getNextConversationIdAfterRemoval(conversations, 'conv-2')).toBe('conv-3');
    expect(getNextConversationIdAfterRemoval(conversations, 'conv-3')).toBe('conv-2');
    expect(getNextConversationIdAfterRemoval([], 'conv-1')).toBeNull();
  });
});
