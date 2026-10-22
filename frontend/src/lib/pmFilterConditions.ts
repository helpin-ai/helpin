// Number-condition filters for the shared PM filter controls.

/** Operators a number filter offers, in menu order. */
export const PM_FILTER_NUMBER_OPERATORS = ['equals', 'gt', 'gte', 'lt', 'lte', 'between', 'is_not_empty', 'is_empty'] as const;
export type PMFilterNumberOperator = (typeof PM_FILTER_NUMBER_OPERATORS)[number];

/** A number filter's operator and operand(s); between uses values. */
export interface PMFilterCondition {
  operator: PMFilterNumberOperator;
  value?: string;
  values?: [string, string];
}

export type PMFilterConditions<K extends string> = Partial<Record<K, PMFilterCondition>>;

/** Whether a number condition has everything its operator needs. */
export function isPMFilterConditionComplete(condition: PMFilterCondition | undefined): boolean {
  if (!condition) return false;
  if (condition.operator === 'is_empty' || condition.operator === 'is_not_empty') return true;
  if (condition.operator === 'between') return Boolean(condition.values?.[0]?.trim() && condition.values?.[1]?.trim());
  return Boolean(condition.value?.trim());
}

/** Operators an option filter offers, in menu order. */
export const PM_FILTER_OPTION_MODES = ['is_any_of', 'is_none_of', 'is_not_empty', 'is_empty'] as const;
export type PMFilterOptionMode = (typeof PM_FILTER_OPTION_MODES)[number];

export type PMFilterOptionModes<K extends string> = Partial<Record<K, PMFilterOptionMode>>;

/** Whether an option mode compares against chosen values. */
export function optionModeUsesValues(mode: PMFilterOptionMode): boolean {
  return mode === 'is_any_of' || mode === 'is_none_of';
}
