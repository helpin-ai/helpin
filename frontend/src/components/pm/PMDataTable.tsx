import { useState } from 'react';
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
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';
import {
  TABLE_CONTAINER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_ROW,
  TABLE_CELL,
  TABLE_RESIZE_HANDLE,
  dynamicCellStyle,
} from '@/lib/tableStyles';

interface PMDataTableProps<T> {
  data: T[];
  columns: ColumnDef<T, any>[];
  columnVisibility?: VisibilityState;
  onColumnVisibilityChange?: OnChangeFn<VisibilityState>;
  onRowClick?: (row: T) => void;
}

export function PMDataTable<T>({
  data,
  columns,
  columnVisibility,
  onColumnVisibilityChange,
  onRowClick,
}: PMDataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>({});

  const table = useReactTable({
    data,
    columns,
    state: {
      columnVisibility,
      sorting,
      columnSizing,
    },
    onColumnVisibilityChange,
    onSortingChange: setSorting,
    onColumnSizingChange: setColumnSizing,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    getSortedRowModel: getSortedRowModel(),
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className={`${TABLE_CONTAINER} overflow-hidden`}>
      {/* Header */}
      <div className={TABLE_HEADER}>
        {table.getHeaderGroups().map((headerGroup) => (
          <div key={headerGroup.id} className="flex items-center">
            {headerGroup.headers.map((header) => {
              const defSize = header.column.columnDef.size ?? 150;
              const runtimeSize = header.getSize();
              const isResized = !!columnSizing[header.column.id];
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
                          <ArrowUp className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                        ) : sorted === 'desc' ? (
                          <ArrowDown className="h-3 w-3 text-foreground/80 stroke-[2.5]" />
                        ) : (
                          <ArrowUpDown className="h-3 w-3 text-muted-foreground stroke-[2]" />
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

      {/* Body */}
      <div className="overflow-auto" style={{ maxHeight: 'calc(100vh - 220px)' }}>
        {table.getRowModel().rows.map((row) => (
          <div
            key={row.id}
            className={`${TABLE_ROW}${onRowClick ? ' cursor-pointer' : ''}`}
            onClick={onRowClick ? () => onRowClick(row.original) : undefined}
          >
            {row.getVisibleCells().map((cell) => {
              const defSize = cell.column.columnDef.size ?? 150;
              const runtimeSize = cell.column.getSize();
              const isResized = !!columnSizing[cell.column.id];
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
        ))}
      </div>
    </div>
  );
}
