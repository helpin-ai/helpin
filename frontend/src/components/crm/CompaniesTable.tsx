import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  getGroupedRowModel,
  getExpandedRowModel,
  getSortedRowModel,
  flexRender,
  createColumnHelper,
  type GroupingState,
  type ExpandedState,
  type Row,
  type RowSelectionState,
  type SortingState,
  type ColumnSizingState,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { ArrowDown, ArrowUp, ArrowUpDown, Building2, ChevronDown, ChevronRight, EllipsisVertical, ExternalLink, Loader2, Plus, Trash2, UserPlus } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Button } from '@/components/ui/button';
import { format, parseISO } from 'date-fns';
import { crmCompanyService } from '@/lib/services/crmService';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { findAssignableMember } from '@/lib/assignableMembers';
import {
  TABLE_CONTAINER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_ROW,
  TABLE_CELL,
  TABLE_GROUP_ROW,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_RESIZE_HANDLE,
  TABLE_PINNED_LEFT,
  TABLE_PINNED_RIGHT,
  TABLE_PINNED_HEADER_LEFT,
  TABLE_PINNED_HEADER_RIGHT,
  TABLE_CHECKBOX_HOVER,
  ROW_HEIGHT,
  GROUP_ROW_HEIGHT,
  CHECKBOX_COL_SIZE,
  dynamicCellStyle,
  pinnedStyle,
} from '@/lib/tableStyles';
import type { CRMCompany } from '@/lib/crmTypes';
import type { AssignableMember } from '@/lib/types';

type GroupByOption = 'none' | 'industry' | 'owner';

const GROUP_BY_OPTIONS: { value: GroupByOption; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'industry', label: 'Industry' },
  { value: 'owner', label: 'Owner' },
];

const GROUP_COLUMN_MAP: Record<GroupByOption, string | null> = {
  none: null,
  industry: 'industryName',
  owner: 'ownerName',
};

const columnHelper = createColumnHelper<CRMCompany>();

interface CompaniesTableProps {
  companies: CRMCompany[];
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  isLoading: boolean;
  onRowClick: (id: string) => void;
  onCreateClick?: () => void;
  onCompanyUpdated?: () => void;
  onCompanyDeleted?: () => void;
}

export function CompaniesTable({
  companies,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  isLoading,
  onRowClick,
  onCreateClick,
  onCompanyUpdated,
  onCompanyDeleted,
}: CompaniesTableProps) {
  const [localCompanies, setLocalCompanies] = useState<CRMCompany[]>(companies);
  const [groupBy, setGroupBy] = useState<GroupByOption>('none');
  const [expanded, setExpanded] = useState<ExpandedState>(true);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>({});
  const parentRef = useRef<HTMLDivElement>(null);

  useEffect(() => { setLocalCompanies(companies); }, [companies]);

  const updateCompanyField = useCallback(
    async (companyId: string, patch: Partial<CRMCompany>) => {
      let snapshot: CRMCompany[] = [];
      setLocalCompanies((current) => {
        snapshot = current;
        return current.map((c) => (c.id === companyId ? { ...c, ...patch } : c));
      });

      const { display_id: _did, workspace_id: _wid, custom_properties: _cp, created_at: _ca, updated_at: _ua, id: _id, ...apiSafe } = patch as Record<string, unknown>;
      const { error } = await crmCompanyService.update(workspaceId, companyId, apiSafe);
      if (error) setLocalCompanies(snapshot);
      else onCompanyUpdated?.();
    },
    [workspaceId, onCompanyUpdated],
  );

  const handleDelete = useCallback(async (companyId: string) => {
    const { error } = await crmCompanyService.remove(workspaceId, companyId);
    if (!error) {
      setLocalCompanies((current) => current.filter((c) => c.id !== companyId));
      onCompanyDeleted?.();
    }
  }, [workspaceId, onCompanyDeleted]);

  const tableColumns = useMemo(
    () => [
      columnHelper.display({
        id: 'select',
        size: CHECKBOX_COL_SIZE,
        enableGrouping: false,
        enableSorting: false,
        enableResizing: false,
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
            aria-label="Select all"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            className={TABLE_CHECKBOX_HOVER}
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            onClick={(e) => e.stopPropagation()}
            aria-label="Select row"
          />
        ),
      }),
      columnHelper.accessor('display_id', {
        id: 'displayId',
        header: 'ID',
        size: 90,
        cell: (info) => (
          <span className="font-mono text-xs text-muted-foreground">{info.getValue()}</span>
        ),
      }),
      columnHelper.accessor('name', {
        id: 'name',
        header: 'Name',
        size: 999,
        enableGrouping: false,
        cell: (info) => (
          <button
            className="flex max-w-full cursor-pointer items-center gap-2 truncate text-left text-sm hover:text-primary"
            onClick={(e) => {
              e.stopPropagation();
              onRowClick(info.row.original.id);
            }}
          >
            <UserAvatar name={info.getValue()} className="h-6 w-6 shrink-0" />
            <span className="truncate">{info.getValue()}</span>
          </button>
        ),
      }),
      columnHelper.accessor('domain', {
        id: 'domain',
        header: 'Domain',
        size: 180,
        enableGrouping: false,
        cell: (info) => {
          const domain = info.getValue();
          return domain ? (
            <span className="cursor-pointer truncate text-xs text-primary">{domain}</span>
          ) : (
            <span className="text-xs text-muted-foreground">-</span>
          );
        },
      }),
      columnHelper.accessor(
        (row) => row.industry ?? 'Unknown',
        {
          id: 'industryName',
          header: 'Industry',
          size: 150,
          cell: (info) => (
            <span className="truncate text-xs text-muted-foreground">{info.row.original.industry ?? '-'}</span>
          ),
        }
      ),
      columnHelper.accessor('employee_count', {
        id: 'employeeCount',
        header: 'Employees',
        size: 120,
        enableGrouping: false,
        cell: (info) => {
          const val = info.getValue();
          return (
            <span className="text-xs text-muted-foreground">
              {val != null ? new Intl.NumberFormat().format(val) : '-'}
            </span>
          );
        },
      }),
      columnHelper.accessor('annual_revenue', {
        id: 'annualRevenue',
        header: 'Revenue',
        size: 140,
        enableGrouping: false,
        cell: (info) => {
          const val = info.getValue();
          return (
            <span className="text-xs text-muted-foreground">
              {val != null ? `$${new Intl.NumberFormat().format(val)}` : '-'}
            </span>
          );
        },
      }),
      columnHelper.accessor(
        (row) => {
          const ownerKey = row.owner_member_id;
          return ownerKey ? ownerNameMap.get(ownerKey) ?? 'Unknown' : 'Unassigned';
        },
        {
          id: 'ownerName',
          header: 'Owner',
          size: 180,
          cell: (info) => (
            <InlineOwnerCell
              company={info.row.original}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onUpdate={updateCompanyField}
            />
          ),
        }
      ),
      columnHelper.accessor('created_at', {
        id: 'createdAt',
        header: 'Created',
        size: 160,
        enableGrouping: false,
        cell: (info) => {
          const val = info.getValue();
          if (!val) return null;
          return (
            <span className="text-xs text-muted-foreground whitespace-nowrap">
              {format(parseISO(val), 'MMM d, yyyy, h:mm a')}
            </span>
          );
        },
      }),
      columnHelper.display({
        id: 'actions',
        header: '',
        size: 44,
        enableGrouping: false,
        enableSorting: false,
        enableResizing: false,
        cell: (info) => (
          <InlineActionsCell
            company={info.row.original}
            onOpen={onRowClick}
            onDelete={handleDelete}
          />
        ),
      }),
    ],
    [ownerNameMap, assignableMembers, onRowClick, updateCompanyField, handleDelete],
  );

  const grouping: GroupingState = useMemo(() => {
    const colId = GROUP_COLUMN_MAP[groupBy];
    return colId ? [colId] : [];
  }, [groupBy]);

  const table = useReactTable({
    data: localCompanies,
    columns: tableColumns,
    state: {
      grouping,
      expanded,
      rowSelection,
      sorting,
      columnSizing,
    },
    onExpandedChange: setExpanded,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnSizingChange: setColumnSizing,
    enableRowSelection: true,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    autoResetExpanded: false,
    getRowId: (row) => row.id,
    getExpandedRowModel: getExpandedRowModel(),
    getGroupedRowModel: getGroupedRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getCoreRowModel: getCoreRowModel(),
  });

  const { rows } = table.getRowModel();

  const estimateSize = useCallback(
    (index: number) => rows[index]?.getIsGrouped() ? GROUP_ROW_HEIGHT : ROW_HEIGHT,
    [rows],
  );

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize,
    overscan: 20,
  });

  if (isLoading) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        Loading companies...
      </div>
    );
  }

  if (companies.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Building2 className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No companies yet</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Add your first company to track organizations
        </p>
        {onCreateClick && (
          <Button size="sm" className="mt-4" onClick={onCreateClick}>
            <Plus className="mr-1 h-4 w-4" />
            Create Company
          </Button>
        )}
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      {/* Group By control */}
      <div className="flex items-center gap-2 px-1">
        <span className="text-xs text-muted-foreground">Group by:</span>
        <Select value={groupBy} onValueChange={(v) => setGroupBy(v as GroupByOption)}>
          <SelectTrigger className="h-7 w-[140px] text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {GROUP_BY_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <span className="text-xs text-muted-foreground">
          {localCompanies.length} {localCompanies.length === 1 ? 'company' : 'companies'}
        </span>
      </div>

      {/* Table */}
      <div ref={parentRef} className={TABLE_CONTAINER}>
        <div className="min-w-fit">
          {/* Header */}
          <div className={TABLE_HEADER}>
            {table.getHeaderGroups().map((headerGroup) => (
              <div key={headerGroup.id} className="flex items-center">
                {headerGroup.headers.map((header) => {
                  if (header.column.getIsGrouped()) return null;
                  const defSize = header.column.columnDef.size ?? 150;
                  const runtimeSize = header.getSize();
                  const isResized = !!columnSizing[header.column.id];
                  const canSort = header.column.getCanSort();
                  const sorted = header.column.getIsSorted();
                  const colId = header.column.id;
                  const pinnedClass = colId === 'select' ? TABLE_PINNED_HEADER_LEFT
                    : colId === 'actions' ? TABLE_PINNED_HEADER_RIGHT : '';
                  const pinnedSt = colId === 'select' ? pinnedStyle('left', 0)
                    : colId === 'actions' ? pinnedStyle('right', 0) : {};
                  return (
                    <div
                      key={header.id}
                      className={`${TABLE_HEADER_CELL} ${canSort ? TABLE_HEADER_CELL_SORTABLE : ''} ${pinnedClass}`}
                      style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 300), ...pinnedSt }}
                      onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                    >
                      <div className="flex items-center gap-1">
                        {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
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

          {/* Virtualized body */}
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const row = rows[virtualRow.index] as Row<CRMCompany>;
              const isGrouped = row.getIsGrouped();

              return (
                <div
                  key={row.id}
                  data-index={virtualRow.index}
                  ref={virtualizer.measureElement}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                >
                  {isGrouped ? (
                    <MemoGroupHeaderRow row={row} />
                  ) : (
                    <MemoDataRow row={row} />
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Group Header Row ──────────────────────────────────────────────

const MemoGroupHeaderRow = memo(function GroupHeaderRow({ row }: { row: Row<CRMCompany> }) {
  const subRows = row.subRows;
  const count = subRows.length;
  const groupValue = row.groupingValue as string;

  return (
    <div
      className={TABLE_GROUP_ROW}
      onClick={() => row.toggleExpanded()}
    >
      {row.getIsExpanded() ? (
        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
      ) : (
        <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
      )}
      <span>{groupValue}</span>
      <span className="ml-2 text-xs font-normal text-muted-foreground">
        {count} {count === 1 ? 'company' : 'companies'}
      </span>
    </div>
  );
});

// ── Data Row ──────────────────────────────────────────────────────

const MemoDataRow = memo(function DataRow({ row }: { row: Row<CRMCompany> }) {
  return (
    <div className={TABLE_ROW}>
      {row.getVisibleCells().map((cell) => {
        if (cell.column.getIsGrouped()) return null;
        const defSize = cell.column.columnDef.size ?? 150;
        const runtimeSize = cell.column.getSize();
        const isResized = runtimeSize !== defSize;
        const colId = cell.column.id;
        const pinnedClass = colId === 'select' ? TABLE_PINNED_LEFT
          : colId === 'actions' ? TABLE_PINNED_RIGHT : '';
        const pinnedSt = colId === 'select' ? pinnedStyle('left', 0)
          : colId === 'actions' ? pinnedStyle('right', 0) : {};
        return (
          <div
            key={cell.id}
            className={`${TABLE_CELL} ${pinnedClass}`}
            style={{ ...dynamicCellStyle(defSize, runtimeSize, isResized, 300), ...pinnedSt }}
          >
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </div>
        );
      })}
    </div>
  );
});

// ── Inline Editing Cells ──────────────────────────────────────────

function InlineOwnerCell({
  company,
  assignableMembers,
  ownerNameMap,
  onUpdate,
}: {
  company: CRMCompany;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onUpdate: (id: string, patch: Partial<CRMCompany>) => void;
}) {
  const ownerName = company.owner_member_id ? ownerNameMap.get(company.owner_member_id) ?? 'Unknown' : null;

  return (
    <MemberPickerPopover
      value={company.owner_member_id || '__none__'}
      members={assignableMembers}
      noneLabel="Unassigned"
      onChange={(value) => {
        onUpdate(company.id, { owner_member_id: value === '__none__' ? '' : value });
      }}
      triggerClassName="flex w-full items-center gap-1.5 truncate text-xs hover:text-primary"
      contentClassName="w-[220px]"
      renderTrigger={() => {
        const selectedMember = findAssignableMember(assignableMembers, company.owner_member_id);
        return selectedMember ? (
          <>
            <UserAvatar
              name={selectedMember.display_name || selectedMember.email}
              avatarUrl={selectedMember.avatar_url}
              className="h-5 w-5 shrink-0"
            />
            <span className="truncate">{ownerName}</span>
          </>
        ) : (
          <span className="flex items-center gap-1 text-muted-foreground">
            <UserPlus className="h-3 w-3" /> Assign
          </span>
        );
      }}
    />
  );
}

function InlineActionsCell({
  company,
  onOpen,
  onDelete,
}: {
  company: CRMCompany;
  onOpen: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity hover:bg-muted group-hover:opacity-100"
          onClick={(e) => e.stopPropagation()}
        >
          <EllipsisVertical className="h-3.5 w-3.5" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-[140px]">
        <DropdownMenuItem onClick={() => onOpen(company.id)}>
          <ExternalLink className="mr-2 h-3.5 w-3.5" />
          Open
        </DropdownMenuItem>
        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => onDelete(company.id)}
        >
          <Trash2 className="mr-2 h-3.5 w-3.5" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
