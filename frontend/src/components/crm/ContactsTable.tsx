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
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronDown, ChevronRight, EllipsisVertical, ExternalLink, Loader2, Plus, Trash2, UserPlus, Users } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { format, parseISO } from 'date-fns';
import { crmContactService } from '@/lib/services/crmService';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { findAssignableMember } from '@/lib/assignableMembers';
import {
  TABLE_CONTAINER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_HEADER_CELL_SORTABLE,
  TABLE_ROW,
  TABLE_CELL,
  TABLE_GROUP_ROW,
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
import type { CRMContact, LifecycleStage, LeadStatus } from '@/lib/crmTypes';
import type { AssignableMember } from '@/lib/types';

type GroupByOption = 'none' | 'lifecycle_stage' | 'lead_status' | 'owner';

const GROUP_BY_OPTIONS: { value: GroupByOption; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'lifecycle_stage', label: 'Lifecycle Stage' },
  { value: 'lead_status', label: 'Lead Status' },
  { value: 'owner', label: 'Owner' },
];

const GROUP_COLUMN_MAP: Record<GroupByOption, string | null> = {
  none: null,
  lifecycle_stage: 'lifecycleStageName',
  lead_status: 'leadStatusName',
  owner: 'ownerName',
};

const lifecycleColors: Record<string, string> = {
  subscriber: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  lead: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  marketing_qualified: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  sales_qualified: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300',
  opportunity: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
  customer: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
  evangelist: 'bg-pink-100 text-pink-700 dark:bg-pink-900 dark:text-pink-300',
};

const LIFECYCLE_STAGES: { value: LifecycleStage; label: string }[] = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const LEAD_STATUSES: { value: LeadStatus; label: string }[] = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

const columnHelper = createColumnHelper<CRMContact>();

interface ContactsTableProps {
  contacts: CRMContact[];
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  isLoading: boolean;
  onRowClick: (id: string) => void;
  onCreateClick?: () => void;
  onContactUpdated?: () => void;
  onContactDeleted?: () => void;
}

export function ContactsTable({
  contacts,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  isLoading,
  onRowClick,
  onCreateClick,
  onContactUpdated,
  onContactDeleted,
}: ContactsTableProps) {
  const [localContacts, setLocalContacts] = useState<CRMContact[]>(contacts);
  const [groupBy, setGroupBy] = useState<GroupByOption>('none');
  const [expanded, setExpanded] = useState<ExpandedState>(true);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>({});
  const columnSizingVersion = useMemo(() => JSON.stringify(columnSizing), [columnSizing]);
  const parentRef = useRef<HTMLDivElement>(null);

  useEffect(() => { setLocalContacts(contacts); }, [contacts]);

  const updateContactField = useCallback(
    async (contactId: string, patch: Partial<CRMContact>) => {
      let snapshot: CRMContact[] = [];
      setLocalContacts((current) => {
        snapshot = current;
        return current.map((c) => (c.id === contactId ? { ...c, ...patch } : c));
      });

      const { display_id: _did, workspace_id: _wid, custom_properties: _cp, created_at: _ca, updated_at: _ua, id: _id, ...apiSafe } = patch as Record<string, unknown>;
      const { error } = await crmContactService.update(workspaceId, contactId, apiSafe);
      if (error) setLocalContacts(snapshot);
      else onContactUpdated?.();
    },
    [workspaceId, onContactUpdated],
  );

  const handleDelete = useCallback(async (contactId: string) => {
    const { error } = await crmContactService.remove(workspaceId, contactId);
    if (!error) {
      setLocalContacts((current) => current.filter((c) => c.id !== contactId));
      onContactDeleted?.();
    }
  }, [workspaceId, onContactDeleted]);

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
      columnHelper.accessor(
        (row) => `${row.first_name} ${row.last_name ?? ''}`.trim(),
        {
          id: 'name',
          header: 'Name',
          size: 999,
          enableGrouping: false,
          cell: (info) => {
            const fullName = info.getValue();
            return (
              <button
                className="flex max-w-full cursor-pointer items-center gap-2 truncate text-left text-sm hover:text-primary"
                onClick={(e) => {
                  e.stopPropagation();
                  onRowClick(info.row.original.id);
                }}
              >
                <UserAvatar name={fullName} className="h-6 w-6 shrink-0" />
                <span className="truncate">{fullName}</span>
              </button>
            );
          },
        }
      ),
      columnHelper.accessor('email', {
        id: 'email',
        header: 'Email',
        size: 200,
        enableGrouping: false,
        cell: (info) => (
          <span className="truncate text-xs text-muted-foreground">{info.getValue() ?? '-'}</span>
        ),
      }),
      columnHelper.accessor('phone', {
        id: 'phone',
        header: 'Phone',
        size: 140,
        enableGrouping: false,
        cell: (info) => (
          <span className="truncate text-xs text-muted-foreground">{info.getValue() ?? '-'}</span>
        ),
      }),
      columnHelper.accessor(
        (row) => row.lifecycle_stage.replace(/_/g, ' '),
        {
          id: 'lifecycleStageName',
          header: 'Stage',
          size: 160,
          cell: (info) => (
            <InlineLifecycleCell
              contact={info.row.original}
              onUpdate={updateContactField}
            />
          ),
        }
      ),
      columnHelper.accessor(
        (row) => row.lead_status.replace(/_/g, ' '),
        {
          id: 'leadStatusName',
          header: 'Status',
          size: 140,
          cell: (info) => (
            <InlineLeadStatusCell
              contact={info.row.original}
              onUpdate={updateContactField}
            />
          ),
        }
      ),
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
              contact={info.row.original}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onUpdate={updateContactField}
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
            contact={info.row.original}
            onOpen={onRowClick}
            onDelete={handleDelete}
          />
        ),
      }),
    ],
    [ownerNameMap, assignableMembers, onRowClick, updateContactField, handleDelete],
  );

  const grouping: GroupingState = useMemo(() => {
    const colId = GROUP_COLUMN_MAP[groupBy];
    return colId ? [colId] : [];
  }, [groupBy]);

  const table = useReactTable({
    data: localContacts,
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
        Loading contacts...
      </div>
    );
  }

  if (contacts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <Users className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No contacts yet</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Add your first contact to start building relationships
        </p>
        {onCreateClick && (
          <Button size="sm" className="mt-4" onClick={onCreateClick}>
            <Plus className="mr-1 h-4 w-4" />
            Create Contact
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
          <SelectTrigger className="h-7 w-[160px] text-xs">
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
          {localContacts.length} {localContacts.length === 1 ? 'contact' : 'contacts'}
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

          {/* Virtualized body */}
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const row = rows[virtualRow.index] as Row<CRMContact>;
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
                    <MemoDataRow row={row} columnSizingVersion={columnSizingVersion} />
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

const MemoGroupHeaderRow = memo(function GroupHeaderRow({ row }: { row: Row<CRMContact> }) {
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
      <span className="capitalize">{groupValue}</span>
      <span className="ml-2 text-xs font-normal text-muted-foreground">
        {count} {count === 1 ? 'contact' : 'contacts'}
      </span>
    </div>
  );
});

// ── Data Row ──────────────────────────────────────────────────────

const MemoDataRow = memo(function DataRow({
  row,
  columnSizingVersion,
}: {
  row: Row<CRMContact>;
  columnSizingVersion: string;
}) {
  return (
    <div className={TABLE_ROW} data-column-sizing={columnSizingVersion}>
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

function InlineLifecycleCell({
  contact,
  onUpdate,
}: {
  contact: CRMContact;
  onUpdate: (id: string, patch: Partial<CRMContact>) => void;
}) {
  return (
    <Select
      value={contact.lifecycle_stage}
      onValueChange={(value) => onUpdate(contact.id, { lifecycle_stage: value as LifecycleStage })}
    >
      <SelectTrigger
        className="h-6 w-full gap-1 border-none bg-transparent px-1 text-xs shadow-none hover:bg-muted"
        onClick={(e) => e.stopPropagation()}
      >
        <Badge variant="outline" className={`text-[10px] px-1.5 py-0 ${lifecycleColors[contact.lifecycle_stage] ?? ''}`}>
          {contact.lifecycle_stage.replace(/_/g, ' ')}
        </Badge>
      </SelectTrigger>
      <SelectContent>
        {LIFECYCLE_STAGES.map((s) => (
          <SelectItem key={s.value} value={s.value}>
            <Badge variant="outline" className={`text-[10px] px-1.5 py-0 ${lifecycleColors[s.value] ?? ''}`}>
              {s.label}
            </Badge>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function InlineLeadStatusCell({
  contact,
  onUpdate,
}: {
  contact: CRMContact;
  onUpdate: (id: string, patch: Partial<CRMContact>) => void;
}) {
  return (
    <Select
      value={contact.lead_status}
      onValueChange={(value) => onUpdate(contact.id, { lead_status: value as LeadStatus })}
    >
      <SelectTrigger
        className="h-6 w-full gap-1 border-none bg-transparent px-1 text-xs shadow-none hover:bg-muted"
        onClick={(e) => e.stopPropagation()}
      >
        <span className="truncate capitalize">{contact.lead_status.replace(/_/g, ' ')}</span>
      </SelectTrigger>
      <SelectContent>
        {LEAD_STATUSES.map((s) => (
          <SelectItem key={s.value} value={s.value}>
            {s.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function InlineOwnerCell({
  contact,
  assignableMembers,
  ownerNameMap,
  onUpdate,
}: {
  contact: CRMContact;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onUpdate: (id: string, patch: Partial<CRMContact>) => void;
}) {
  const ownerName = contact.owner_member_id ? ownerNameMap.get(contact.owner_member_id) ?? 'Unknown' : null;

  return (
    <MemberPickerPopover
      value={contact.owner_member_id || '__none__'}
      members={assignableMembers}
      noneLabel="Unassigned"
      onChange={(value) => {
        onUpdate(contact.id, { owner_member_id: value === '__none__' ? '' : value });
      }}
      triggerClassName="flex w-full items-center gap-1.5 truncate text-xs hover:text-primary"
      contentClassName="w-[220px]"
      renderTrigger={() => {
        const selectedMember = findAssignableMember(assignableMembers, contact.owner_member_id);
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
  contact,
  onOpen,
  onDelete,
}: {
  contact: CRMContact;
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
        <DropdownMenuItem onClick={() => onOpen(contact.id)}>
          <ExternalLink className="mr-2 h-3.5 w-3.5" />
          Open
        </DropdownMenuItem>
        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => onDelete(contact.id)}
        >
          <Trash2 className="mr-2 h-3.5 w-3.5" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
