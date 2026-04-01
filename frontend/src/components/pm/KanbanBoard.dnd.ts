import { useCallback, useSyncExternalStore } from 'react';
import type { Story } from '@/lib/pmTypes';

// ── DragPreviewManager ─────────────────────────────────────────────
// Stores drag preview state outside React's render cycle.
// Per-column subscriptions ensure only affected columns re-render.

type Listener = () => void;

export class DragPreviewManager {
  private activeStory: Story | null = null;
  private columnOverrides = new Map<string, Story[]>();
  private columnListeners = new Map<string, Set<Listener>>();
  private globalListeners = new Set<Listener>();

  // ── Mutations ──

  setActiveStory(story: Story | null) {
    this.activeStory = story;
    this.notifyGlobal();
  }

  updatePreview(fromId: string, toId: string, newFromStories: Story[], newToStories: Story[]) {
    this.columnOverrides.set(fromId, newFromStories);
    this.columnOverrides.set(toId, newToStories);
    this.notifyColumn(fromId);
    if (toId !== fromId) this.notifyColumn(toId);
  }

  clear() {
    const affectedIds = [...this.columnOverrides.keys()];
    this.activeStory = null;
    this.columnOverrides.clear();
    for (const id of affectedIds) {
      this.notifyColumn(id);
    }
    this.notifyGlobal();
  }

  // ── Reads ──

  getActiveStory(): Story | null {
    return this.activeStory;
  }

  getColumnStories(columnId: string): Story[] | null {
    return this.columnOverrides.get(columnId) ?? null;
  }

  // ── Subscriptions ──

  subscribeColumn(columnId: string, listener: Listener): () => void {
    let listeners = this.columnListeners.get(columnId);
    if (!listeners) {
      listeners = new Set();
      this.columnListeners.set(columnId, listeners);
    }
    listeners.add(listener);
    return () => {
      listeners!.delete(listener);
      if (listeners!.size === 0) this.columnListeners.delete(columnId);
    };
  }

  subscribeGlobal(listener: Listener): () => void {
    this.globalListeners.add(listener);
    return () => { this.globalListeners.delete(listener); };
  }

  // ── Notifications ──

  private notifyColumn(columnId: string) {
    const listeners = this.columnListeners.get(columnId);
    if (listeners) {
      for (const listener of listeners) listener();
    }
  }

  private notifyGlobal() {
    for (const listener of this.globalListeners) listener();
  }
}

// ── Hooks ───────────────────────────────────────────────────────────

/**
 * Subscribe to drag preview for a specific column.
 * Returns override stories during drag, or baseStories otherwise.
 */
export function useColumnDragPreview(
  manager: DragPreviewManager,
  columnId: string,
  baseStories: Story[],
): Story[] {
  const subscribe = useCallback(
    (cb: () => void) => manager.subscribeColumn(columnId, cb),
    [manager, columnId],
  );
  const getSnapshot = useCallback(
    () => manager.getColumnStories(columnId),
    [manager, columnId],
  );
  const override = useSyncExternalStore(subscribe, getSnapshot, () => null);
  return override ?? baseStories;
}

/**
 * Subscribe to the active dragged story (for DragOverlay).
 */
export function useActiveStory(manager: DragPreviewManager): Story | null {
  const subscribe = useCallback(
    (cb: () => void) => manager.subscribeGlobal(cb),
    [manager],
  );
  const getSnapshot = useCallback(
    () => manager.getActiveStory(),
    [manager],
  );
  return useSyncExternalStore(subscribe, getSnapshot, () => null);
}

// ── Existing helpers ────────────────────────────────────────────────

export function getStateBoardPreviewInsertIndex({
  toStateType,
  overId,
  toStateId,
  overIdx,
  columnLength,
  pointerBelowMid,
}: {
  toStateType?: string;
  overId: string;
  toStateId: string;
  overIdx: number;
  columnLength: number;
  pointerBelowMid: boolean;
}) {
  if (toStateType === 'done') {
    return 0;
  }
  if (overId === toStateId) {
    return columnLength;
  }
  if (overIdx < 0) {
    return columnLength;
  }
  return pointerBelowMid ? overIdx + 1 : overIdx;
}

export function getSameStateBoardDropIndex({
  overId,
  stateId,
  overIndex,
  columnLength,
}: {
  overId: string;
  stateId: string;
  overIndex: number;
  columnLength: number;
}) {
  if (overId === stateId) {
    return Math.max(0, columnLength - 1);
  }
  if (overIndex < 0) {
    return Math.max(0, columnLength - 1);
  }
  return overIndex;
}

export function commitDropBeforeClearingPreview<T>({
  commit,
  clearPreview,
}: {
  commit: () => Promise<T>;
  clearPreview: () => void;
}) {
  try {
    const commitPromise = commit();
    clearPreview();
    return commitPromise;
  } catch (error) {
    clearPreview();
    throw error;
  }
}
