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
/**
 * The server changes a batch one conversation after another, so status
 * changes go in small batches, a few at a time. A bulk resolve stays as quick
 * as one request per conversation and its progress keeps moving, with a
 * fraction of the requests.
 */
const STATUS_BATCH_SIZE = 5;
/** Requests a bulk action keeps in flight at once. */
const BULK_CONCURRENCY = 4;

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

// Sends the actions to the bulk endpoint in batches, `concurrency` requests at a time.
async function runServerBatches(
  workspaceId: string,
  ids: string[],
  actions: SupportConversationActions,
  batchSize: number,
  concurrency: number,
  onProgress?: (completed: number) => void,
): Promise<ConversationBulkResult> {
  const result: ConversationBulkResult = { succeeded: [], failed: [] };
  const batches: string[][] = [];
  for (let offset = 0; offset < ids.length; offset += batchSize) batches.push(ids.slice(offset, offset + batchSize));
  let completed = 0;
  const runBatch = async (batch: string[]) => {
    let response: Awaited<ReturnType<typeof supportService.bulkConversationActions>>;
    try {
      response = await supportService.bulkConversationActions(workspaceId, batch, actions);
    } catch (error) {
      response = { data: null, error: error instanceof Error ? error.message : 'Unable to update conversations' } as typeof response;
    }
    if (response.error || !response.data) {
      const error = response.error ?? 'Unable to update conversations';
      result.failed.push(...batch.map((id) => ({ id, error })));
    } else {
      const outcomes = new Map(response.data.results.map((outcome) => [outcome.conversation_id, outcome]));
      for (const id of batch) {
        const outcome = outcomes.get(id);
        if (outcome && !outcome.error) result.succeeded.push(id);
        else result.failed.push({ id, error: outcome?.error ?? 'Unable to update conversation' });
      }
    }
    completed += batch.length;
    onProgress?.(completed);
  };
  for (let index = 0; index < batches.length; index += concurrency) {
    await Promise.all(batches.slice(index, index + concurrency).map(runBatch));
  }
  return result;
}

// Status changes run on the server: the bulk endpoint makes the same change
// as the single-conversation one. It can't mark spam, so spam, moves, read
// state and deletes reuse the individual endpoints so permissions, audit
// events, routing, and AI state transitions follow the same path as a
// single-conversation action.
export async function runConversationBulkAction(
  workspaceId: string,
  conversations: SupportConversation[],
  action: ConversationBulkAction,
  onProgress?: (completed: number) => void,
): Promise<ConversationBulkResult> {
  const unique = [...new Map(conversations.map((item) => [item.id, item])).values()];
  const ids = unique.map((conversation) => conversation.id);
  if (action.type === 'status' && action.status !== 'spam') {
    return runServerBatches(workspaceId, ids, { status: action.status }, STATUS_BATCH_SIZE, BULK_CONCURRENCY, onProgress);
  }
  if (action.type === 'operations') {
    return runServerBatches(workspaceId, ids, action.actions, SUPPORT_BULK_ACTION_LIMIT, 1, onProgress);
  }
  const result: ConversationBulkResult = { succeeded: [], failed: [] };
  let completed = 0;
  for (let offset = 0; offset < unique.length; offset += BULK_CONCURRENCY) {
    const batch = unique.slice(offset, offset + BULK_CONCURRENCY);
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
