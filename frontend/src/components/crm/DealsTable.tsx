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
import { ArrowDown02Icon, ArrowUp02Icon, ArrowUpDownIcon, Calendar03Icon, ArrowDown01Icon, ArrowRight01Icon, DollarCircleIcon, MoreVerticalIcon, LinkSquare01Icon, Loading01Icon, PlusSignIcon, Delete01Icon, UserAdd01Icon } from '@/lib/icons';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Calendar } from '@/components/ui/calendar';
import { Checkbox } from '@/components/ui/checkbox';
import { format, parseISO } from 'date-fns';
import { crmDealService } from '@/lib/services/crmService';
import { StageTypeIcon, STAGE_TYPE_CONFIG } from '@/lib/crmConstants';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Button } from '@/components/ui/button';
import { useDealDisplayStore } from '@/stores/dealDisplayStore';
import { MemberPickerPopover } from '@/components/pm/MemberPickerPopover';
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
import type { CRMDeal, CRMPipeline, CRMPipelineStage } from '@/lib/crmTypes';
import type { AssignableMember } from '@/lib/types';

type GroupByOption = 'none' | 'stage' | 'owner' | 'stage_type';

const GROUP_BY_OPTIONS: { value: GroupByOption; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'stage', label: 'Stage' },
  { value: 'owner', label: 'Owner' },
  { value: 'stage_type', label: 'Stage Type' },
];

const GROUP_COLUMN_MAP: Record<GroupByOption, string | null> = {
  none: null,
  stage: 'stageName',
  owner: 'ownerName',
  stage_type: 'stageTypeName',
};

const HIDDEN_GROUP_COLUMNS = ['stageTypeName'];

const columnHelper = createColumnHelper<CRMDeal>();

interface DealsTableProps {
  deals: CRMDeal[];
  pipeline?: CRMPipeline;
  workspaceId: string;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  isLoading: boolean;
  onDealClick: (id: string) => void;
  onCreateClick?: () => void;
  onDealUpdated?: (deal: CRMDeal) => void;
  onDealDeleted?: (dealId: string) => void;
}

export function DealsTable({
  deals,
  pipeline,
  workspaceId,
  assignableMembers,
  ownerNameMap,
  isLoading,
  onDealClick,
  onCreateClick,
  onDealUpdated,
  onDealDeleted,
}: DealsTableProps) {
  const displayProps = useDealDisplayStore((s) => s.properties);
  const [localDeals, setLocalDeals] = useState<CRMDeal[]>(deals);
  const [groupBy, setGroupBy] = useState<GroupByOption>('none');
  const [expanded, setExpanded] = useState<ExpandedState>(true);
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>({});
  const columnSizingVersion = useMemo(() => JSON.stringify(columnSizing), [columnSizing]);
  const parentRef = useRef<HTMLDivElement>(null);

  useEffect(() => { setLocalDeals(deals); }, [deals]);

  const stages = useMemo(() => {
    if (!pipeline?.stages) return [];
    return [...pipeline.stages].sort((a, b) => a.position - b.position);
  }, [pipeline?.stages]);

  const stageMap = useMemo(() => {
    const map = new Map<string, CRMPipelineStage>();
    for (const s of stages) map.set(s.id, s);
    return map;
  }, [stages]);

  const updateDealField = useCallback(
    async (dealId: string, patch: Partial<CRMDeal>) => {
      let snapshot: CRMDeal[] = [];
      const optimisticPatch: Partial<CRMDeal> = { ...patch };
      if (patch.stage_id) {
        const stage = stageMap.get(patch.stage_id);
        if (stage) optimisticPatch.stage = stage;
      }

      setLocalDeals((current) => {
        snapshot = current;
        return current.map((d) => (d.id === dealId ? { ...d, ...optimisticPatch } : d));
      });

      const { stage: _stage, pipeline: _pipeline, display_id: _did, ...apiSafe } = patch as Record<string, unknown>;
      const { error } = await crmDealService.update(workspaceId, dealId, apiSafe);
      if (error) setLocalDeals(snapshot);
      else onDealUpdated?.({ ...snapshot.find((d) => d.id === dealId)!, ...optimisticPatch } as CRMDeal);
    },
    [workspaceId, stageMap, onDealUpdated],
  );

  const handleDelete = useCallback(async (dealId: string) => {
    const { error } = await crmDealService.remove(workspaceId, dealId);
    if (!error) {
      setLocalDeals((current) => current.filter((d) => d.id !== dealId));
      onDealDeleted?.(dealId);
    }
  }, [workspaceId, onDealDeleted]);

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
              onDealClick(info.row.original.id);
            }}
          >
            <UserAvatar name={info.getValue()} className="h-6 w-6 shrink-0" />
            <span className="truncate">{info.getValue()}</span>
          </button>
        ),
      }),
      columnHelper.accessor(
        (row) => stageMap.get(row.stage_id)?.name ?? 'Unknown',
        {
          id: 'stageName',
          header: 'Stage',
          size: 160,
          cell: (info) => (
            <InlineStageCell
              deal={info.row.original}
              stages={stages}
              stageMap={stageMap}
              onUpdate={updateDealField}
            />
          ),
        }
      ),
      columnHelper.accessor('amount', {
        id: 'amount',
        header: 'Amount',
        size: 130,
        enableGrouping: false,
        cell: (info) => (
          <InlineAmountCell
            deal={info.row.original}
            onUpdate={updateDealField}
          />
        ),
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
              deal={info.row.original}
              assignableMembers={assignableMembers}
              ownerNameMap={ownerNameMap}
              onUpdate={updateDealField}
            />
          ),
        }
      ),
      columnHelper.accessor('probability', {
        id: 'probability',
        header: 'Prob.',
        size: 100,
        enableGrouping: false,
        cell: (info) => (
          <InlineProbabilityCell
            deal={info.row.original}
            onUpdate={updateDealField}
          />
        ),
      }),
      columnHelper.accessor('close_date', {
        id: 'closeDate',
        header: 'Close Date',
        size: 140,
        enableGrouping: false,
        cell: (info) => (
          <InlineCloseDateCell
            deal={info.row.original}
            onUpdate={updateDealField}
          />
        ),
      }),
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
      columnHelper.accessor(
        (row) => {
          const stage = stageMap.get(row.stage_id);
          return stage ? STAGE_TYPE_CONFIG[stage.stage_type].label : 'Unknown';
        },
        {
          id: 'stageTypeName',
          header: 'Stage Type',
          size: 0,
          enableHiding: true,
          cell: () => null,
        }
      ),
      columnHelper.display({
        id: 'actions',
        header: '',
        size: 44,
        enableGrouping: false,
        enableSorting: false,
        enableResizing: false,
        cell: (info) => (
          <InlineActionsCell
            deal={info.row.original}
            onOpen={onDealClick}
            onDelete={handleDelete}
          />
        ),
      }),
    ],
    [stageMap, stages, ownerNameMap, assignableMembers, onDealClick, updateDealField, handleDelete],
  );

  const columnVisibility = useMemo(() => {
    const vis: Record<string, boolean> = {};
    for (const c of HIDDEN_GROUP_COLUMNS) vis[c] = false;
    if (!displayProps.amount) vis['amount'] = false;
    if (!displayProps.probability) vis['probability'] = false;
    if (!displayProps.close_date) vis['closeDate'] = false;
    if (!displayProps.owner) vis['ownerName'] = false;
    if (!displayProps.created_at) vis['createdAt'] = false;
    return vis;
  }, [displayProps]);

  const grouping: GroupingState = useMemo(() => {
    const colId = GROUP_COLUMN_MAP[groupBy];
    return colId ? [colId] : [];
  }, [groupBy]);

  const table = useReactTable({
    data: localDeals,
    columns: tableColumns,
    state: {
      grouping,
      expanded,
      columnVisibility,
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
        <Loading01Icon className="mr-2 h-4 w-4 animate-spin" />
        Loading deals...
      </div>
    );
  }

  if (deals.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <DollarCircleIcon className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No deals yet</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Create your first deal to start tracking revenue
        </p>
        {onCreateClick && (
          <Button size="sm" className="mt-4" onClick={onCreateClick}>
            <PlusSignIcon className="mr-1 h-4 w-4" />
            Create Deal
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
          {localDeals.length} {localDeals.length === 1 ? 'deal' : 'deals'}
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

          {/* Virtualized body */}
          <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((virtualRow) => {
              const row = rows[virtualRow.index] as Row<CRMDeal>;
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
                    <MemoGroupHeaderRow row={row} stageMap={stageMap} groupBy={groupBy} />
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

const MemoGroupHeaderRow = memo(function GroupHeaderRow({
  row,
  stageMap,
  groupBy,
}: {
  row: Row<CRMDeal>;
  stageMap: Map<string, CRMPipelineStage>;
  groupBy: GroupByOption;
}) {
  const subRows = row.subRows;
  const dealCount = subRows.length;
  const totalAmount = subRows.reduce((sum, r) => sum + (r.original.amount ?? 0), 0);
  const groupValue = row.groupingValue as string;

  let icon = null;
  if (groupBy === 'stage') {
    const stage = stageMap.get(row.original?.stage_id ?? '');
    if (stage) icon = <StageTypeIcon stageType={stage.stage_type} className="h-4 w-4" />;
  } else if (groupBy === 'stage_type') {
    const stageType = Object.entries(STAGE_TYPE_CONFIG).find(([, v]) => v.label === groupValue);
    if (stageType) icon = <StageTypeIcon stageType={stageType[0] as 'open' | 'won' | 'lost'} className="h-4 w-4" />;
  }

  return (
    <div
      className={TABLE_GROUP_ROW}
      onClick={() => row.toggleExpanded()}
    >
      {row.getIsExpanded() ? (
        <ArrowDown01Icon className="h-3.5 w-3.5 text-muted-foreground" />
      ) : (
        <ArrowRight01Icon className="h-3.5 w-3.5 text-muted-foreground" />
      )}
      {icon}
      <span>{groupValue}</span>
      <span className="ml-2 text-xs font-normal text-muted-foreground">
        {dealCount} {dealCount === 1 ? 'deal' : 'deals'}
        {totalAmount > 0 && ` \u00B7 $${new Intl.NumberFormat().format(totalAmount)}`}
      </span>
    </div>
  );
});

// ── Data Row ──────────────────────────────────────────────────────

const MemoDataRow = memo(function DataRow({
  row,
  columnSizingVersion,
}: {
  row: Row<CRMDeal>;
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

function InlineStageCell({
  deal,
  stages,
  stageMap,
  onUpdate,
}: {
  deal: CRMDeal;
  stages: CRMPipelineStage[];
  stageMap: Map<string, CRMPipelineStage>;
  onUpdate: (id: string, patch: Partial<CRMDeal>) => void;
}) {
  const stage = stageMap.get(deal.stage_id);
  return (
    <Select
      value={deal.stage_id}
      onValueChange={(value) => onUpdate(deal.id, { stage_id: value })}
    >
      <SelectTrigger
        className="h-6 w-full gap-1 border-none bg-transparent px-1 text-xs shadow-none hover:bg-muted"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-1.5 truncate">
          {stage && <StageTypeIcon stageType={stage.stage_type} className="h-3.5 w-3.5 shrink-0" />}
          <span className="truncate">{stage?.name ?? 'Unknown'}</span>
        </div>
      </SelectTrigger>
      <SelectContent>
        {stages.map((s) => (
          <SelectItem key={s.id} value={s.id}>
            <div className="flex items-center gap-1.5">
              <StageTypeIcon stageType={s.stage_type} className="h-3.5 w-3.5" />
              {s.name}
            </div>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function InlineAmountCell({
  deal,
  onUpdate,
}: {
  deal: CRMDeal;
  onUpdate: (id: string, patch: Partial<CRMDeal>) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(String(deal.amount ?? ''));
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) {
      setValue(String(deal.amount ?? ''));
      setTimeout(() => inputRef.current?.select(), 0);
    }
  }, [editing, deal.amount]);

  if (!editing) {
    return (
      <button
        className="w-full text-left text-xs hover:text-primary"
        onClick={(e) => { e.stopPropagation(); setEditing(true); }}
      >
        {deal.amount != null ? `${deal.currency} ${new Intl.NumberFormat().format(deal.amount)}` : '-'}
      </button>
    );
  }

  const commit = () => {
    setEditing(false);
    const num = parseFloat(value);
    if (!isNaN(num) && num !== deal.amount) {
      onUpdate(deal.id, { amount: num });
    }
  };

  return (
    <input
      ref={inputRef}
      className="w-full rounded border border-primary/40 bg-background px-1 py-0.5 text-left text-xs outline-none"
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') commit();
        if (e.key === 'Escape') setEditing(false);
        e.stopPropagation();
      }}
      onClick={(e) => e.stopPropagation()}
    />
  );
}

function InlineOwnerCell({
  deal,
  assignableMembers,
  ownerNameMap,
  onUpdate,
}: {
  deal: CRMDeal;
  assignableMembers: AssignableMember[];
  ownerNameMap: Map<string, string>;
  onUpdate: (id: string, patch: Partial<CRMDeal>) => void;
}) {
  const ownerName = deal.owner_member_id ? ownerNameMap.get(deal.owner_member_id) ?? 'Unknown' : null;

  return (
    <MemberPickerPopover
      value={deal.owner_member_id || '__none__'}
      members={assignableMembers}
      noneLabel="Unassigned"
      onChange={(value) => {
        onUpdate(deal.id, { owner_member_id: value === '__none__' ? '' : value });
      }}
      triggerClassName="flex w-full items-center gap-1.5 truncate text-xs hover:text-primary"
      contentClassName="w-[220px]"
      renderTrigger={() => {
        const selectedMember = findAssignableMember(assignableMembers, deal.owner_member_id);
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
            <UserAdd01Icon className="h-3 w-3" /> Assign
          </span>
        );
      }}
    />
  );
}

function InlineProbabilityCell({
  deal,
  onUpdate,
}: {
  deal: CRMDeal;
  onUpdate: (id: string, patch: Partial<CRMDeal>) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(String(deal.probability ?? ''));
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) {
      setValue(String(deal.probability ?? ''));
      setTimeout(() => inputRef.current?.select(), 0);
    }
  }, [editing, deal.probability]);

  if (!editing) {
    return (
      <button
        className="flex w-full items-center gap-1.5 text-xs hover:text-primary"
        onClick={(e) => { e.stopPropagation(); setEditing(true); }}
      >
        {deal.probability != null ? (
          <>
            <div className="h-1 w-8 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary/60" style={{ width: `${deal.probability}%` }} />
            </div>
            <span>{deal.probability}%</span>
          </>
        ) : '-'}
      </button>
    );
  }

  const commit = () => {
    setEditing(false);
    const num = parseInt(value, 10);
    if (!isNaN(num) && num >= 0 && num <= 100 && num !== deal.probability) {
      onUpdate(deal.id, { probability: num });
    }
  };

  return (
    <input
      ref={inputRef}
      className="w-full rounded border border-primary/40 bg-background px-1 py-0.5 text-left text-xs outline-none"
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') commit();
        if (e.key === 'Escape') setEditing(false);
        e.stopPropagation();
      }}
      onClick={(e) => e.stopPropagation()}
    />
  );
}

function InlineCloseDateCell({
  deal,
  onUpdate,
}: {
  deal: CRMDeal;
  onUpdate: (id: string, patch: Partial<CRMDeal>) => void;
}) {
  const [open, setOpen] = useState(false);
  const dateValue = deal.close_date ? parseISO(deal.close_date) : undefined;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className="flex w-full items-center gap-1 text-xs text-muted-foreground hover:text-primary"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          <Calendar03Icon className="h-3 w-3 shrink-0" />
          {dateValue ? format(dateValue, 'MMM d, yyyy') : 'Set date'}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-auto p-0"
          align="start"
          onClick={(e) => e.stopPropagation()}
        >
          <Calendar
            mode="single"
            selected={dateValue}
            onSelect={(date) => {
              if (date) {
                const iso = format(date, 'yyyy-MM-dd');
                onUpdate(deal.id, { close_date: iso });
              }
              setOpen(false);
            }}
          />
        </PopoverContent>
      )}
    </Popover>
  );
}

function InlineActionsCell({
  deal,
  onOpen,
  onDelete,
}: {
  deal: CRMDeal;
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
          <MoreVerticalIcon className="h-3.5 w-3.5" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-[140px]">
        <DropdownMenuItem onClick={() => onOpen(deal.id)}>
          <LinkSquare01Icon className="mr-2 h-3.5 w-3.5" />
          Open
        </DropdownMenuItem>
        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => onDelete(deal.id)}
        >
          <Delete01Icon className="mr-2 h-3.5 w-3.5" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
