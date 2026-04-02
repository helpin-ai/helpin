// Shared table design tokens for consistent styling across all tables

/** Outer scrollable container */
export const TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto';

/** Sticky header bar */
export const TABLE_HEADER = 'sticky top-0 z-10 bg-card';

/** Individual header cell – compact, subtle text */
export const TABLE_HEADER_CELL =
  'ui-divider-bottom-fade ui-divider-right-fade relative shrink-0 px-2.5 py-1.5 text-left text-[11px] font-medium text-muted-foreground last:bg-none';

/** Sortable header cell – adds cursor pointer */
export const TABLE_HEADER_CELL_SORTABLE = 'cursor-pointer select-none hover:bg-muted/40';

/** Data row – compact h-9 (36px), `group` class for hover-reveal actions */
export const TABLE_ROW =
  'group flex h-9 items-center bg-card transition-colors hover:bg-[color-mix(in_oklab,var(--card)_50%,var(--muted)_50%)]';

/** Data cell – right border for grid lines, cell-level hover */
export const TABLE_CELL =
  'ui-divider-bottom-fade ui-divider-right-fade flex shrink-0 items-center px-2.5 self-stretch last:bg-none transition-colors';

/** Group header row (for grouped/expandable tables) */
export const TABLE_GROUP_ROW =
  'ui-divider-bottom-fade flex h-9 cursor-pointer items-center gap-2 bg-muted/20 px-3 text-sm font-semibold hover:bg-muted/40';

/** Column resize handle – always-visible 1px separator, expands on hover */
export const TABLE_RESIZE_HANDLE =
  'absolute right-0 top-0 h-full w-px cursor-col-resize touch-none select-none bg-border/40 hover:w-1 hover:bg-primary/30 active:bg-primary/50';

/** Row height for virtualizer estimateSize */
export const ROW_HEIGHT = 36;
export const GROUP_ROW_HEIGHT = 36;

/** Checkbox column width */
export const CHECKBOX_COL_SIZE = 40;

/** Actions column width */
export const ACTIONS_COL_SIZE = 44;

// --- Pinned column tokens ---

/** Pinned cell (left) – sticky with background so content doesn't bleed through */
export const TABLE_PINNED_LEFT =
  'sticky z-[2] bg-inherit';

/** Pinned cell (right) – sticky right with background */
export const TABLE_PINNED_RIGHT =
  'sticky right-0 z-[2] bg-inherit';

/** Pinned header cell (left) – higher z-index than both header and pinned cells */
export const TABLE_PINNED_HEADER_LEFT = 'sticky z-[11] bg-card';

/** Pinned header cell (right) – higher z-index, sticky right */
export const TABLE_PINNED_HEADER_RIGHT = 'sticky right-0 z-[11] bg-card';

/** Checkbox hover-reveal – hidden by default, visible on row hover or when checked */
export const TABLE_CHECKBOX_HOVER =
  'opacity-0 group-hover:opacity-100 transition-opacity data-[state=checked]:opacity-100 data-[state=indeterminate]:opacity-100';

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
