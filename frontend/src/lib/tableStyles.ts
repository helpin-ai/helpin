// Shared table design tokens for consistent styling across all tables

/** Outer scrollable container */
export const TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto bg-card dark:bg-transparent';

/** Resting table surface. Keep the workspace backdrop visible in dark mode. */
export const TABLE_SURFACE = 'bg-card dark:bg-transparent';

/** Sticky header bar */
export const TABLE_HEADER = 'shared-table-header sticky top-0 z-10 bg-card dark:bg-transparent';

/** Individual header cell – compact, subtle text */
export const TABLE_HEADER_CELL =
  'shared-table-cell border-b border-border/60 relative shrink-0 bg-card dark:bg-transparent px-2.5 py-1.5 text-left text-[11px] font-medium text-muted-foreground';

/** Sortable header cell – adds cursor pointer */
export const TABLE_HEADER_CELL_SORTABLE = 'cursor-pointer select-none hover:bg-muted dark:hover:bg-muted';

/** Data row – pixel-pinned to ROW_HEIGHT so it exactly matches the virtualizer's translateY spacing. */
export const TABLE_ROW =
  'shared-table-row group/row flex h-[40px] cursor-pointer items-center border-b border-border/60 bg-card dark:bg-transparent hover:bg-muted dark:hover:bg-muted';

/** Data cell – right border for grid lines */
export const TABLE_CELL =
  'shared-table-cell flex shrink-0 items-center self-stretch bg-inherit px-2.5';

/** Checkbox column cell – no horizontal padding so the checkbox sits centered in CHECKBOX_COL_SIZE */
export const TABLE_CELL_SELECT =
  'shared-table-cell flex shrink-0 items-center justify-center self-stretch bg-inherit';

/** Checkbox column header cell – centers the select-all checkbox to match TABLE_CELL_SELECT */
export const TABLE_HEADER_CELL_SELECT =
  'shared-table-cell border-b border-border/60 relative flex shrink-0 items-center justify-center bg-card dark:bg-transparent py-1.5 text-[11px] font-medium text-muted-foreground';

/** Primary entity name shown in table cells. */
export const TABLE_NAME_TEXT = 'text-sm text-foreground/90';

/** Group header row (for grouped/expandable tables) – pixel-pinned to GROUP_ROW_HEIGHT */
export const TABLE_GROUP_ROW =
  'shared-table-row border-b border-border/60 flex h-[40px] cursor-pointer items-center bg-muted/60 text-sm font-semibold hover:bg-muted dark:bg-transparent dark:hover:bg-muted';

/** Inner sticky wrapper for group row content – pins chevron/label to viewport left during horizontal scroll */
export const TABLE_GROUP_ROW_INNER =
  'shared-table-group-inner sticky left-0 z-[1] flex items-center gap-2 px-3';

/** Column resize handle – always-visible 1px separator, expands on hover */
export const TABLE_RESIZE_HANDLE =
  'absolute right-0 top-0 h-full w-px cursor-col-resize touch-none select-none bg-border/40 hover:w-1 hover:bg-primary/30 active:bg-primary/50';

/** Row height for virtualizer estimateSize */
export const ROW_HEIGHT = 40;
export const GROUP_ROW_HEIGHT = 40;

/** Checkbox column width */
export const CHECKBOX_COL_SIZE = 40;

/** Actions column width (kebab menu only) */
export const ACTIONS_COL_SIZE = 45;

/**
 * Actions header cell – sticky-right with bg-card but no border/padding/sort
 * affordance. The actions column has no header label, so the bare TABLE_HEADER_CELL
 * would render as a visible empty box; this strips that chrome.
 */
export const TABLE_HEADER_CELL_ACTIONS = 'shared-table-cell shared-table-pinned-right shrink-0 sticky right-0 z-[11] bg-card dark:bg-transparent border-l border-border/60';

// --- Pinned column tokens ---

/** Pinned cell (left) – scrolling cells are clipped behind it by useTableSurface */
export const TABLE_PINNED_LEFT =
  'shared-table-pinned-left sticky z-[3] bg-inherit dark:bg-transparent dark:group-hover/row:bg-muted border-r border-border/60';

/** Pinned cell (left, last in the pinned group) – carries the divider against the scrollable area */
export const TABLE_PINNED_LEFT_NAME =
  'shared-table-pinned-left sticky z-[4] bg-inherit dark:bg-transparent dark:group-hover/row:bg-muted border-r border-border/60';

/** Pinned cell (right) – sticky right with background */
export const TABLE_PINNED_RIGHT =
  'shared-table-pinned-right sticky right-0 z-[3] bg-inherit dark:bg-transparent dark:group-hover/row:bg-muted border-l border-border/60';

/** Pinned header cell (left) – higher z-index than both header and pinned cells */
export const TABLE_PINNED_HEADER_LEFT = 'shared-table-pinned-left sticky z-[11] bg-card dark:bg-transparent border-r border-border/60';

/** Pinned header cell (left, last in the pinned group) – carries the divider against the scrollable area */
export const TABLE_PINNED_HEADER_LEFT_NAME = 'shared-table-pinned-left sticky z-[12] bg-card dark:bg-transparent border-r border-border/60';

/** Pinned header cell (right) – higher z-index, sticky right */
export const TABLE_PINNED_HEADER_RIGHT = 'shared-table-pinned-right sticky right-0 z-[11] bg-card dark:bg-transparent border-l border-border/60';

/** Checkbox hover-reveal – hidden by default, visible on row hover or when checked */
export const TABLE_CHECKBOX_HOVER =
  'opacity-0 group-hover/row:opacity-100 data-[state=checked]:opacity-100 data-[state=indeterminate]:opacity-100 data-checked:opacity-100';

/**
 * Returns inline style for pinned columns.
 *
 * Horizontal pinning is a wide-viewport affordance only: below `md` the
 * `.shared-table-pinned-*` rule in index.css drops these back to static so the
 * pinned group cannot swallow a narrow viewport.
 */
export function pinnedStyle(
  position: 'left' | 'right',
  offset: number,
): React.CSSProperties {
  return { position: 'sticky', [position]: offset, zIndex: 2 };
}

/**
 * Returns inline style for a column cell/header based on the size convention.
 * size === 999 means "flex-grow to fill remaining space".
 */
export function cellStyle(size: number, minFlexWidth = 300): React.CSSProperties {
  if (size === 999) {
    return { flex: '1 1 0%', minWidth: minFlexWidth };
  }
  return { width: size };
}

/**
 * Returns inline style accounting for column resizing.
 * If the column has been resized (isResized=true), always use the runtime pixel width.
 * Otherwise, fall back to the original cellStyle logic (flex for 999).
 */
export function dynamicCellStyle(
  defSize: number,
  runtimeSize: number,
  isResized: boolean,
  minFlexWidth = 300,
): React.CSSProperties {
  if (defSize === 999 && !isResized) {
    return { flex: '1 1 0%', minWidth: minFlexWidth };
  }
  return { width: runtimeSize };
}

/**
 * Resolves a column's runtime width, preferring the live columnSizing entry
 * over the column.getSize() reading. Avoids stale sizes inside memoized rows
 * where react-table's internal column.getSize() may not have re-evaluated.
 */
export function resolveColumnRuntimeSize(
  column: { id: string; columnDef: { size?: number }; getSize: () => number },
  columnSizing: Record<string, number>,
): { defSize: number; runtimeSize: number; isResized: boolean } {
  const defSize = column.columnDef.size ?? 150;
  const resized = columnSizing[column.id];
  const runtimeSize = resized ?? column.getSize();
  return { defSize, runtimeSize, isResized: resized !== undefined };
}

/**
 * Style for a virtualized row's absolute container. Sized to max-content with
 * minWidth: 100% so the row's background + bottom border extend across the
 * full scrolled width, not just the visible viewport.
 */
export function virtualRowStyle(translateY: number): React.CSSProperties {
  return {
    position: 'absolute',
    top: 0,
    left: 0,
    width: 'max-content',
    minWidth: '100%',
    transform: `translateY(${translateY}px)`,
  };
}

/**
 * Resolves the collective selection state for a grouped row.
 * Returns 'indeterminate' when only some leaf rows are selected.
 */
export function getGroupSelectionState<T>(
  groupRow: {
    getLeafRows: () => Array<{ id: string; getIsGrouped: () => boolean; original?: T }>;
  },
  rowSelection: Record<string, boolean>,
): boolean | 'indeterminate' {
  const leaves = groupRow.getLeafRows().filter((leaf) => !leaf.getIsGrouped());
  if (leaves.length === 0) return false;
  let selected = 0;
  for (const leaf of leaves) {
    if (rowSelection[leaf.id]) selected += 1;
  }
  if (selected === 0) return false;
  if (selected === leaves.length) return true;
  return 'indeterminate';
}

/**
 * Returns the next rowSelection state after toggling all leaves under a group.
 * Pass this to setRowSelection: `setRowSelection((s) => toggleGroupSelection(groupRow, s, checked))`
 */
export function toggleGroupSelection<T>(
  groupRow: {
    getLeafRows: () => Array<{ id: string; getIsGrouped: () => boolean; original?: T }>;
  },
  rowSelection: Record<string, boolean>,
  checked: boolean,
): Record<string, boolean> {
  const leafIds = groupRow.getLeafRows()
    .filter((leaf) => !leaf.getIsGrouped())
    .map((leaf) => leaf.id);
  if (leafIds.length === 0) return rowSelection;
  const next = { ...rowSelection };
  for (const id of leafIds) {
    if (checked) next[id] = true;
    else delete next[id];
  }
  return next;
}
