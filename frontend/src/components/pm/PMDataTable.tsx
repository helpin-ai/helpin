import { useCallback, useState, useRef } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  flexRender,
  type ColumnDef,
  type VisibilityState,
  type OnChangeFn,
  type SortingState,
  type ColumnSizingState,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ArrowDown02Icon, ArrowUp02Icon, ArrowUpDownIcon } from '@/lib/icons';
import {
  TABLE_CONTAINER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_ROW,
  TABLE_CELL,
  TABLE_RESIZE_HANDLE,
  ROW_HEIGHT,
  dynamicCellStyle,
  resolveColumnRuntimeSize,
  virtualRowStyle,
} from '@/lib/tableStyles';

interface PMDataTableProps<T> {
  data: T[];
  columns: ColumnDef<T, any>[];
  columnVisibility?: VisibilityState;
  onColumnVisibilityChange?: OnChangeFn<VisibilityState>;
  onRowClick?: (row: T) => void;
  sorting?: SortingState;
  onSortingChange?: OnChangeFn<SortingState>;
  columnSizing?: ColumnSizingState;
  onColumnSizingChange?: OnChangeFn<ColumnSizingState>;
  hideHeader?: boolean;
  containerClassName?: string;
  bodyClassName?: string;
  bodyStyle?: React.CSSProperties;
}

export function PMDataTable<T>({
  data,
  columns,
  columnVisibility,
  onColumnVisibilityChange,
  onRowClick,
  sorting,
  onSortingChange,
  columnSizing,
  onColumnSizingChange,
  hideHeader = false,
  containerClassName,
  bodyClassName,
  bodyStyle,
}: PMDataTableProps<T>) {
  const [internalSorting, setInternalSorting] = useState<SortingState>([]);
  const [internalColumnSizing, setInternalColumnSizing] = useState<ColumnSizingState>({});
  const resolvedSorting = sorting ?? internalSorting;
  const resolvedColumnSizing = columnSizing ?? internalColumnSizing;

  const table = useReactTable({
    data,
    columns,
    state: {
      columnVisibility,
      sorting: resolvedSorting,
      columnSizing: resolvedColumnSizing,
    },
    onColumnVisibilityChange,
    onSortingChange: onSortingChange ?? setInternalSorting,
    onColumnSizingChange: onColumnSizingChange ?? setInternalColumnSizing,
    enableColumnResizing: true,
    columnResizeMode: 'onEnd',
    getSortedRowModel: getSortedRowModel(),
    getCoreRowModel: getCoreRowModel(),
  });
  const rows = table.getRowModel().rows;
  const bodyRef = useRef<HTMLDivElement>(null);
  const estimateSize = useCallback(() => ROW_HEIGHT, []);
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => bodyRef.current,
    estimateSize,
    overscan: 8,
  });

  return (
    <div className={containerClassName ?? TABLE_CONTAINER}>
      {/* Header */}
      {!hideHeader ? (
        <div className={TABLE_HEADER}>
          {table.getHeaderGroups().map((headerGroup) => (
            <div key={headerGroup.id} className="flex items-center">
              {headerGroup.headers.map((header) => {
                const defSize = header.column.columnDef.size ?? 150;
                const runtimeSize = header.getSize();
                const isResized = !!resolvedColumnSizing[header.column.id];
                const canSort = header.column.getCanSort();
                const sorted = header.column.getIsSorted();
                return (
                  <div
                    key={header.id}
                    className={`${TABLE_HEADER_CELL} ${canSort ? TABLE_HEADER_CELL_SORTABLE : ''}`}
                    style={dynamicCellStyle(defSize, runtimeSize, isResized, 200)}
                    onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                  >
                    <div className="flex items-center gap-1">
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                      {canSort && (
                        <span className="ml-auto shrink-0">
                          {sorted === 'asc' ? (
                            <ArrowUp02Icon className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                          ) : sorted === 'desc' ? (
                            <ArrowDown02Icon className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                          ) : (
                            <ArrowUpDownIcon className="h-3 w-3 text-muted-foreground stroke-[2]" />
                          )}
                        </span>
                      )}
                    </div>
                    {header.column.getCanResize() && (
                      <div
                        onMouseDown={header.getResizeHandler()}
                        onTouchStart={header.getResizeHandler()}
                        onClick={(e) => e.stopPropagation()}
                        className={`${TABLE_RESIZE_HANDLE} ${header.column.getIsResizing() ? 'bg-primary/50' : ''}`}
                      />
                    )}
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      ) : null}

      {/* Body */}
      <div ref={bodyRef} className={bodyClassName ?? 'overflow-auto'} style={bodyStyle ?? { maxHeight: 'calc(100vh - 220px)' }}>
        <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
          {virtualizer.getVirtualItems().map((virtualRow) => {
            const row = rows[virtualRow.index];
            if (!row) return null;
            return (
              <div
                key={row.id}
                data-index={virtualRow.index}
                className={`${TABLE_ROW}${onRowClick ? ' cursor-pointer' : ''}`}
                onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                style={virtualRowStyle(virtualRow.start)}
              >
                {row.getVisibleCells().map((cell) => {
                  const { defSize, runtimeSize, isResized } = resolveColumnRuntimeSize(cell.column, resolvedColumnSizing);
                  return (
                    <div
                      key={cell.id}
                      className={`${TABLE_CELL} overflow-hidden`}
                      style={dynamicCellStyle(defSize, runtimeSize, isResized, 200)}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </div>
                  );
                })}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
