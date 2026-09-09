import { EpicColorSwatch } from './EpicColorSwatch';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { PlusSignIcon } from '@/lib/icons';
import { PRIORITY_CONFIG, SEVERITY_CONFIG, TASK_TYPE_CONFIG } from '@/lib/pmConstants';
import type { Priority, Severity, TaskType, Label, EpicWithStats, SprintWithStats } from '@/lib/pmTypes';
import type { AssignableMember, TeamUserMembership } from '@/lib/types';
import { usePMBoardStore, type BoardFilters } from '@/stores/pmBoardStore';
import { isDefaultView } from '@/lib/pmDefaultViews';
import { buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { useCompanies, useContacts, useConversations, useDeals } from '@/hooks/queries';
import { OwnerAvatarFilterRow } from './OwnerAvatarFilterRow';
import { filterAssignableMembersForTeam } from '@/components/pm/task-detail/taskFilterMembers';
import { SaveViewDialog } from './SaveViewDialog';
import {
  PMFilterBar,
  PMFilterTrigger,
  type PMFilterDefinition,
  type PMFilterOption,
} from './PMFilterControls';

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

type FilterOption = PMFilterOption;
type FilterDefinition = PMFilterDefinition<FilterKey>;

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
  visibleKeys: Set<FilterKey>;
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
  excludedKeys?: string[];
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
  excludedKeys,
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
      icon: <EpicColorSwatch color={e.epic.color} />,
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

    const excluded = new Set(excludedKeys ?? []);
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
    ].filter((definition) => !excluded.has(definition.key));
  }, [assignableMembers, labels, epics, sprints, contactsRes, companiesRes, dealsRes, conversationsRes, excludedKeys]);

  const activeKeys = useMemo(() => {
    const keys = new Set<FilterKey>();
    for (const [key, values] of Object.entries(filterState)) {
      if (values && values.length > 0) {
        keys.add(key as FilterKey);
      }
    }
    return keys;
  }, [filterState]);

  const visibleKeys = useMemo(
    () => new Set(Object.keys(filterState).map((key) => key as FilterKey)),
    [filterState],
  );

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
      if (!def || def.options.length === 0) return;
      setFilterState((current) => ({ ...current, [key]: current[key] ?? [] }));
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
      const rest = { ...filterState };
      delete rest[key];
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
    visibleKeys,
    activeCount: activeKeys.size,
    handleAdd,
    handleToggle,
    handleRemove,
    handleClearAll,
  }), [workspaceId, filterState, definitions, assignableMembers, activeTeamId, userMemberships, activeKeys, visibleKeys, handleAdd, handleToggle, handleRemove, handleClearAll]);

  return <FilterContext.Provider value={value}>{children}</FilterContext.Provider>;
}

// ── Trigger button (goes in the header row) ────────────────────────

export function TaskFilterTrigger() {
  const { definitions, filterState, visibleKeys, activeCount, handleAdd, handleToggle } = useFilterContext();
  return (
    <PMFilterTrigger
      definitions={definitions}
      values={filterState}
      visibleKeys={visibleKeys}
      activeCount={activeCount}
      onAdd={handleAdd}
      onToggle={handleToggle}
    />
  );
}

// ── Filter bar (renders on its own row below the header) ───────────

function suggestViewName(
  filterState: FilterState,
  definitions: FilterDefinition[],
): string {
  const parts: string[] = [];
  for (const def of definitions) {
    const values = filterState[def.key];
    if (!values || values.length === 0) continue;
    if (values.length === 1) {
      const label = def.options.find((o) => o.value === values[0])?.label ?? values[0];
      parts.push(`${def.label}: ${label}`);
    } else {
      parts.push(`${def.label} (${values.length})`);
    }
  }
  return parts.slice(0, 2).join(' · ');
}

export function TaskFilterBar() {
  const { workspaceId, filterState, definitions, visibleKeys, activeCount, handleToggle, handleRemove, handleClearAll } = useFilterContext();
  const { activeViewId, saveCurrentAsView } = usePMBoardStore();
  const [saveOpen, setSaveOpen] = useState(false);

  const canSaveAsView = !activeViewId || isDefaultView(activeViewId);

  return (
    <PMFilterBar
      definitions={definitions}
      values={filterState}
      visibleKeys={visibleKeys}
      onToggle={handleToggle}
      onRemove={handleRemove}
      onClearAll={handleClearAll}
      trailing={canSaveAsView && activeCount > 0 ? (
        <>
          <button
            type="button"
            onClick={() => setSaveOpen(true)}
            className="inline-flex items-center gap-1 rounded-md border border-dashed border-border/80 px-2 py-1 text-xs text-muted-foreground hover:border-foreground/40 hover:text-foreground transition-colors"
          >
            <PlusSignIcon className="h-3 w-3" />
            Save as view
          </button>
          <SaveViewDialog
            open={saveOpen}
            onOpenChange={setSaveOpen}
            title="Save as new view"
            initialName={suggestViewName(filterState, definitions)}
            onSave={(name, isShared) => {
              saveCurrentAsView(workspaceId, name, isShared);
            }}
          />
        </>
      ) : null}
    />
  );
}

export function TaskOwnerAvatarFilterRow() {
  const { workspaceId, assignableMembers, activeTeamId, userMemberships, filterState, handleToggle } = useFilterContext();
  const members = useMemo(
    () => filterAssignableMembersForTeam(assignableMembers, activeTeamId, userMemberships),
    [assignableMembers, activeTeamId, userMemberships],
  );

  return (
    <OwnerAvatarFilterRow
      workspaceId={workspaceId}
      members={members}
      selectedIds={filterState.owner_member_ids ?? []}
      onToggle={(memberId) => handleToggle('owner_member_ids', memberId)}
    />
  );
}
