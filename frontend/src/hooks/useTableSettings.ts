import { useState, useCallback, useRef, useEffect } from 'react';
import type { VisibilityState, ColumnSizingState } from '@tanstack/react-table';

interface TableSettings {
  columnVisibility: VisibilityState;
  columnSizing: ColumnSizingState;
  columnOrder: string[];
}

const STORAGE_PREFIX = 'teampulse:table-settings:';
const DEBOUNCE_MS = 300;
const PINNED_START = new Set(['select', 'email']);
const PINNED_END = new Set(['actions']);
const PINNED_COLUMNS = new Set([...PINNED_START, ...PINNED_END]);

/** Ensures pinned-start columns stay first and pinned-end columns stay last in column order. */
function normalizeColumnOrder(order: string[], allColumnIds?: string[]): string[] {
  if (order.length === 0 && !allColumnIds) return order;

  // Start from existing order or all column IDs
  const base = order.length > 0 ? order : (allColumnIds ?? []);
  const middle = base.filter((id) => !PINNED_COLUMNS.has(id));

  // If allColumnIds provided, insert any missing columns before pinned-end
  if (allColumnIds) {
    const existing = new Set(middle);
    for (const id of allColumnIds) {
      if (!PINNED_COLUMNS.has(id) && !existing.has(id)) {
        middle.push(id);
      }
    }
  }

  const all = allColumnIds ?? order;
  const result: string[] = [];
  // Pinned start columns in definition order
  for (const id of ['select', 'email']) {
    if (all.includes(id)) result.push(id);
  }
  result.push(...middle);
  // Pinned end columns
  if (all.includes('actions')) result.push('actions');
  return result;
}

function loadSettings(tableId: string): Partial<TableSettings> | null {
  try {
    const raw = localStorage.getItem(STORAGE_PREFIX + tableId);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function saveSettings(tableId: string, settings: Partial<TableSettings>) {
  try {
    localStorage.setItem(STORAGE_PREFIX + tableId, JSON.stringify(settings));
  } catch {
    // localStorage may be full or unavailable
  }
}

export function useTableSettings(
  tableId: string,
  defaults: {
    columnVisibility?: VisibilityState;
    columnOrder?: string[];
  } = {},
  /** All column IDs from column definitions, in definition order. Used to reconcile order on visibility changes. */
  allColumnIds?: string[],
) {
  const [stored] = useState(() => loadSettings(tableId));

  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(
    stored?.columnVisibility ?? defaults.columnVisibility ?? {},
  );
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>(
    stored?.columnSizing ?? {},
  );
  const [columnOrder, setColumnOrderRaw] = useState<string[]>(
    normalizeColumnOrder(stored?.columnOrder ?? defaults.columnOrder ?? [], allColumnIds),
  );

  const setColumnOrder = useCallback(
    (orderOrUpdater: string[] | ((prev: string[]) => string[])) => {
      setColumnOrderRaw((prev) => {
        const next = typeof orderOrUpdater === 'function' ? orderOrUpdater(prev) : orderOrUpdater;
        return normalizeColumnOrder(next, allColumnIds);
      });
    },
    [allColumnIds],
  );

  // When visibility changes, reconcile column order so new columns go before actions
  useEffect(() => {
    if (!allColumnIds) return;
    setColumnOrderRaw((prev) => normalizeColumnOrder(prev, allColumnIds));
  }, [columnVisibility, allColumnIds]);

  // Debounced persistence
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const latestRef = useRef({ columnVisibility, columnSizing, columnOrder });
  latestRef.current = { columnVisibility, columnSizing, columnOrder };

  useEffect(() => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      saveSettings(tableId, latestRef.current);
    }, DEBOUNCE_MS);
    return () => clearTimeout(timerRef.current);
  }, [tableId, columnVisibility, columnSizing, columnOrder]);

  const resetColumnSize = useCallback(
    (columnId: string) => {
      setColumnSizing((prev) => {
        const next = { ...prev };
        delete next[columnId];
        return next;
      });
    },
    [],
  );

  return {
    columnVisibility,
    setColumnVisibility,
    columnSizing,
    setColumnSizing,
    columnOrder,
    setColumnOrder,
    resetColumnSize,
  };
}
