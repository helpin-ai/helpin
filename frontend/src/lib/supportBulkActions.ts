import type { ConversationStatus, SupportConversation, SupportConversationActions } from '@/lib/pmTypes';
import { supportService } from '@/lib/services/supportService';

export type ConversationBulkAction =
  | { type: 'status'; status: ConversationStatus }
  | { type: 'move'; mailboxId: string | null }
  | { type: 'read' | 'unread' | 'delete' }
  // Assignment, priority, tags and snooze run on the server in batches.
  | { type: 'operations'; actions: SupportConversationActions };

/** The server applies at most this many conversations per bulk request. */
export const SUPPORT_BULK_ACTION_LIMIT = 100;

/** Whether an action takes conversations out of the current list. */
export function bulkActionRemovesFromList(action: ConversationBulkAction): boolean {
  switch (action.type) {
    case 'delete':
    case 'move':
      return true;
    case 'status':
      return action.status !== 'open';
    case 'operations':
      return action.actions.snooze_minutes !== undefined || action.actions.mailbox_id !== undefined
        || (action.actions.status !== undefined && action.actions.status !== 'open');
    default:
      return false;
  }
}

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
  if (action.type === 'operations') {
    for (let offset = 0; offset < unique.length; offset += SUPPORT_BULK_ACTION_LIMIT) {
      const ids = unique.slice(offset, offset + SUPPORT_BULK_ACTION_LIMIT).map((conversation) => conversation.id);
      let response: Awaited<ReturnType<typeof supportService.bulkConversationActions>>;
      try {
        response = await supportService.bulkConversationActions(workspaceId, ids, action.actions);
      } catch (error) {
        response = { data: null, error: error instanceof Error ? error.message : 'Unable to update conversations' } as typeof response;
      }
      if (response.error || !response.data) {
        const error = response.error ?? 'Unable to update conversations';
        result.failed.push(...ids.map((id) => ({ id, error })));
      } else {
        const outcomes = new Map(response.data.results.map((outcome) => [outcome.conversation_id, outcome]));
        for (const id of ids) {
          const outcome = outcomes.get(id);
          if (outcome && !outcome.error) result.succeeded.push(id);
          else result.failed.push({ id, error: outcome?.error ?? 'Unable to update conversation' });
        }
      }
      completed += ids.length;
      onProgress?.(completed);
    }
    return result;
  }
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
