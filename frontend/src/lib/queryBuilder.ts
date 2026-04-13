export type QueryFilterLogic = 'and' | 'or';

export type QueryFilterOperator =
  | 'is'
  | 'is_not'
  | 'contains'
  | 'not_contains'
  | 'starts_with'
  | 'ends_with'
  | 'is_empty'
  | 'is_not_empty'
  | 'on'
  | 'before'
  | 'after'
  | 'on_or_before'
  | 'on_or_after'
  | 'between';

export type QueryFilterFieldType = 'text' | 'enum' | 'member' | 'date';

export interface QueryFilterRule {
  field: string;
  operator: QueryFilterOperator;
  value?: string;
  values?: string[];
}

export interface QueryFilterGroup {
  logic: QueryFilterLogic;
  rules: QueryFilterRule[];
}

export interface QueryBuilderOption {
  value: string;
  label: string;
}

export interface QueryBuilderFieldDefinition {
  field: string;
  label: string;
  type: QueryFilterFieldType;
  placeholder?: string;
  operators?: QueryFilterOperator[];
  options?: QueryBuilderOption[];
}

export const QUERY_FILTER_OPERATOR_LABELS: Record<QueryFilterOperator, string> = {
  is: 'is',
  is_not: 'is not',
  contains: 'contains',
  not_contains: 'does not contain',
  starts_with: 'starts with',
  ends_with: 'ends with',
  is_empty: 'is empty',
  is_not_empty: 'is not empty',
  on: 'is on',
  before: 'is before',
  after: 'is after',
  on_or_before: 'is on or before',
  on_or_after: 'is on or after',
  between: 'is between',
};

const TEXT_OPERATORS: QueryFilterOperator[] = [
  'is',
  'is_not',
  'contains',
  'not_contains',
  'starts_with',
  'ends_with',
  'is_empty',
  'is_not_empty',
];

const ENUM_OPERATORS: QueryFilterOperator[] = [
  'is',
  'is_not',
  'is_empty',
  'is_not_empty',
];

const DATE_OPERATORS: QueryFilterOperator[] = [
  'on',
  'before',
  'after',
  'on_or_before',
  'on_or_after',
  'between',
  'is_empty',
  'is_not_empty',
];

export function getOperatorsForField(field: QueryBuilderFieldDefinition): QueryFilterOperator[] {
  if (field.operators?.length) {
    return field.operators;
  }

  switch (field.type) {
    case 'date':
      return DATE_OPERATORS;
    case 'enum':
    case 'member':
      return ENUM_OPERATORS;
    case 'text':
    default:
      return TEXT_OPERATORS;
  }
}

export function operatorUsesValue(operator: QueryFilterOperator): boolean {
  return operator !== 'is_empty' && operator !== 'is_not_empty';
}

export function operatorUsesRange(operator: QueryFilterOperator): boolean {
  return operator === 'between';
}

export function getDefaultOperator(field: QueryBuilderFieldDefinition): QueryFilterOperator {
  return getOperatorsForField(field)[0];
}

export function isQueryFilterRuleComplete(rule: QueryFilterRule): boolean {
  if (!rule.field || !rule.operator) {
    return false;
  }
  if (!operatorUsesValue(rule.operator)) {
    return true;
  }
  if (operatorUsesRange(rule.operator)) {
    return !!rule.values?.[0] && !!rule.values?.[1];
  }
  return !!rule.value;
}

export function normalizeQueryFilterGroup(group?: QueryFilterGroup | null): QueryFilterGroup | undefined {
  if (!group) {
    return undefined;
  }

  const rules = group.rules
    .map((rule) => {
      const normalized: QueryFilterRule = {
        field: rule.field,
        operator: rule.operator,
      };

      if (operatorUsesRange(rule.operator)) {
        normalized.values = (rule.values ?? []).map((value) => value.trim()).filter(Boolean);
      } else if (rule.value != null) {
        const value = rule.value.trim();
        if (value) {
          normalized.value = value;
        }
      }

      return normalized;
    })
    .filter(isQueryFilterRuleComplete);

  if (rules.length === 0) {
    return undefined;
  }

  return {
    logic: group.logic === 'or' ? 'or' : 'and',
    rules,
  };
}

export function serializeQueryFilterGroup(group?: QueryFilterGroup | null): string | undefined {
  const normalized = normalizeQueryFilterGroup(group);
  return normalized ? JSON.stringify(normalized) : undefined;
}

export function parseQueryFilterGroup(raw?: string): QueryFilterGroup | undefined {
  if (!raw) {
    return undefined;
  }

  try {
    const parsed = JSON.parse(raw) as QueryFilterGroup;
    return normalizeQueryFilterGroup(parsed);
  } catch {
    return undefined;
  }
}
