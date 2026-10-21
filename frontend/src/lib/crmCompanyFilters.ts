import type { VisibilityState } from '@tanstack/react-table';
import type { PMFilterDefinition } from '@/components/pm/PMFilterControls';
import { buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { customPropertyLabel } from '@/lib/crmCustomProperties';
import type { CRMCompanyFilterOptions, CRMCustomPropertySummary } from '@/lib/crmTypes';
import type { QueryFilterGroup } from '@/lib/queryBuilder';
import type { AssignableMember } from '@/lib/types';

/** Filter keys for native company fields. */
export const COMPANY_OWNER_FILTER = 'owner_member_id';
export const COMPANY_INDUSTRY_FILTER = 'industry';

const CUSTOM_PROPERTY_FIELD_PREFIX = 'custom_properties.';
const CUSTOM_PROPERTY_COLUMN_PREFIX = 'cp:';

/** Selected values per filter key, as the shared filter pills hold them. */
export type CompanyFilterValues = Record<string, string[]>;

/** Query-builder field for a custom property key. */
export function customPropertyFilterField(key: string): string {
  return `${CUSTOM_PROPERTY_FIELD_PREFIX}${key}`;
}

/** Table column id for a custom property key. */
export function customPropertyColumnId(key: string): string {
  return `${CUSTOM_PROPERTY_COLUMN_PREFIX}${key}`;
}

export function isCustomPropertyColumnId(id: string): boolean {
  return id.startsWith(CUSTOM_PROPERTY_COLUMN_PREFIX);
}

/**
 * Builds the list API filter group: one is_any_of rule per key with selected
 * values. Values of one key are alternatives; different keys must all match.
 */
export function companyFilterValuesToGroup(values: CompanyFilterValues): QueryFilterGroup | undefined {
  const rules = Object.entries(values)
    .filter(([, selected]) => selected.length > 0)
    .map(([field, selected]) => ({ field, operator: 'is_any_of' as const, values: selected }));
  return rules.length > 0 ? { logic: 'and', rules } : undefined;
}

/** Reads selected values back from a filter group; other rules are ignored. */
export function companyFilterGroupToValues(group: QueryFilterGroup | undefined): CompanyFilterValues {
  const values: CompanyFilterValues = {};
  for (const rule of group?.rules ?? []) {
    if (rule.operator === 'is_any_of' && rule.values?.length) values[rule.field] = rule.values;
  }
  return values;
}

/** Returns values with one option toggled for a key. */
export function toggleCompanyFilterValue(values: CompanyFilterValues, key: string, value: string): CompanyFilterValues {
  const selected = values[key] ?? [];
  const next = selected.includes(value) ? selected.filter((item) => item !== value) : [...selected, value];
  return { ...values, [key]: next };
}

/** Returns values without a key. */
export function removeCompanyFilter(values: CompanyFilterValues, key: string): CompanyFilterValues {
  const next = { ...values };
  delete next[key];
  return next;
}

const COLUMN_STORAGE_PREFIX = 'helpin.crm.companies.columns.v1:';

/** Column visibility the user chose for a workspace's company list. */
export function readCompanyColumnVisibility(workspaceId: string): VisibilityState {
  if (!workspaceId) return {};
  try {
    const raw = window.localStorage.getItem(COLUMN_STORAGE_PREFIX + workspaceId);
    const parsed: unknown = raw ? JSON.parse(raw) : {};
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    return Object.fromEntries(
      Object.entries(parsed as Record<string, unknown>).filter((entry): entry is [string, boolean] => typeof entry[1] === 'boolean'),
    );
  } catch {
    return {};
  }
}

export function writeCompanyColumnVisibility(workspaceId: string, visibility: VisibilityState): void {
  if (!workspaceId) return;
  try {
    window.localStorage.setItem(COLUMN_STORAGE_PREFIX + workspaceId, JSON.stringify(visibility));
  } catch {
    // Storage can be unavailable (private mode, quota); visibility then lasts for the session.
  }
}

/**
 * Applies defaults to saved visibility: custom property columns stay hidden
 * until the user shows them, so new keys never widen the table on their own.
 */
export function effectiveCompanyColumnVisibility(saved: VisibilityState, customColumnIds: string[]): VisibilityState {
  const visibility: VisibilityState = {};
  for (const id of customColumnIds) visibility[id] = false;
  return { ...visibility, ...saved };
}

// Above this many options, value pickers get a search field and wider menu.
const SEARCHABLE_OPTION_COUNT = 8;

function customPropertyOptionLabel(summary: CRMCustomPropertySummary, value: string): string {
  if (summary.value_type === 'boolean' || summary.value_type === 'mixed') {
    if (value === 'true') return 'Yes';
    if (value === 'false') return 'No';
  }
  return value;
}

/**
 * Filter definitions for the company list: Owner first, then Industry, then
 * every custom property key in use, each offering the values found in the data.
 */
export function buildCompanyFilterDefinitions(
  options: CRMCompanyFilterOptions | undefined,
  assignableMembers: AssignableMember[],
): PMFilterDefinition<string>[] {
  const definitions: PMFilterDefinition<string>[] = [
    {
      key: COMPANY_OWNER_FILTER,
      label: 'Owner',
      options: buildAssignableMemberOptions(assignableMembers).map((member) => ({ value: member.id, label: member.name })),
      searchableValues: true,
    },
    {
      key: COMPANY_INDUSTRY_FILTER,
      label: 'Industry',
      options: (options?.industries ?? []).map((industry) => ({ value: industry.value, label: industry.value })),
      searchableValues: (options?.industries.length ?? 0) > SEARCHABLE_OPTION_COUNT,
    },
  ];
  for (const summary of options?.custom_properties ?? []) {
    definitions.push({
      key: customPropertyFilterField(summary.key),
      label: customPropertyLabel(summary.key),
      options: summary.values.map((entry) => ({ value: entry.value, label: customPropertyOptionLabel(summary, entry.value) })),
      searchableValues: summary.values.length > SEARCHABLE_OPTION_COUNT,
    });
  }
  return definitions;
}
