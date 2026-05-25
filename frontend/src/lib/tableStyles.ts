// Shared table design tokens for consistent styling across all tables

/** Outer scrollable container */
export const TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto';

/** Sticky header bar */
export const TABLE_HEADER = 'sticky top-0 z-10 bg-card';

/** Individual header cell – compact, subtle text */
export const TABLE_HEADER_CELL =
  'ui-divider-bottom-fade ui-divider-right-fade relative shrink-0 bg-card px-2.5 py-1.5 text-left text-[11px] font-medium text-muted-foreground last:bg-none';

/** Sortable header cell – adds cursor pointer */
export const TABLE_HEADER_CELL_SORTABLE = 'cursor-pointer select-none hover:bg-muted';

/** Data row – compact h-9 (36px), named group for hover-reveal actions */
export const TABLE_ROW =
  'group/row flex h-9 items-center bg-card hover:bg-muted';

/** Data cell – right border for grid lines */
export const TABLE_CELL =
  'ui-divider-bottom-fade ui-divider-right-fade flex shrink-0 items-center self-stretch bg-inherit px-2.5 last:bg-none';

/** Group header row (for grouped/expandable tables) */
export const TABLE_GROUP_ROW =
  'ui-divider-bottom-fade flex h-9 cursor-pointer items-center bg-muted/20 text-sm font-semibold hover:bg-muted';

/** Inner sticky wrapper for group row content – pins chevron/label to viewport left during horizontal scroll */
export const TABLE_GROUP_ROW_INNER =
  'sticky left-0 z-[1] flex items-center gap-2 px-3';

/** Column resize handle – always-visible 1px separator, expands on hover */
export const TABLE_RESIZE_HANDLE =
  'absolute right-0 top-0 h-full w-px cursor-col-resize touch-none select-none bg-border/40 hover:w-1 hover:bg-primary/30 active:bg-primary/50';

/** Row height for virtualizer estimateSize */
export const ROW_HEIGHT = 36;
export const GROUP_ROW_HEIGHT = 36;

/** Checkbox column width */
export const CHECKBOX_COL_SIZE = 40;

/** Actions column width (kebab menu only) */
export const ACTIONS_COL_SIZE = 45;

/**
 * Actions header cell – sticky-right with bg-card but no border/padding/sort
 * affordance. The actions column has no header label, so the bare TABLE_HEADER_CELL
 * would render as a visible empty box; this strips that chrome.
 */
export const TABLE_HEADER_CELL_ACTIONS = 'shrink-0 sticky right-0 z-[11] bg-card';

// --- Pinned column tokens ---

/** Pinned cell (left) – sticky with background so content doesn't bleed through */
export const TABLE_PINNED_LEFT =
  'sticky z-[3] bg-inherit';

/** Pinned cell (left, after checkbox) – higher z so it layers above the select column */
export const TABLE_PINNED_LEFT_NAME =
  'sticky z-[4] bg-inherit';

/** Pinned cell (right) – sticky right with background */
export const TABLE_PINNED_RIGHT =
  'sticky right-0 z-[3] bg-inherit';

/** Pinned header cell (left) – higher z-index than both header and pinned cells */
export const TABLE_PINNED_HEADER_LEFT = 'sticky z-[11] bg-card';

/** Pinned header cell (left, after checkbox) – sticky name header */
export const TABLE_PINNED_HEADER_LEFT_NAME = 'sticky z-[12] bg-card';

/** Pinned header cell (right) – higher z-index, sticky right */
export const TABLE_PINNED_HEADER_RIGHT = 'sticky right-0 z-[11] bg-card';

/** Checkbox hover-reveal – hidden by default, visible on row hover or when checked */
export const TABLE_CHECKBOX_HOVER =
  'opacity-0 group-hover/row:opacity-100 data-[state=checked]:opacity-100 data-[state=indeterminate]:opacity-100 data-checked:opacity-100';

/**
 * Returns inline style for pinned columns.
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
