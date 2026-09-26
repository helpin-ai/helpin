import type { Table, VisibilityState } from '@tanstack/react-table';
import { DisplaySettingsMenu } from '@/components/design-system/display-settings-menu';
const NON_TOGGLEABLE = new Set(['select', 'actions']);
export function ColumnVisibilityPopover<T>({
  table,
  visibilityState,
}: {
  table: Table<T>;
  visibilityState: VisibilityState;
}) {
  const columns = table
    .getAllLeafColumns()
    .filter((col) => !NON_TOGGLEABLE.has(col.id));
  return (
    <DisplaySettingsMenu
      options={columns.map((col) => ({
        value: col.id,
        label:
          typeof col.columnDef.header === 'string'
            ? col.columnDef.header
            : col.id,
      }))}
      selected={columns
        .filter((col) => visibilityState[col.id] !== false)
        .map((col) => col.id)}
      onToggle={(id) => {
        const col = columns.find((col) => col.id === id);
        col?.toggleVisibility(visibilityState[id] === false);
      }}
    />
  );
}
