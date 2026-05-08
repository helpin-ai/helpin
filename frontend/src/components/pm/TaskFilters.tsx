import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Tick01Icon, FilterHorizontalIcon, Cancel01Icon, ArrowLeft02Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { PRIORITY_CONFIG, SEVERITY_CONFIG, TASK_TYPE_CONFIG } from '@/lib/pmConstants';
import type { Priority, Severity, TaskType, Label, EpicWithStats, SprintWithStats } from '@/lib/pmTypes';
import type { AssignableMember, TeamUserMembership } from '@/lib/types';
import type { BoardFilters } from '@/stores/pmBoardStore';
import { buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { useCompanies, useContacts, useConversations, useDeals } from '@/hooks/queries';
import { useWorkspaceMemberPresenceMap } from '@/hooks/queries';
import { UserAvatar } from './UserAvatar';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { filterAssignableMembersForTeam } from '@/components/pm/task-detail/taskFilterMembers';

// ── Types ──────────────────────────────────────────────────────────

type FilterKey =
  | 'priority'
  | 'severity'
  | 'task_type'
  | 'owner_member_ids'
  | 'requester_member_id'
  | 'label_id'
  | 'epic_id'
  | 'sprint_id'
  | 'contact_id'
  | 'company_id'
  | 'deal_id'
  | 'support_conversation_id'
  | 'blocked'
  | 'blocking'
  | 'archived';

type FilterState = Partial<Record<FilterKey, string[]>>;

interface FilterOption {
  value: string;
  label: string;
  icon?: React.ReactNode;
}

interface FilterDefinition {
  key: FilterKey;
  label: string;
  options: FilterOption[];
  singleSelect?: boolean;
  searchableValues?: boolean;
}

// ── Helpers ────────────────────────────────────────────────────────

function filterStateToQueryParams(state: FilterState): BoardFilters {
  const params: BoardFilters = {};
  for (const [key, values] of Object.entries(state)) {
    if (values && values.length > 0) {
      params[key] = values.join(',');
    }
  }
  return params;
}

function normalizeHexColor(color?: string | null): string | undefined {
  if (!color) return undefined;
  return color.startsWith('#') ? color : `#${color}`;
}

// ── Context for shared state between trigger + bar ─────────────────

interface FilterContextValue {
  workspaceId: string;
  filterState: FilterState;
  definitions: FilterDefinition[];
  assignableMembers: AssignableMember[];
  activeTeamId?: string | null;
  userMemberships: TeamUserMembership[];
  activeKeys: Set<FilterKey>;
  activeCount: number;
  handleAdd: (key: FilterKey) => void;
  handleToggle: (key: FilterKey, value: string) => void;
  handleRemove: (key: FilterKey) => void;
  handleClearAll: () => void;
}

const FilterContext = createContext<FilterContextValue | null>(null);

function useFilterContext() {
  const ctx = useContext(FilterContext);
  if (!ctx) throw new Error('useFilterContext must be used within TaskFilterProvider');
  return ctx;
}

// ── Sub-components ─────────────────────────────────────────────────

function FilterValueSelect({
  definition,
  selected,
  onToggle,
  onEmptyClose,
}: {
  definition: FilterDefinition;
  selected: string[];
  onToggle: (value: string) => void;
  onEmptyClose: () => void;
}) {
  const [open, setOpen] = useState(false);
  const selectedLabels = selected.map((value) =>
    definition.options.find((opt) => opt.value === value)?.label ?? value,
  );

  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (!nextOpen && selected.length === 0) {
          onEmptyClose();
        }
      }}
    >
      <PopoverTrigger asChild>
        <button className="inline-flex items-center gap-1 rounded border border-border bg-background px-1.5 py-0.5 text-xs hover:bg-accent transition-colors">
          {selectedLabels.length === 0
            ? 'Choose value'
            : selectedLabels.length === 1
            ? selectedLabels[0]
            : `${selectedLabels.length} selected`}
        </button>
      </PopoverTrigger>
      <PopoverContent className={`${definition.searchableValues ? 'w-80' : 'w-52'} p-0`} align="start">
        <Command>
          {definition.searchableValues ? (
            <CommandInput placeholder={`Search ${definition.label.toLowerCase()}...`} />
          ) : null}
          <CommandList>
            <CommandEmpty>No results.</CommandEmpty>
            <CommandGroup>
              {definition.options.map((opt) => {
                const isSelected = selected.includes(opt.value);
                return (
                  <CommandItem
                    key={opt.value}
                    value={opt.label}
                    onSelect={() => onToggle(opt.value)}
                  >
                    <div className={`mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}>
                      {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                    </div>
                    {opt.icon ? <span className="mr-1.5 shrink-0">{opt.icon}</span> : null}
                    <span className="min-w-0 flex-1 truncate">{opt.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function FilterPill({
  definition,
  selected,
  onToggle,
  onRemove,
  onEmptyClose,
}: {
  definition: FilterDefinition;
  selected: string[];
  onToggle: (value: string) => void;
  onRemove: () => void;
  onEmptyClose: () => void;
}) {
  return (
    <div className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
      <span className="font-medium text-muted-foreground">{definition.label}</span>
      <span className="text-muted-foreground/60">is</span>
      <FilterValueSelect definition={definition} selected={selected} onToggle={onToggle} onEmptyClose={onEmptyClose} />
      <button
        onClick={onRemove}
        className="ml-0.5 rounded p-0.5 text-muted-foreground/60 hover:bg-accent hover:text-foreground transition-colors"
      >
        <Cancel01Icon className="h-3 w-3" />
      </button>
    </div>
  );
}

// ── Provider ───────────────────────────────────────────────────────

interface TaskFilterProviderProps {
  workspaceId: string;
  assignableMembers: AssignableMember[];
  activeTeamId?: string | null;
  userMemberships?: TeamUserMembership[];
  labels: Label[];
  epics: EpicWithStats[];
  sprints: SprintWithStats[];
  onChange: (filters: BoardFilters) => void;
  externalFilters?: BoardFilters;
  children: React.ReactNode;
}

export function TaskFilterProvider({
  workspaceId,
  assignableMembers,
  activeTeamId,
  userMemberships = [],
  labels,
  epics,
  sprints,
  onChange,
  externalFilters,
  children,
}: TaskFilterProviderProps) {
  const [filterState, setFilterState] = useState<FilterState>({});
  const internalChangeRef = useRef(false);
  const { data: contactsRes } = useContacts(workspaceId, { page: 1, per_page: 100 });
  const { data: companiesRes } = useCompanies(workspaceId, { page: 1, per_page: 100 });
  const { data: dealsRes } = useDeals(workspaceId, { page: 1, per_page: 100 });
  const { data: conversationsRes } = useConversations(workspaceId);

  // Hydrate internal filter state from external filters (e.g. when a view is applied).
  // Skip when the change originated from user interaction with filter pills.
  useEffect(() => {
    if (!externalFilters) return;
    if (internalChangeRef.current) {
      internalChangeRef.current = false;
      return;
    }
    const next: FilterState = {};
    for (const [key, value] of Object.entries(externalFilters)) {
      if (value) next[key as FilterKey] = value.split(',');
    }
    setFilterState(next);
  }, [externalFilters]);

  const definitions = useMemo<FilterDefinition[]>(() => {
    const priorityOptions: FilterOption[] = (
      ['urgent', 'high', 'medium', 'low', 'none'] as Priority[]
    ).map((p) => {
      const cfg = PRIORITY_CONFIG[p];
      const Icon = cfg.icon;
      return { value: p, label: cfg.label, icon: <Icon className={`h-3.5 w-3.5 ${cfg.color}`} /> };
    });

    const severityOptions: FilterOption[] = (
      ['critical', 'major', 'minor', 'none'] as Severity[]
    ).map((s) => {
      const cfg = SEVERITY_CONFIG[s];
      const Icon = cfg.icon;
      return { value: s, label: cfg.label, icon: <Icon className={`h-3.5 w-3.5 ${cfg.color}`} /> };
    });

    const typeOptions: FilterOption[] = (['feature', 'bug', 'chore'] as TaskType[]).map((t) => {
      const cfg = TASK_TYPE_CONFIG[t];
      const Icon = cfg.icon;
      return { value: t, label: cfg.label, icon: <Icon className={`h-3.5 w-3.5 ${cfg.color}`} /> };
    });

    const memberOptions: FilterOption[] = buildAssignableMemberOptions(assignableMembers).map((m) => ({
      value: m.id,
      label: m.name,
    }));

    const labelOptions: FilterOption[] = labels.map((l) => ({
      value: l.id,
      label: l.name,
      icon: normalizeHexColor(l.color) ? (
        <span className="h-3 w-3 rounded-full shrink-0" style={{ backgroundColor: normalizeHexColor(l.color) }} />
      ) : undefined,
    }));

    const epicOptions: FilterOption[] = epics.map((e) => ({
      value: e.epic.id,
      label: e.epic.name,
    }));

    const sprintOptions: FilterOption[] = sprints.map((i) => ({
      value: i.sprint.id,
      label: i.sprint.name,
    }));

    const contactOptions: FilterOption[] = (contactsRes?.data ?? []).map((contact) => ({
      value: contact.id,
      label: `${contact.first_name}${contact.last_name ? ` ${contact.last_name}` : ''}${contact.email ? ` (${contact.email})` : ''}`,
    }));

    const companyOptions: FilterOption[] = (companiesRes?.data ?? []).map((company) => ({
      value: company.id,
      label: company.name,
    }));

    const dealOptions: FilterOption[] = (dealsRes?.data ?? []).map((deal) => ({
      value: deal.id,
      label: deal.name,
    }));

    const supportConversationOptions: FilterOption[] = (conversationsRes?.data ?? []).map((conversation) => ({
      value: conversation.id,
      label: `#${conversation.display_id} ${conversation.subject || conversation.customer_name || conversation.customer_email || 'Conversation'}`,
    }));

    const blockedOptions: FilterOption[] = [
      { value: 'true', label: 'Blocked' },
      { value: 'false', label: 'Not blocked' },
    ];
    const blockingOptions: FilterOption[] = [
      { value: 'true', label: 'Blocking others' },
      { value: 'false', label: 'Not blocking others' },
    ];
    const archivedOptions: FilterOption[] = [
      { value: 'true', label: 'Archived' },
      { value: 'false', label: 'Not archived' },
    ];

    return [
      { key: 'priority' as FilterKey, label: 'Priority', options: priorityOptions },
      { key: 'severity' as FilterKey, label: 'Severity', options: severityOptions },
      { key: 'task_type' as FilterKey, label: 'Type', options: typeOptions },
      { key: 'owner_member_ids' as FilterKey, label: 'Owner', options: memberOptions, searchableValues: true },
      { key: 'requester_member_id' as FilterKey, label: 'Requester', options: memberOptions, searchableValues: true },
      { key: 'label_id' as FilterKey, label: 'Label', options: labelOptions, searchableValues: true },
      { key: 'epic_id' as FilterKey, label: 'Epic', options: epicOptions, searchableValues: true },
      { key: 'sprint_id' as FilterKey, label: 'Sprint', options: sprintOptions, searchableValues: true },
      { key: 'contact_id' as FilterKey, label: 'Contact', options: contactOptions, searchableValues: true },
      { key: 'company_id' as FilterKey, label: 'Company', options: companyOptions, searchableValues: true },
      { key: 'deal_id' as FilterKey, label: 'Deal', options: dealOptions, searchableValues: true },
      { key: 'support_conversation_id' as FilterKey, label: 'Support', options: supportConversationOptions, searchableValues: true },
      { key: 'blocked' as FilterKey, label: 'Blocked', options: blockedOptions },
      { key: 'blocking' as FilterKey, label: 'Blocking', options: blockingOptions },
      { key: 'archived' as FilterKey, label: 'Archived', options: archivedOptions, singleSelect: true },
    ];
  }, [assignableMembers, labels, epics, sprints, contactsRes, companiesRes, dealsRes, conversationsRes]);

  const activeKeys = useMemo(() => {
    const keys = new Set<FilterKey>();
    for (const key of Object.keys(filterState)) {
      if (filterState[key as FilterKey]) {
        keys.add(key as FilterKey);
      }
    }
    return keys;
  }, [filterState]);

  const emitChange = useCallback(
    (next: FilterState) => {
      internalChangeRef.current = true;
      onChange(filterStateToQueryParams(next));
    },
    [onChange]
  );

  const handleAdd = useCallback(
    (key: FilterKey) => {
      const def = definitions.find((d) => d.key === key);
      if (!def) return;
    },
    [definitions]
  );

  const handleToggle = useCallback(
    (key: FilterKey, value: string) => {
      const def = definitions.find((d) => d.key === key);
      const current = filterState[key] ?? [];
      let next: string[];
      if (def?.singleSelect) {
        next = current.includes(value) ? [] : [value];
      } else {
        next = current.includes(value)
          ? current.filter((v) => v !== value)
          : [...current, value];
      }
      if (next.length === 0) {
        const updated = { ...filterState, [key]: [] };
        setFilterState(updated);
        emitChange(updated);
      } else {
        const updated = { ...filterState, [key]: next };
        setFilterState(updated);
        emitChange(updated);
      }
    },
    [filterState, definitions, emitChange]
  );

  const handleRemove = useCallback(
    (key: FilterKey) => {
      const { [key]: _, ...rest } = filterState;
      setFilterState(rest);
      emitChange(rest);
    },
    [filterState, emitChange]
  );

  const handleClearAll = useCallback(() => {
    setFilterState({});
    emitChange({});
  }, [emitChange]);

  const value = useMemo<FilterContextValue>(() => ({
    workspaceId,
    filterState,
    definitions,
    assignableMembers,
    activeTeamId,
    userMemberships,
    activeKeys,
    activeCount: activeKeys.size,
    handleAdd,
    handleToggle,
    handleRemove,
    handleClearAll,
  }), [workspaceId, filterState, definitions, assignableMembers, activeTeamId, userMemberships, activeKeys, handleAdd, handleToggle, handleRemove, handleClearAll]);

  return <FilterContext.Provider value={value}>{children}</FilterContext.Provider>;
}

// ── Trigger button (goes in the header row) ────────────────────────

export function TaskFilterTrigger() {
  const { definitions, filterState, activeKeys, activeCount, handleAdd, handleToggle } = useFilterContext();
  const [open, setOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<FilterKey | null>(null);
  const available = definitions.filter((d) => !activeKeys.has(d.key) && d.options.length > 0);
  const selectedDefinition = selectedKey
    ? definitions.find((definition) => definition.key === selectedKey)
    : undefined;
  const canChooseFilter = available.length > 0 || Boolean(selectedDefinition);

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen);
    if (!nextOpen) {
      setSelectedKey(null);
    }
  };

  return (
    <>
      {canChooseFilter ? (
        <Popover open={open} onOpenChange={handleOpenChange}>
          <PopoverTrigger asChild>
            <Button variant="ghost" size="sm" className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1">
                <FilterHorizontalIcon className="h-3.5 w-3.5" />
                Filters
              </span>
              <Badge
                variant="secondary"
                className={`rounded-full px-1.5 py-0 text-[10px] transition-opacity ${activeCount > 0 ? 'opacity-100' : 'opacity-0'}`}
              >
                {activeCount || 0}
              </Badge>
            </Button>
          </PopoverTrigger>
          <PopoverContent
            className={`${selectedDefinition ? (selectedDefinition.searchableValues ? 'w-80' : 'w-52') : 'w-48'} p-0`}
            align="start"
          >
            {selectedDefinition ? (
              <Command>
                <div className="flex items-center gap-1 border-b border-border/70 px-1.5 py-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6"
                    aria-label="Back to filter fields"
                    onClick={() => setSelectedKey(null)}
                  >
                    <ArrowLeft02Icon className="h-3.5 w-3.5" />
                  </Button>
                  <span className="truncate text-xs font-medium">{selectedDefinition.label}</span>
                </div>
                {selectedDefinition.searchableValues ? (
                  <CommandInput placeholder={`Search ${selectedDefinition.label.toLowerCase()}...`} />
                ) : null}
                <CommandList>
                  <CommandEmpty>No results.</CommandEmpty>
                  <CommandGroup>
                    {selectedDefinition.options.map((option) => {
                      const isSelected = filterState[selectedDefinition.key]?.includes(option.value) ?? false;
                      return (
                        <CommandItem
                          key={option.value}
                          value={option.label}
                          onSelect={() => {
                            handleToggle(selectedDefinition.key, option.value);
                            if (selectedDefinition.singleSelect) {
                              setOpen(false);
                              setSelectedKey(null);
                            }
                          }}
                        >
                          <div className={`mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}>
                            {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                          </div>
                          {option.icon ? <span className="mr-1.5 shrink-0">{option.icon}</span> : null}
                          <span className="min-w-0 flex-1 truncate">{option.label}</span>
                        </CommandItem>
                      );
                    })}
                  </CommandGroup>
                </CommandList>
              </Command>
            ) : (
              <Command>
                <CommandInput placeholder="Filter by..." />
                <CommandList>
                  <CommandEmpty>No filters.</CommandEmpty>
                  <CommandGroup>
                    {available.map((def) => (
                      <CommandItem
                        key={def.key}
                        value={def.label}
                        onSelect={() => {
                          handleAdd(def.key);
                          setSelectedKey(def.key);
                        }}
                      >
                        {def.label}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                </CommandList>
              </Command>
            )}
          </PopoverContent>
        </Popover>
      ) : (
        <Button variant="ghost" size="sm" className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground" disabled>
          <span className="inline-flex items-center gap-1">
            <FilterHorizontalIcon className="h-3.5 w-3.5" />
            Filters
          </span>
          <Badge variant="secondary" className="ml-0.5 rounded-full px-1.5 py-0 text-[10px]">
            {activeCount}
          </Badge>
        </Button>
      )}
    </>
  );
}

// ── Filter bar (renders on its own row below the header) ───────────

export function TaskFilterBar() {
  const { filterState, definitions, activeKeys, activeCount, handleToggle, handleRemove, handleClearAll } = useFilterContext();

  if (activeCount === 0) return null;

  return (
    <div className="ui-divider-bottom-fade flex flex-wrap items-center gap-1.5 px-3 py-1.5">
      {definitions
        .filter((def) => activeKeys.has(def.key))
        .map((def) => (
          <FilterPill
            key={def.key}
            definition={def}
            selected={filterState[def.key] ?? []}
            onToggle={(value) => handleToggle(def.key, value)}
            onRemove={() => handleRemove(def.key)}
            onEmptyClose={() => {
              if ((filterState[def.key] ?? []).length === 0) {
                handleRemove(def.key);
              }
            }}
          />
        ))}
      <Button
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-[10px] text-muted-foreground"
        onClick={handleClearAll}
      >
        Clear all
      </Button>
    </div>
  );
}

export function TaskOwnerAvatarFilterRow() {
  const { workspaceId, assignableMembers, activeTeamId, userMemberships, filterState, handleToggle } = useFilterContext();
  const { data: memberPresenceByUserId } = useWorkspaceMemberPresenceMap(workspaceId);
  const ownerFilters = filterState.owner_member_ids ?? [];
  const members = useMemo(
    () => filterAssignableMembersForTeam(assignableMembers, activeTeamId, userMemberships),
    [assignableMembers, activeTeamId, userMemberships],
  );

  if (members.length === 0) return null;

  return (
    <div className="ml-3 flex min-w-0 items-center -space-x-1">
      {members.map((member) => {
        const isSelected = ownerFilters.includes(member.id);
        const label = member.display_name?.trim() || member.email;
        const presenceStatus = member.user_id ? (memberPresenceByUserId?.get(member.user_id)?.status ?? null) : null;
        return (
          <QuickTooltip key={member.id} label={label}>
            <button
              type="button"
              onClick={() => handleToggle('owner_member_ids', member.id)}
              className={`relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full ring-offset-1 ring-offset-background transition-all hover:z-10 ${
                isSelected
                  ? 'z-10 ring-[1.5px] ring-ring'
                  : 'ring-0 opacity-70 hover:opacity-100'
              }`}
              aria-pressed={isSelected}
              aria-label={`Filter by owner ${label}`}
            >
              <UserAvatar
                name={label}
                avatarUrl={member.avatar_url}
                avatarStyle={member.avatar_style}
                avatarSeed={member.avatar_seed}
                avatarBackgroundMode={member.avatar_background_mode}
                avatarBackgroundColor={member.avatar_background_color}
                presenceStatus={presenceStatus}
                className="h-6 w-6"
                fallbackClassName="text-[8px]"
              />
            </button>
          </QuickTooltip>
        );
      })}
    </div>
  );
}
