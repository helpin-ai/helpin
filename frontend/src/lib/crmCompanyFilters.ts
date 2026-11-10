import type { VisibilityState } from '@tanstack/react-table';
import type { PMFilterDefinition } from '@/components/pm/PMFilterControls';
import {
  isPMFilterConditionComplete,
  optionModeUsesValues,
  type PMFilterCondition,
  type PMFilterConditions,
  type PMFilterOptionMode,
  type PMFilterOptionModes,
} from '@/lib/pmFilterConditions';
import { buildAssignableMemberOptions } from '@/lib/assignableMembers';
import { customPropertyLabel, customPropertyOptionLabel, isNumericCustomProperty } from '@/lib/crmCustomProperties';
import type { CRMCompanyFilterOptions, CRMCustomPropertySummary } from '@/lib/crmTypes';
import { DEFINED_FIELDS_GROUP, UNDEFINED_FIELDS_GROUP, type CustomFieldDefinition } from '@/lib/customFields';
import type { QueryFilterGroup, QueryFilterRule } from '@/lib/queryBuilder';
import type { AssignableMember } from '@/lib/types';

/** Filter keys for native company fields. */
const COMPANY_OWNER_FILTER = 'owner_member_id';
const COMPANY_INDUSTRY_FILTER = 'industry';

const CUSTOM_PROPERTY_FIELD_PREFIX = 'custom_properties.';
const CUSTOM_PROPERTY_COLUMN_PREFIX = 'cp:';

/** Selected values per filter key, as the shared filter pills hold them. */
export type CompanyFilterValues = Record<string, string[]>;

/** Number conditions per filter key, for numeric custom properties. */
export type CompanyFilterConditions = PMFilterConditions<string>;

/** Chosen operator per options filter; a missing entry means is any of. */
export type CompanyFilterModes = PMFilterOptionModes<string>;

/** All applied company filters. */
export interface CompanyFilterState {
  values: CompanyFilterValues;
  conditions: CompanyFilterConditions;
  modes: CompanyFilterModes;
}

/** A state with no filters. */
export const EMPTY_COMPANY_FILTERS: CompanyFilterState = { values: {}, conditions: {}, modes: {} };

/** Query-builder field for a custom property key. */
export function customPropertyFilterField(key: string): string {
  return `${CUSTOM_PROPERTY_FIELD_PREFIX}${key}`;
}

/** Table column id for a custom property key. */
export function customPropertyColumnId(key: string): string {
  return `${CUSTOM_PROPERTY_COLUMN_PREFIX}${key}`;
}

// Date conditions reuse the number operators; the API takes date operators.
const DATE_RULE_OPERATORS = { equals: 'on', gt: 'after', gte: 'on_or_after', lt: 'before', lte: 'on_or_before' } as const;

function conditionToRule(field: string, condition: PMFilterCondition, date: boolean): QueryFilterRule {
  if (date && condition.operator in DATE_RULE_OPERATORS) {
    return { field, operator: DATE_RULE_OPERATORS[condition.operator as keyof typeof DATE_RULE_OPERATORS], value: condition.value!.trim() };
  }
  switch (condition.operator) {
    // Equality compares numerically, so 40 matches "40" and 40.0.
    case 'equals':
      return { field, operator: 'between', values: [condition.value!.trim(), condition.value!.trim()] };
    case 'between':
      return { field, operator: 'between', values: [condition.values![0].trim(), condition.values![1].trim()] };
    case 'is_empty':
    case 'is_not_empty':
      return { field, operator: condition.operator };
    default:
      return { field, operator: condition.operator, value: condition.value!.trim() };
  }
}

function ruleToCondition(rule: QueryFilterRule): PMFilterCondition | undefined {
  switch (rule.operator) {
    case 'between': {
      const [start = '', end = ''] = rule.values ?? [];
      return start === end ? { operator: 'equals', value: start } : { operator: 'between', values: [start, end] };
    }
    case 'gt':
    case 'gte':
    case 'lt':
    case 'lte':
      return { operator: rule.operator, value: rule.value ?? '' };
    case 'is_empty':
    case 'is_not_empty':
      return { operator: rule.operator };
    default: {
      const dateOperator = Object.entries(DATE_RULE_OPERATORS).find(([, operator]) => operator === rule.operator)?.[0];
      return dateOperator ? { operator: dateOperator as PMFilterCondition['operator'], value: rule.value ?? '', date: true } : undefined;
    }
  }
}

/**
 * Builds the list API filter group: one is_any_of rule per key with selected
 * values, and one rule per complete number condition. Values of one key are
 * alternatives; different keys must all match. Incomplete conditions, such as
 * a pill still waiting for its number, are left out.
 */
export function companyFiltersToGroup(state: CompanyFilterState, dateFields: ReadonlySet<string> = new Set()): QueryFilterGroup | undefined {
  const rules: QueryFilterRule[] = [];
  const optionFields = new Set([...Object.keys(state.values), ...Object.keys(state.modes)]);
  for (const field of optionFields) {
    const mode: PMFilterOptionMode = state.modes[field] ?? 'is_any_of';
    if (!optionModeUsesValues(mode)) {
      rules.push({ field, operator: mode });
      continue;
    }
    const selected = state.values[field] ?? [];
    if (selected.length > 0) rules.push({ field, operator: mode, values: selected });
  }
  for (const [field, condition] of Object.entries(state.conditions)) {
    if (condition && isPMFilterConditionComplete(condition)) rules.push(conditionToRule(field, condition, dateFields.has(field) || Boolean(condition.date)));
  }
  return rules.length > 0 ? { logic: 'and', rules } : undefined;
}

/**
 * Reads applied filters back from a filter group; unsupported rules are
 * ignored. numberFields (number and date filters) disambiguates empty checks,
 * which option filters use too.
 */
export function companyFilterGroupToState(
  group: QueryFilterGroup | undefined,
  numberFields: ReadonlySet<string> = new Set(),
): CompanyFilterState {
  const state: CompanyFilterState = { values: {}, conditions: {}, modes: {} };
  for (const rule of group?.rules ?? []) {
    if (rule.operator === 'is_any_of' || rule.operator === 'is_none_of') {
      if (!rule.values?.length) continue;
      state.values[rule.field] = rule.values;
      if (rule.operator === 'is_none_of') state.modes[rule.field] = 'is_none_of';
      continue;
    }
    if ((rule.operator === 'is_empty' || rule.operator === 'is_not_empty') && !numberFields.has(rule.field)) {
      state.modes[rule.field] = rule.operator;
      continue;
    }
    const condition = ruleToCondition(rule);
    if (condition) state.conditions[rule.field] = condition;
  }
  return state;
}

/** Returns values with one option toggled for a key. */
export function toggleCompanyFilterValue(values: CompanyFilterValues, key: string, value: string): CompanyFilterValues {
  const selected = values[key] ?? [];
  const next = selected.includes(value) ? selected.filter((item) => item !== value) : [...selected, value];
  return { ...values, [key]: next };
}

/** Returns the state without any filter for a key. */
export function removeCompanyFilter(state: CompanyFilterState, key: string): CompanyFilterState {
  const values = { ...state.values };
  const conditions = { ...state.conditions };
  const modes = { ...state.modes };
  delete values[key];
  delete conditions[key];
  delete modes[key];
  return { values, conditions, modes };
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

/** Filter for a defined field, typed by its definition. */
function definedFieldFilter(field: CustomFieldDefinition, summary: CRMCustomPropertySummary | undefined): PMFilterDefinition<string> {
  const base = { key: customPropertyFilterField(field.key), label: field.display_label, group: DEFINED_FIELDS_GROUP };
  switch (field.field_type) {
    case 'number':
      return { ...base, options: [], kind: 'number' };
    case 'date':
      return { ...base, options: [], kind: 'date' };
    case 'boolean':
      return { ...base, options: [{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }], optionModes: true };
    case 'select':
    case 'multi_select':
      return { ...base, options: field.options, searchableValues: field.options.length > SEARCHABLE_OPTION_COUNT, optionModes: true };
    default: {
      const values = summary?.values ?? [];
      return { ...base, options: values.map((entry) => ({ value: entry.value, label: entry.value })), searchableValues: values.length > SEARCHABLE_OPTION_COUNT, optionModes: true };
    }
  }
}

/**
 * Filter definitions for the company list: Owner first, then Industry, then
 * defined fields, then other custom property keys in use, each offering the
 * values found in the data.
 */
export function buildCompanyFilterDefinitions(
  options: CRMCompanyFilterOptions | undefined,
  assignableMembers: AssignableMember[],
  fields: CustomFieldDefinition[] = [],
): PMFilterDefinition<string>[] {
  const definitions: PMFilterDefinition<string>[] = [
    {
      key: COMPANY_OWNER_FILTER,
      label: 'Owner',
      options: buildAssignableMemberOptions(assignableMembers).map((member) => ({ value: member.id, label: member.name })),
      searchableValues: true,
      optionModes: true,
    },
    {
      key: COMPANY_INDUSTRY_FILTER,
      label: 'Industry',
      options: (options?.industries ?? []).map((industry) => ({ value: industry.value, label: industry.value })),
      searchableValues: (options?.industries.length ?? 0) > SEARCHABLE_OPTION_COUNT,
      optionModes: true,
    },
  ];
  const summaries = new Map((options?.custom_properties ?? []).map((summary) => [summary.key, summary]));
  for (const field of fields) definitions.push(definedFieldFilter(field, summaries.get(field.key)));
  const group = fields.length > 0 ? UNDEFINED_FIELDS_GROUP : undefined;
  const defined = new Set(fields.map((field) => field.key));
  for (const summary of options?.custom_properties ?? []) {
    if (defined.has(summary.key)) continue;
    if (isNumericCustomProperty(summary)) {
      const skipped = summary.count - summary.numeric_count;
      definitions.push({
        key: customPropertyFilterField(summary.key),
        label: customPropertyLabel(summary.key),
        group,
        options: [],
        kind: 'number',
        note: skipped > 0 ? `${skipped} not numeric` : undefined,
      });
      continue;
    }
    definitions.push({
      key: customPropertyFilterField(summary.key),
      label: customPropertyLabel(summary.key),
      group,
      options: summary.values.map((entry) => ({ value: entry.value, label: customPropertyOptionLabel(summary, entry.value) })),
      searchableValues: summary.values.length > SEARCHABLE_OPTION_COUNT,
      optionModes: true,
    });
  }
  return definitions;
}
