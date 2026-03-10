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
import { Building2, Check, ChevronDown, ChevronRight, EllipsisVertical, ExternalLink, Loader2, Plus, Trash2, UserPlus } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Button } from '@/components/ui/button';
import { format, parseISO } from 'date-fns';
import { crmCompanyService } from '@/lib/services/crmService';
import { UserAvatar } from '@/components/pm/UserAvatar';
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
              onRowClick(info.row.original.id);
            }}
          >
            {info.getValue()}
          </button>
        ),
      }),
      columnHelper.accessor('domain', {
        id: 'domain',
        header: 'Domain',
        size: 180,
        enableGrouping: false,
        cell: (info) => (
          <span className="truncate text-xs text-muted-foreground">{info.getValue() ?? '-'}</span>
        ),
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
      columnHelper.display({
        id: 'actions',
        header: '',
        size: 44,
        enableGrouping: false,
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
                      <GroupHeaderRow row={row} />
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

function GroupHeaderRow({ row }: { row: Row<CRMCompany> }) {
  const subRows = row.subRows;
  const count = subRows.length;
  const groupValue = row.groupingValue as string;

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
      <span>{groupValue}</span>
      <span className="ml-2 text-xs font-normal text-muted-foreground">
        {count} {count === 1 ? 'company' : 'companies'}
      </span>
    </div>
  );
}

// ── Data Row ──────────────────────────────────────────────────────

function DataRow({ row }: { row: Row<CRMCompany> }) {
  return (
    <div className="flex h-9 items-center border-b border-border/30 transition-colors hover:bg-muted/30">
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
  const [open, setOpen] = useState(false);
  const ownerName = company.owner_member_id ? ownerNameMap.get(company.owner_member_id) ?? 'Unknown' : null;

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
                  const isSelected = company.owner_member_id === m.id;
                  return (
                    <CommandItem
                      key={m.id}
                      value={name}
                      onSelect={() => {
                        onUpdate(company.id, { owner_member_id: isSelected ? '' : m.id });
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
          className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity hover:bg-muted group-hover:opacity-100 [div:hover>&]:opacity-100"
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
