import { useCallback, useSyncExternalStore } from 'react';
import type { Task, TaskGroup } from '@/lib/pmTypes';

const EMPTY_TASK_GROUPS: TaskGroup[] = [];

export const PM_BOARD_DRAG_ACTIVATION_DISTANCE = 2;
export const PM_BOARD_DRAG_OVER_THROTTLE_MS = 16;
export const PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT = 96;
export const PM_BOARD_DROP_PLACEHOLDER_ID_PREFIX = '__pm-board-drop-placeholder__';
const PM_TASK_CARD_DRAG_SOURCE_SELECTOR = '[data-pm-task-card="true"]';
const PM_BOARD_SCROLLBAR_GUTTER_CLASS = '[scrollbar-gutter:stable]';
const PM_BOARD_SCROLL_CONTAINER_BASE_CLASS = 'scrollbar-hover min-h-0 flex-1 overflow-y-auto flex flex-col rounded-md transition-all duration-200';
const PM_BOARD_SCROLL_CONTAINER_OVER_CLASS = 'ring-1 ring-inset ring-sky-500/25';

// ── DragPreviewManager ─────────────────────────────────────────────
// Stores drag preview state outside React's render cycle.
// Per-column subscriptions ensure only affected columns re-render.

type Listener = () => void;

export interface PreviewDropTarget {
  fromColumnId: string;
  toColumnId: string;
  toIndex: number;
}

export function getPMBoardScrollContainerClassName({
  variant,
  isOver,
}: {
  variant: 'state' | 'member';
  isOver: boolean;
}) {
  return [
    PM_BOARD_SCROLL_CONTAINER_BASE_CLASS,
    PM_BOARD_SCROLLBAR_GUTTER_CLASS,
    variant === 'state' ? 'pl-3 pr-2 py-2' : 'p-2',
    isOver ? PM_BOARD_SCROLL_CONTAINER_OVER_CLASS : null,
    'gap-2',
  ].filter(Boolean).join(' ');
}

export function getDragStartTaskRect({
  activeId,
  activatorEvent,
  dndRect,
}: {
  activeId: string;
  activatorEvent: Event | null | undefined;
  dndRect: { height: number; width: number } | null | undefined;
}) {
  const target = activatorEvent?.target;
  if (typeof Element !== 'undefined' && target instanceof Element) {
    const card = target.closest<HTMLElement>(PM_TASK_CARD_DRAG_SOURCE_SELECTOR);
    if (card?.dataset.pmTaskCardId === activeId) {
      const liveRect = card.getBoundingClientRect();
      if (Number.isFinite(liveRect.height) && liveRect.height > 0) {
        return liveRect;
      }
    }
  }
  return dndRect ?? null;
}

export class DragPreviewManager {
  private activeTask: Task | null = null;
  private columnOverrides = new Map<string, Task[]>();
  private dropTarget: PreviewDropTarget | null = null;
  private dropPlaceholderHeight = PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT;
  private columnListeners = new Map<string, Set<Listener>>();
  private globalListeners = new Set<Listener>();

  // ── Mutations ──

  setActiveTask(task: Task | null) {
    this.activeTask = task;
    if (task === null) {
      this.dropTarget = null;
      this.dropPlaceholderHeight = PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT;
    }
    this.notifyGlobal();
  }

  setDropPlaceholderRect(rect: { height: number; width: number } | null | undefined) {
    this.dropPlaceholderHeight = getTaskDropPlaceholderHeight(rect);
    this.notifyGlobal();
  }

  updatePreview(fromId: string, toId: string, newFromTasks: Task[], newToTasks: Task[], toIndex: number) {
    const previousIds = new Set(this.columnOverrides.keys());
    this.columnOverrides.set(fromId, newFromTasks);
    this.columnOverrides.set(toId, newToTasks);
    for (const previousId of previousIds) {
      if (previousId !== fromId && previousId !== toId) {
        this.columnOverrides.delete(previousId);
      }
    }
    this.dropTarget = {
      fromColumnId: fromId,
      toColumnId: toId,
      toIndex,
    };
    const affectedIds = new Set([...previousIds, fromId, toId]);
    for (const id of affectedIds) {
      this.notifyColumn(id);
    }
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
    this.dropPlaceholderHeight = PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT;
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

  getDropPlaceholderHeight(): number {
    return this.dropPlaceholderHeight;
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

export function useColumnDropPlaceholderIndex(
  manager: DragPreviewManager | null | undefined,
  columnId: string,
  taskCount: number,
): number | null {
  const subscribe = useCallback(
    (cb: () => void) => manager ? manager.subscribeColumn(columnId, cb) : NOOP_UNSUB,
    [manager, columnId],
  );
  const getSnapshot = useCallback(
    () => getTaskDropPlaceholderIndex({
      dropTarget: manager?.getDropTarget() ?? null,
      columnId,
      taskCount,
    }),
    [manager, columnId, taskCount],
  );
  return useSyncExternalStore(subscribe, getSnapshot, () => null);
}

export function useDropPlaceholderHeight(manager: DragPreviewManager | null | undefined): number {
  const subscribe = useCallback(
    (cb: () => void) => manager ? manager.subscribeGlobal(cb) : NOOP_UNSUB,
    [manager],
  );
  const getSnapshot = useCallback(
    () => manager?.getDropPlaceholderHeight() ?? PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT,
    [manager],
  );
  return useSyncExternalStore(subscribe, getSnapshot, () => PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT);
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

export function getStateColumnTaskGroupsForRender({
  stateType,
  hasPreviewOverride,
  taskGroups,
}: {
  stateType?: string;
  hasPreviewOverride: boolean;
  taskGroups?: TaskGroup[];
}) {
  if (stateType !== 'done') {
    return EMPTY_TASK_GROUPS;
  }
  if (hasPreviewOverride) {
    return EMPTY_TASK_GROUPS;
  }
  return taskGroups ?? EMPTY_TASK_GROUPS;
}

export function getBaseDragSourceColumnId({
  activeId,
  columns,
}: {
  activeId: string;
  columns: Array<{ id: string; tasks: Array<{ id: string }> }>;
}) {
  return columns.find((column) => column.tasks.some((task) => task.id === activeId))?.id ?? null;
}

function taskListIDsChanged(a: Array<{ id: string }> | null | undefined, b: Array<{ id: string }>) {
  if (!a) return true;
  if (a.length !== b.length) return true;
  for (let i = 0; i < a.length; i++) {
    if (a[i]?.id !== b[i]?.id) return true;
  }
  return false;
}

function dropTargetsChanged(a: PreviewDropTarget | null | undefined, b: PreviewDropTarget | null | undefined) {
  if (!a && !b) return false;
  if (!a || !b) return true;
  return a.fromColumnId !== b.fromColumnId
    || a.toColumnId !== b.toColumnId
    || a.toIndex !== b.toIndex;
}

export function hasDragPreviewChanged({
  currentFrom,
  nextFrom,
  currentTo,
  nextTo,
  currentDropTarget,
  nextDropTarget,
}: {
  currentFrom: Array<{ id: string }> | null | undefined;
  nextFrom: Array<{ id: string }>;
  currentTo: Array<{ id: string }> | null | undefined;
  nextTo: Array<{ id: string }>;
  currentDropTarget?: PreviewDropTarget | null;
  nextDropTarget?: PreviewDropTarget | null;
}) {
  return taskListIDsChanged(currentFrom, nextFrom)
    || taskListIDsChanged(currentTo, nextTo)
    || dropTargetsChanged(currentDropTarget, nextDropTarget);
}

export function getTaskDropPlaceholderPreview({
  activeId,
  fromColumnId,
  toColumnId,
  fromTasks,
  toTasks,
  toIndex,
}: {
  activeId: string;
  fromColumnId: string;
  toColumnId: string;
  fromTasks: Task[];
  toTasks: Task[];
  toIndex: number;
}) {
  const fromPreviewTasks = fromTasks.filter((task) => task.id !== activeId);
  const toPreviewTasks = fromColumnId === toColumnId
    ? fromPreviewTasks
    : toTasks.filter((task) => task.id !== activeId);
  return {
    fromTasks: fromPreviewTasks,
    toTasks: toPreviewTasks,
    dropTarget: {
      fromColumnId,
      toColumnId,
      toIndex,
    },
  };
}

export function getTaskDropPlaceholderIndex({
  dropTarget,
  columnId,
  taskCount,
}: {
  dropTarget: PreviewDropTarget | null;
  columnId: string;
  taskCount: number;
}) {
  if (!dropTarget || dropTarget.toColumnId !== columnId) {
    return null;
  }
  return Math.max(0, Math.min(dropTarget.toIndex, taskCount));
}

export function getTaskDropPlaceholderId(columnId: string, index: number) {
  return `${PM_BOARD_DROP_PLACEHOLDER_ID_PREFIX}:${columnId}:${index}`;
}

export function parseTaskDropPlaceholderId(id: string) {
  const prefix = `${PM_BOARD_DROP_PLACEHOLDER_ID_PREFIX}:`;
  if (!id.startsWith(prefix)) {
    return null;
  }
  const payload = id.slice(prefix.length);
  const indexSeparator = payload.lastIndexOf(':');
  if (indexSeparator <= 0) {
    return null;
  }
  const index = Number(payload.slice(indexSeparator + 1));
  if (!Number.isInteger(index) || index < 0) {
    return null;
  }
  return {
    columnId: payload.slice(0, indexSeparator),
    index,
  };
}

export function getTaskDropPlaceholderHeight(rect: { height: number; width: number } | null | undefined) {
  if (!rect || !Number.isFinite(rect.height)) {
    return PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT;
  }
  return Math.max(24, Math.round(rect.height));
}

export function resolveBoardDropTarget({
  activeId,
  fromColumnId,
  overId,
  pointerBelowMid,
  previewTarget,
  columns,
}: {
  activeId: string;
  fromColumnId: string;
  overId: string;
  pointerBelowMid: boolean;
  previewTarget: PreviewDropTarget | null;
  columns: Array<{ id: string; stateType?: string; tasks: Array<{ id: string }> }>;
}) {
  const placeholderTarget = parseTaskDropPlaceholderId(overId);
  if (placeholderTarget) {
    const targetColumn = columns.find((column) => column.id === placeholderTarget.columnId);
    if (!targetColumn) {
      return null;
    }
    const targetLength = targetColumn.tasks.filter((task) => task.id !== activeId).length;
    return {
      toColumnId: placeholderTarget.columnId,
      toIndex: Math.max(0, Math.min(placeholderTarget.index, targetLength)),
    };
  }

  const eventColumn = columns.find((column) => (
    column.id === overId || column.tasks.some((task) => task.id === overId)
  ));
  const validColumnIds = columns.map((column) => column.id);
  const getStoredTarget = (targetColumnId?: string) => {
    if (!previewTarget) {
      return null;
    }
    if (previewTarget.fromColumnId !== fromColumnId) {
      return null;
    }
    if (targetColumnId && previewTarget.toColumnId !== targetColumnId) {
      return null;
    }
    const targetColumn = columns.find((column) => column.id === previewTarget.toColumnId);
    if (!targetColumn || !validColumnIds.includes(previewTarget.toColumnId)) {
      return null;
    }
    const targetLength = targetColumn.tasks.filter((task) => task.id !== activeId).length;
    return {
      toColumnId: previewTarget.toColumnId,
      toIndex: Math.max(0, Math.min(previewTarget.toIndex, targetLength)),
    };
  };

  if (overId === activeId) {
    return getStoredTarget();
  }

  if (!eventColumn) {
    return null;
  }

  const targetTasksWithoutActive = eventColumn.tasks.filter((task) => task.id !== activeId);
  if (overId === eventColumn.id) {
    const stored = getStoredTarget(eventColumn.id);
    if (stored) {
      return stored;
    }
    return {
      toColumnId: eventColumn.id,
      toIndex: eventColumn.id === fromColumnId
        ? targetTasksWithoutActive.length
        : getStateBoardPreviewInsertIndex({
            toStateType: eventColumn.stateType,
            overId,
            toStateId: eventColumn.id,
            overIdx: -1,
            columnLength: targetTasksWithoutActive.length,
            pointerBelowMid,
          }),
    };
  }

  if (eventColumn.id === fromColumnId) {
    const fromIndex = eventColumn.tasks.findIndex((task) => task.id === activeId);
    const overIndex = eventColumn.tasks.findIndex((task) => task.id === overId);
    return {
      toColumnId: eventColumn.id,
      toIndex: getSameStateBoardDropIndex({
        fromIndex,
        overId,
        stateId: eventColumn.id,
        overIndex,
        columnLength: eventColumn.tasks.length,
        pointerBelowMid,
      }),
    };
  }

  const overIdx = targetTasksWithoutActive.findIndex((task) => task.id === overId);
  return {
    toColumnId: eventColumn.id,
    toIndex: getStateBoardPreviewInsertIndex({
      toStateType: eventColumn.stateType,
      overId,
      toStateId: eventColumn.id,
      overIdx,
      columnLength: targetTasksWithoutActive.length,
      pointerBelowMid,
    }),
  };
}

export function getSameStateBoardDropIndex({
  fromIndex,
  overId,
  stateId,
  overIndex,
  columnLength,
  pointerBelowMid,
}: {
  fromIndex: number;
  overId: string;
  stateId: string;
  overIndex: number;
  columnLength: number;
  pointerBelowMid: boolean;
}) {
  const previewLength = Math.max(0, columnLength - 1);
  if (overId === stateId) {
    return previewLength;
  }
  if (overIndex < 0 || fromIndex < 0) {
    return previewLength;
  }
  const adjustedOverIndex = overIndex > fromIndex ? overIndex - 1 : overIndex;
  const rawIndex = pointerBelowMid ? adjustedOverIndex + 1 : adjustedOverIndex;
  return Math.max(0, Math.min(rawIndex, previewLength));
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
