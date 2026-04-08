import { describe, expect, it } from 'vitest';
import {
  isSupportConversationListQueryKey,
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
          ai_all: 0,
          ai_pending: 0,
        },
      },
    };

    expect(updateConversationListUnreadCount(current, 'conv-1', 1)?.data[0]?.unread_count).toBe(1);
    expect(
      updateConversationListUnreadCount({ foo: 'bar' } as ConversationListResponse, 'conv-1', 1),
    ).toEqual({ foo: 'bar' });
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
});
