import type { InboxClearedMemory } from './inboxClearedMessages';

const storageKey = (userId: string) => `helpin:inbox-cleared:${userId}`;

const emptyMemory = (): InboxClearedMemory => ({ recent: [], unlocked: {} });

/** Loads what this viewer has seen; storage can be blocked or empty. */
export function loadInboxClearedMemory(userId: string): InboxClearedMemory {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(storageKey(userId)) ?? 'null');
    if (!parsed || typeof parsed !== 'object') return emptyMemory();
    const { recent, unlocked } = parsed as Partial<InboxClearedMemory>;
    return {
      recent: Array.isArray(recent) ? recent.filter((item): item is string => typeof item === 'string') : [],
      unlocked: unlocked && typeof unlocked === 'object' ? { ...unlocked } : {},
    };
  } catch {
    return emptyMemory();
  }
}

export function saveInboxClearedMemory(userId: string, memory: InboxClearedMemory): void {
  try {
    localStorage.setItem(storageKey(userId), JSON.stringify(memory));
  } catch {
    // Private windows and blocked storage only lose the anti-repeat memory.
  }
}
