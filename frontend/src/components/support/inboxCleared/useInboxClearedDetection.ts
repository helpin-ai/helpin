import { useEffect, useRef } from 'react';
import { format, startOfDay } from 'date-fns';

import { supportService } from '@/lib/services/supportService';
import { useSupportInboxCelebrationStore } from '@/stores/supportInboxCelebrationStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type { SupportInboxClearStats } from '@/lib/pmTypes';
import { loadInboxClearedMemory, saveInboxClearedMemory } from './inboxClearedMemory';
import { composeInboxClearedMessage } from './inboxClearedMessages';
import { inboxClearScopeFor, isOwnInboxClear, type InboxClearScope } from './inboxClearedScope';

const STATS_TIMEOUT_MS = 2000;

interface CelebrateInboxClearInput {
  workspaceId: string;
  userId: string;
  scope: InboxClearScope;
  names: { inbox: string; inboxInSentence: string };
  conversationId: string;
  fallbackCount: number;
  now?: Date;
}

async function recordClear(workspaceId: string, payload: Parameters<typeof supportService.recordInboxClear>[1]): Promise<SupportInboxClearStats | null> {
  let timer = 0;
  const timeout = new Promise<null>((resolve) => {
    timer = window.setTimeout(() => resolve(null), STATS_TIMEOUT_MS);
  });
  try {
    const response = await Promise.race([supportService.recordInboxClear(workspaceId, payload), timeout]);
    return response?.data ?? null;
  } catch {
    return null;
  } finally {
    window.clearTimeout(timer);
  }
}

/**
 * Records the clear, picks the words and shows the celebration. Without the
 * server's numbers it still celebrates, using only the time of day.
 */
export async function celebrateInboxClear({
  workspaceId,
  userId,
  scope,
  names,
  conversationId,
  fallbackCount,
  now = new Date(),
}: CelebrateInboxClearInput): Promise<void> {
  const localDate = format(now, 'yyyy-MM-dd');
  const stats = await recordClear(workspaceId, {
    conversation_id: conversationId,
    scope: scope.scope,
    mailbox_id: scope.mailboxId,
    local_date: localDate,
    day_start: startOfDay(now).toISOString(),
  });

  // The moment has passed if the viewer opened something or changed view.
  const inbox = useSupportInboxStore.getState();
  if (inbox.selectedConversationId || inboxClearScopeFor(inbox.navFilter, inbox.selectedMailboxId)?.scopeKey !== scope.scopeKey) {
    return;
  }

  const { message, memory } = composeInboxClearedMessage(
    { ...names, hour: now.getHours(), day: now.getDay(), stats },
    loadInboxClearedMemory(userId),
    { scopeKey: scope.scopeKey, localDate },
  );
  saveInboxClearedMemory(userId, memory);
  useSupportInboxCelebrationStore.getState().showCelebration({
    id: `${scope.scopeKey}:${now.getTime()}`,
    scopeKey: scope.scopeKey,
    message,
    clearedCount: stats?.cleared_count ?? fallbackCount,
  });
}

/**
 * Watches the visible list and celebrates when the viewer's own action
 * empties a clearable inbox. Also puts the celebration away once the list
 * fills again or the view changes.
 */
export function useInboxClearedDetection({
  workspaceId,
  userId,
  scope,
  names,
  conversationIds,
  settled,
}: {
  workspaceId: string;
  userId: string | undefined;
  /** Null when the current view is not an inbox to clear. */
  scope: InboxClearScope | null;
  names: { inbox: string; inboxInSentence: string };
  conversationIds: string[];
  /** False while the list is loading, refetching an empty list or failed. */
  settled: boolean;
}) {
  const previous = useRef<{ scopeKey: string | null; ids: string[] }>({ scopeKey: null, ids: [] });
  const scopeKey = scope?.scopeKey ?? null;
  const idsKey = conversationIds.join(',');

  useEffect(() => {
    if (!settled) return;
    const ids = idsKey ? idsKey.split(',') : [];
    const before = previous.current;
    previous.current = { scopeKey, ids };

    const celebrations = useSupportInboxCelebrationStore.getState();
    if (celebrations.celebration && (celebrations.celebration.scopeKey !== scopeKey || ids.length > 0)) {
      celebrations.dismissCelebration();
    }

    // A re-run with the same list compares it with itself and stays quiet.
    if (!scope || !userId || before.scopeKey !== scopeKey) return;
    if (!isOwnInboxClear({ previousIds: before.ids, currentIds: ids, ownRemovals: celebrations.ownRemovals, now: Date.now() })) return;
    celebrations.forgetOwnRemovals(before.ids);
    void celebrateInboxClear({
      workspaceId,
      userId,
      scope,
      names,
      conversationId: before.ids[before.ids.length - 1],
      fallbackCount: before.ids.length,
    });
  }, [idsKey, names, scope, scopeKey, settled, userId, workspaceId]);
}
