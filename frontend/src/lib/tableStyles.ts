// Shared table design tokens for consistent styling across all tables

/** Outer scrollable container (replaces rounded-md border border-border/70) */
export const TABLE_CONTAINER = 'min-h-0 flex-1 overflow-auto';

/** Sticky header bar – darker background to stand out from data rows */
export const TABLE_HEADER = 'sticky top-0 z-10 border-b border-border/60 bg-muted/80';

/** Individual header cell – always left-aligned, right border for grid lines */
export const TABLE_HEADER_CELL = 'relative px-3 py-2.5 text-left text-xs font-semibold text-foreground/70 border-r border-border/60 last:border-r-0';

/** Sortable header cell – adds cursor pointer */
export const TABLE_HEADER_CELL_SORTABLE = 'cursor-pointer select-none hover:bg-muted/40';

/** Data row – taller (h-11 = 44px), `group` class for hover-reveal actions */
export const TABLE_ROW = 'group flex h-11 items-center border-b border-border/50 transition-colors hover:bg-muted/70';

/** Data cell – always left-aligned, right border for grid lines */
export const TABLE_CELL = 'flex items-center px-3 self-stretch border-r border-border/60 last:border-r-0';

/** Group header row (for grouped/expandable tables) */
export const TABLE_GROUP_ROW =
  'flex h-10 cursor-pointer items-center gap-2 border-b border-border/50 bg-muted/20 px-3 text-sm font-semibold hover:bg-muted/40';

/** Column resize handle */
export const TABLE_RESIZE_HANDLE =
  'absolute right-0 top-0 h-full w-1 cursor-col-resize touch-none select-none bg-transparent hover:bg-primary/30 active:bg-primary/50';

/** Row height for virtualizer estimateSize */
export const ROW_HEIGHT = 44;
export const GROUP_ROW_HEIGHT = 40;

/** Checkbox column width */
export const CHECKBOX_COL_SIZE = 40;

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
