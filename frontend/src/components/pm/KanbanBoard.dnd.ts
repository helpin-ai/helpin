import { useCallback, useSyncExternalStore } from 'react';
import type { Task } from '@/lib/pmTypes';

// ── DragPreviewManager ─────────────────────────────────────────────
// Stores drag preview state outside React's render cycle.
// Per-column subscriptions ensure only affected columns re-render.

type Listener = () => void;

export interface PreviewDropTarget {
  fromColumnId: string;
  toColumnId: string;
  toIndex: number;
}

export class DragPreviewManager {
  private activeTask: Task | null = null;
  private columnOverrides = new Map<string, Task[]>();
  private dropTarget: PreviewDropTarget | null = null;
  private columnListeners = new Map<string, Set<Listener>>();
  private globalListeners = new Set<Listener>();

  // ── Mutations ──

  setActiveTask(task: Task | null) {
    this.activeTask = task;
    if (task === null) {
      this.dropTarget = null;
    }
    this.notifyGlobal();
  }

  updatePreview(fromId: string, toId: string, newFromTasks: Task[], newToTasks: Task[], toIndex: number) {
    this.columnOverrides.set(fromId, newFromTasks);
    this.columnOverrides.set(toId, newToTasks);
    this.dropTarget = {
      fromColumnId: fromId,
      toColumnId: toId,
      toIndex,
    };
    this.notifyColumn(fromId);
    if (toId !== fromId) this.notifyColumn(toId);
  }

  clearColumnOverrides() {
    const affectedIds = [...this.columnOverrides.keys()];
    this.columnOverrides.clear();
    this.dropTarget = null;
    for (const id of affectedIds) {
      this.notifyColumn(id);
    }
  }

  clear() {
    this.activeTask = null;
    this.clearColumnOverrides();
    this.notifyGlobal();
  }

  // ── Reads ──

  getActiveTask(): Task | null {
    return this.activeTask;
  }

  getColumnTasks(columnId: string): Task[] | null {
    return this.columnOverrides.get(columnId) ?? null;
  }

  getDropTarget(): PreviewDropTarget | null {
    return this.dropTarget;
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
 * Returns override tasks during drag, or baseTasks otherwise.
 */
const NOOP_UNSUB = () => {};

export function useColumnDragPreview(
  manager: DragPreviewManager | null | undefined,
  columnId: string,
  baseTasks: Task[],
): Task[] {
  const subscribe = useCallback(
    (cb: () => void) => manager ? manager.subscribeColumn(columnId, cb) : NOOP_UNSUB,
    [manager, columnId],
  );
  const getSnapshot = useCallback(
    () => manager ? manager.getColumnTasks(columnId) : null,
    [manager, columnId],
  );
  const override = useSyncExternalStore(subscribe, getSnapshot, () => null);
  return override ?? baseTasks;
}

/**
 * Subscribe to the active dragged task (for DragOverlay).
 */
export function useActiveTask(manager: DragPreviewManager | null | undefined): Task | null {
  const subscribe = useCallback(
    (cb: () => void) => manager ? manager.subscribeGlobal(cb) : NOOP_UNSUB,
    [manager],
  );
  const getSnapshot = useCallback(
    () => manager ? manager.getActiveTask() : null,
    [manager],
  );
  return useSyncExternalStore(subscribe, getSnapshot, () => null);
}

export function getStoredCrossColumnDropTarget({
  previewTarget,
  fromColumnId,
  validColumnIds,
}: {
  previewTarget: PreviewDropTarget | null;
  fromColumnId: string;
  validColumnIds: string[];
}) {
  if (!previewTarget) {
    return null;
  }
  if (previewTarget.fromColumnId !== fromColumnId) {
    return null;
  }
  if (previewTarget.toColumnId === fromColumnId) {
    return null;
  }
  if (!validColumnIds.includes(previewTarget.toColumnId)) {
    return null;
  }
  return {
    toColumnId: previewTarget.toColumnId,
    toIndex: previewTarget.toIndex,
  };
}

export function getStableCrossColumnPreviewIndex({
  previewTarget,
  fromColumnId,
  toColumnId,
  overId,
  containerId,
  computedIndex,
  columnLength,
}: {
  previewTarget: PreviewDropTarget | null;
  fromColumnId: string;
  toColumnId: string;
  overId: string;
  containerId: string;
  computedIndex: number;
  columnLength: number;
}) {
  if (overId !== containerId) {
    return computedIndex;
  }
  if (!previewTarget) {
    return computedIndex;
  }
  if (previewTarget.fromColumnId !== fromColumnId || previewTarget.toColumnId !== toColumnId) {
    return computedIndex;
  }
  return Math.max(0, Math.min(previewTarget.toIndex, columnLength));
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
