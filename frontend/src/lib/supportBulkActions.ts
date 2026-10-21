import type { ConversationStatus, SupportConversation } from '@/lib/pmTypes';
import { supportService } from '@/lib/services/supportService';

export type ConversationBulkAction =
  | { type: 'status'; status: ConversationStatus }
  | { type: 'move'; mailboxId: string | null }
  | { type: 'read' | 'unread' | 'delete' };

export interface ConversationBulkResult {
  succeeded: string[];
  failed: Array<{ id: string; error: string }>;
}

// Build an explicit selection before enabling actions. New arrivals after this
// selection are never silently included in a later bulk action.
export async function loadConversationSelection(
  workspaceId: string,
  filters: Parameters<typeof supportService.listConversations>[1],
  signal: AbortSignal,
  onProgress?: (count: number) => void,
): Promise<SupportConversation[]> {
  const selected = new Map<string, SupportConversation>();
  let pageCount = 1;
  for (let page = 1; page <= pageCount; page++) {
    signal.throwIfAborted();
    const response = await supportService.listConversations(workspaceId, { ...filters, page, per_page: 100 });
    signal.throwIfAborted();
    if (response.error) throw new Error(response.error);
    if (!response.data || !Array.isArray(response.data.data)) throw new Error('Could not load conversations. Please try again.');
    if (page === 1) pageCount = response.data.total_pages;
    for (const conversation of response.data.data) selected.set(conversation.id, conversation);
    onProgress?.(selected.size);
    if (response.data.data.length === 0) break;
  }
  return [...selected.values()];
}

// Reuse the individual endpoints so permissions, audit events, routing, and AI
// state transitions follow the same path as a single-conversation action.
export async function runConversationBulkAction(
  workspaceId: string,
  conversations: SupportConversation[],
  action: ConversationBulkAction,
  onProgress?: (completed: number) => void,
): Promise<ConversationBulkResult> {
  const unique = [...new Map(conversations.map((item) => [item.id, item])).values()];
  const result: ConversationBulkResult = { succeeded: [], failed: [] };
  let completed = 0;
  for (let offset = 0; offset < unique.length; offset += 4) {
    const batch = unique.slice(offset, offset + 4);
    const outcomes = await Promise.allSettled(batch.map(async (conversation) => {
      try {
        const response = await (() => {
          switch (action.type) {
            case 'status': return supportService.updateConversationStatus(workspaceId, conversation.id, action.status);
            case 'move': return supportService.moveConversation(workspaceId, conversation.id, action.mailboxId);
            case 'read': return supportService.markConversationRead(workspaceId, conversation.id, conversation.last_customer_message_id ?? undefined);
            case 'unread': return supportService.markConversationUnread(workspaceId, conversation.id);
            case 'delete': return supportService.deleteConversation(workspaceId, conversation.id);
          }
        })();
        if (response.error) throw new Error(response.error);
      } finally {
        onProgress?.(++completed);
      }
    }));
    outcomes.forEach((outcome, index) => {
      const id = batch[index].id;
      if (outcome.status === 'fulfilled') result.succeeded.push(id);
      else result.failed.push({ id, error: outcome.reason instanceof Error ? outcome.reason.message : 'Unable to update conversation' });
    });
  }
  return result;
}
