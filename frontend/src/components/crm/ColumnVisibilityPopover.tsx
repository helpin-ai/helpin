import type { Table } from '@tanstack/react-table';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Checkbox } from '@/components/ui/checkbox';
import { ViewIcon } from '@/lib/icons';

const NON_TOGGLEABLE = new Set(['select', 'actions']);

interface ColumnVisibilityPopoverProps<T> {
  table: Table<T>;
}

export function ColumnVisibilityPopover<T>({ table }: ColumnVisibilityPopoverProps<T>) {
  const columns = table
    .getAllLeafColumns()
    .filter((col) => !NON_TOGGLEABLE.has(col.id));

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          className="flex h-7 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs text-muted-foreground hover:bg-muted"
          title="Toggle columns"
        >
          <ViewIcon className="h-3.5 w-3.5" />
          Columns
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-[180px] p-2">
        <p className="mb-1.5 text-[11px] font-medium text-muted-foreground">
          Toggle columns
        </p>
        <div className="flex flex-col gap-1">
          {columns.map((col) => {
            const label =
              typeof col.columnDef.header === 'string'
                ? col.columnDef.header
                : col.id;
            return (
              <label
                key={col.id}
                className="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 text-xs hover:bg-muted"
              >
                <Checkbox
                  checked={col.getIsVisible()}
                  onCheckedChange={(checked) => col.toggleVisibility(!!checked)}
                />
                <span className="capitalize">{label}</span>
              </label>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
