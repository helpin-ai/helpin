import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SupportConversation } from '@/lib/pmTypes';
import { supportService } from '@/lib/services/supportService';
import { loadConversationSelection, runConversationBulkAction } from '../supportBulkActions';

vi.mock('@/lib/services/supportService', () => ({
  supportService: {
    listConversations: vi.fn(),
    updateConversationStatus: vi.fn(),
    deleteConversation: vi.fn(),
    moveConversation: vi.fn(),
    markConversationRead: vi.fn(),
    markConversationUnread: vi.fn(),
  },
}));

const conversation = (id: string) => ({ id, last_customer_message_id: `message-${id}` }) as SupportConversation;

describe('support bulk actions', () => {
  beforeEach(() => vi.resetAllMocks());

  it('loads every page with the current filters and deduplicates conversations', async () => {
    vi.mocked(supportService.listConversations)
      .mockResolvedValueOnce({ data: { data: [conversation('1'), conversation('2')], total_pages: 2 }, error: null } as never)
      .mockResolvedValueOnce({ data: { data: [conversation('2'), conversation('3')], total_pages: 3 }, error: null } as never);
    const result = await loadConversationSelection('ws', { search: 'refund', mailbox_id: 'team' }, new AbortController().signal);
    expect(result.map((item) => item.id)).toEqual(['1', '2', '3']);
    expect(supportService.listConversations).toHaveBeenNthCalledWith(2, 'ws', { search: 'refund', mailbox_id: 'team', page: 2, per_page: 100 });
    expect(supportService.listConversations).toHaveBeenCalledTimes(2);
  });

  it('does not select a partial result when loading a later page fails', async () => {
    vi.mocked(supportService.listConversations)
      .mockResolvedValueOnce({ data: { data: [conversation('1')], total_pages: 2 }, error: null } as never)
      .mockResolvedValueOnce({ data: null, error: 'Access denied' } as never);
    await expect(loadConversationSelection('ws', {}, new AbortController().signal)).rejects.toThrow('Access denied');
  });

  it('stops loading when the selection scope changes', async () => {
    const controller = new AbortController();
    vi.mocked(supportService.listConversations).mockImplementation(async () => {
      controller.abort();
      return { data: { data: [conversation('1')], total_pages: 3 }, error: null } as never;
    });
    await expect(loadConversationSelection('ws', {}, controller.signal)).rejects.toMatchObject({ name: 'AbortError' });
    expect(supportService.listConversations).toHaveBeenCalledTimes(1);
  });

  it('reports per-conversation failures without retrying successful changes', async () => {
    vi.mocked(supportService.updateConversationStatus).mockImplementation(async (_ws, id) => {
      if (id === '2') return { data: null, error: 'You no longer have access' } as never;
      if (id === '3') throw new Error('Network unavailable');
      return { data: conversation(id), error: null } as never;
    });
    const progress = vi.fn();
    const result = await runConversationBulkAction('ws', ['1', '2', '3', '4', '1'].map(conversation), { type: 'status', status: 'resolved' }, progress);
    expect(result.succeeded).toEqual(['1', '4']);
    expect(result.failed.map((item) => item.id)).toEqual(['2', '3']);
    expect(supportService.updateConversationStatus).toHaveBeenCalledTimes(4);
    expect(progress).toHaveBeenLastCalledWith(4);
  });

  it('treats a failed delete API response as a failure', async () => {
    vi.mocked(supportService.deleteConversation).mockResolvedValue({ data: null, error: 'Forbidden' } as never);
    const result = await runConversationBulkAction('ws', [conversation('1')], { type: 'delete' });
    expect(result.succeeded).toEqual([]);
    expect(result.failed).toEqual([{ id: '1', error: 'Forbidden' }]);
  });

  it('marks read only through the customer message that was selected', async () => {
    vi.mocked(supportService.markConversationRead).mockResolvedValue({ data: {}, error: null } as never);
    await runConversationBulkAction('ws', [conversation('1')], { type: 'read' });
    expect(supportService.markConversationRead).toHaveBeenCalledWith('ws', '1', 'message-1');
  });

  it('limits the number of simultaneous requests', async () => {
    let active = 0;
    let peak = 0;
    vi.mocked(supportService.markConversationUnread).mockImplementation(async () => {
      peak = Math.max(peak, ++active);
      await Promise.resolve();
      --active;
      return { data: {}, error: null } as never;
    });
    await runConversationBulkAction('ws', Array.from({ length: 11 }, (_, index) => conversation(String(index))), { type: 'unread' });
    expect(peak).toBeLessThanOrEqual(4);
    expect(supportService.markConversationUnread).toHaveBeenCalledTimes(11);
  });
});
