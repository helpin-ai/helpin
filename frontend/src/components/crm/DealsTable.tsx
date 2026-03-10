import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  useReactTable,
  getCoreRowModel,
  getGroupedRowModel,
  getExpandedRowModel,
  flexRender,
  createColumnHelper,
  type GroupingState,
  type ExpandedState,
  type Row,
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { CalendarDays, Check, ChevronDown, ChevronRight, DollarSign, EllipsisVertical, ExternalLink, Loader2, Plus, Trash2, UserPlus } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Calendar } from '@/components/ui/calendar';
import { format, parseISO } from 'date-fns';
import { crmDealService } from '@/lib/services/crmService';
import { StageTypeIcon, STAGE_TYPE_CONFIG } from '@/lib/crmConstants';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Button } from '@/components/ui/button';
import { useDealDisplayStore } from '@/stores/dealDisplayStore';
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
      if (patch.owner_member_id !== undefined) {
        // No owner_name on deal, handled via ownerNameMap
      }
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
      else onDealUpdated?.({ ...localDeals.find((d) => d.id === dealId)!, ...optimisticPatch } as CRMDeal);
    },
    [workspaceId, stageMap, onDealUpdated, localDeals],
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
            className="max-w-full truncate text-left text-sm hover:text-primary hover:underline"
            onClick={(e) => {
              e.stopPropagation();
              onDealClick(info.row.original.id);
            }}
          >
            {info.getValue()}
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
        size: 120,
        enableGrouping: false,
        cell: (info) => {
          const val = info.getValue();
          if (!val) return null;
          return (
            <span className="text-xs text-muted-foreground whitespace-nowrap">
              {format(parseISO(val), 'MMM d, yyyy')}
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
    },
    onExpandedChange: setExpanded,
    autoResetExpanded: false,
    getRowId: (row) => row.id,
    getExpandedRowModel: getExpandedRowModel(),
    getGroupedRowModel: getGroupedRowModel(),
    getCoreRowModel: getCoreRowModel(),
  });

  const { rows } = table.getRowModel();

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) => {
      const row = rows[index];
      return row?.getIsGrouped() ? 40 : 36;
    },
    overscan: 20,
  });

  if (isLoading) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        Loading deals...
      </div>
    );
  }

  if (deals.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted">
          <DollarSign className="h-8 w-8 text-muted-foreground/50" />
        </div>
        <h3 className="mt-4 text-base font-medium">No deals yet</h3>
        <p className="mt-1 max-w-sm text-sm text-muted-foreground">
          Create your first deal to start tracking revenue
        </p>
        {onCreateClick && (
          <Button size="sm" className="mt-4" onClick={onCreateClick}>
            <Plus className="mr-1 h-4 w-4" />
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
      <div className="min-h-0 flex-1 overflow-auto rounded-md border border-border/70">
        <div className="min-w-fit">
          {/* Header */}
          <div className="sticky top-0 z-10 border-b border-border/70 bg-muted/50">
            {table.getHeaderGroups().map((headerGroup) => (
              <div key={headerGroup.id} className="flex items-center">
                {headerGroup.headers.map((header) => {
                  if (header.column.getIsGrouped()) return null;
                  const size = header.getSize();
                  if (size === 0) return null;
                  return (
                    <div
                      key={header.id}
                      className={`px-2 py-1.5 text-xs font-medium text-muted-foreground ${size !== 999 ? 'text-center' : ''}`}
                      style={{
                        width: size === 999 ? undefined : size,
                        flex: size === 999 ? '1 1 0%' : undefined,
                        minWidth: size === 999 ? 300 : undefined,
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

          {/* Virtualized body */}
          <div
            ref={parentRef}
            className="overflow-auto"
            style={{ height: 'calc(100% - 30px)' }}
          >
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
                      <GroupHeaderRow row={row} stageMap={stageMap} groupBy={groupBy} />
                    ) : (
                      <DataRow row={row} />
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ── Group Header Row ──────────────────────────────────────────────

function GroupHeaderRow({
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
      className="flex h-10 cursor-pointer items-center gap-2 border-b border-border/50 bg-muted/40 px-3 text-sm font-semibold hover:bg-muted/60"
      onClick={() => row.toggleExpanded()}
    >
      {row.getIsExpanded() ? (
        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
      ) : (
        <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
      )}
      {icon}
      <span>{groupValue}</span>
      <span className="ml-2 text-xs font-normal text-muted-foreground">
        {dealCount} {dealCount === 1 ? 'deal' : 'deals'}
        {totalAmount > 0 && ` \u00B7 $${new Intl.NumberFormat().format(totalAmount)}`}
      </span>
    </div>
  );
}

// ── Data Row ──────────────────────────────────────────────────────

function DataRow({ row }: { row: Row<CRMDeal> }) {
  return (
    <div
      className="flex h-9 items-center border-b border-border/30 transition-colors hover:bg-muted/30"
    >
      {row.getVisibleCells().map((cell) => {
        if (cell.column.getIsGrouped()) return null;
        const size = cell.column.getSize();
        if (size === 0) return null;
        return (
          <div
            key={cell.id}
            className={`flex items-center px-2 ${size !== 999 ? 'justify-center' : ''}`}
            style={{
              width: size === 999 ? undefined : size,
              flex: size === 999 ? '1 1 0%' : undefined,
              minWidth: size === 999 ? 300 : undefined,
            }}
          >
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </div>
        );
      })}
    </div>
  );
}

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
        className="w-full text-center text-xs hover:text-primary"
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
      className="w-full rounded border border-primary/40 bg-background px-1 py-0.5 text-center text-xs outline-none"
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
  const [open, setOpen] = useState(false);
  const ownerName = deal.owner_member_id ? ownerNameMap.get(deal.owner_member_id) ?? 'Unknown' : null;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className="flex w-full items-center gap-1.5 truncate text-xs hover:text-primary"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          {ownerName ? (
            <>
              <UserAvatar name={ownerName} className="h-5 w-5 shrink-0" />
              <span className="truncate">{ownerName}</span>
            </>
          ) : (
            <span className="flex items-center gap-1 text-muted-foreground">
              <UserPlus className="h-3 w-3" /> Assign
            </span>
          )}
        </button>
      </PopoverTrigger>
      {open && (
        <PopoverContent
          className="w-[220px] p-0"
          align="start"
          side="bottom"
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <Command>
            <CommandInput placeholder="Search members..." className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No members found</CommandEmpty>
              <CommandGroup>
                {assignableMembers.map((m) => {
                  const name = ownerNameMap.get(m.id) ?? m.display_name ?? m.email;
                  const isSelected = deal.owner_member_id === m.id;
                  return (
                    <CommandItem
                      key={m.id}
                      value={name}
                      onSelect={() => {
                        onUpdate(deal.id, { owner_member_id: isSelected ? '' : m.id });
                        setOpen(false);
                      }}
                      className="flex items-center gap-2 text-xs"
                    >
                      <UserAvatar name={name} className="h-5 w-5" />
                      <span className="truncate">{name}</span>
                      {isSelected && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      )}
    </Popover>
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
        className="flex w-full items-center justify-center gap-1.5 text-xs hover:text-primary"
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
      className="w-full rounded border border-primary/40 bg-background px-1 py-0.5 text-center text-xs outline-none"
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
          className="flex w-full items-center justify-center gap-1 text-xs text-muted-foreground hover:text-primary"
          onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        >
          <CalendarDays className="h-3 w-3 shrink-0" />
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
          className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity hover:bg-muted group-hover:opacity-100 [div:hover>&]:opacity-100"
          onClick={(e) => e.stopPropagation()}
        >
          <EllipsisVertical className="h-3.5 w-3.5" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-[140px]">
        <DropdownMenuItem onClick={() => onOpen(deal.id)}>
          <ExternalLink className="mr-2 h-3.5 w-3.5" />
          Open
        </DropdownMenuItem>
        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => onDelete(deal.id)}
        >
          <Trash2 className="mr-2 h-3.5 w-3.5" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
