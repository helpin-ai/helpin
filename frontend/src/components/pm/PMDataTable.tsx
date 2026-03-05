import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
  type VisibilityState,
  type OnChangeFn,
} from '@tanstack/react-table';

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
  const table = useReactTable({
    data,
    columns,
    state: {
      columnVisibility,
    },
    onColumnVisibilityChange,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="min-h-0 flex-1 overflow-hidden rounded-md border border-border/70">
      {/* Header */}
      <div className="border-b border-border/70 bg-muted/50">
        {table.getHeaderGroups().map((headerGroup) => (
          <div key={headerGroup.id} className="flex items-center">
            {headerGroup.headers.map((header) => {
              const size = header.getSize();
              return (
                <div
                  key={header.id}
                  className="px-2 py-1.5 text-xs font-medium text-muted-foreground"
                  style={{
                    width: size === 999 ? undefined : size,
                    flex: size === 999 ? '1 1 0%' : undefined,
                    minWidth: size === 999 ? 200 : undefined,
                  }}
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
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
            className={`flex items-center border-b border-border/30 transition-colors hover:bg-muted/30${
              onRowClick ? ' cursor-pointer' : ''
            }`}
            onClick={onRowClick ? () => onRowClick(row.original) : undefined}
          >
            {row.getVisibleCells().map((cell) => {
              const size = cell.column.getSize();
              return (
                <div
                  key={cell.id}
                  className="overflow-hidden px-2 py-1.5"
                  style={{
                    width: size === 999 ? undefined : size,
                    flex: size === 999 ? '1 1 0%' : undefined,
                    minWidth: size === 999 ? 200 : undefined,
                  }}
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
