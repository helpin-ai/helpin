import {
  PMFilterBar,
  PMFilterTrigger,
  type PMFilterCondition,
  type PMFilterDefinition,
  type PMFilterOptionMode,
} from '@/components/pm/PMFilterControls';
import type { CompanyFilterConditions, CompanyFilterModes, CompanyFilterValues } from '@/lib/crmCompanyFilters';

interface CompaniesFilterControlsProps {
  definitions: PMFilterDefinition<string>[];
  values: CompanyFilterValues;
  visibleKeys: Set<string>;
  onAdd: (key: string) => void;
  onToggle: (key: string, value: string) => void;
}

/** Toolbar trigger that adds a filter field, then chooses its values. */
export function CompaniesFilterTrigger({
  definitions,
  values,
  activeCount,
  visibleKeys,
  onAdd,
  onToggle,
}: CompaniesFilterControlsProps & { activeCount: number }) {
  return (
    <PMFilterTrigger
      definitions={definitions}
      values={values}
      visibleKeys={visibleKeys}
      activeCount={activeCount}
      onAdd={onAdd}
      onToggle={onToggle}
    />
  );
}

/** Applied-filter row below the toolbar: field, is, editable values, remove. */
export function CompaniesAppliedFilters({
  definitions,
  values,
  conditions,
  onConditionChange,
  optionModes,
  onOptionModeChange,
  visibleKeys,
  onToggle,
  onRemove,
  onClearAll,
}: Omit<CompaniesFilterControlsProps, 'onAdd'> & {
  conditions: CompanyFilterConditions;
  onConditionChange: (key: string, condition: PMFilterCondition) => void;
  optionModes: CompanyFilterModes;
  onOptionModeChange: (key: string, mode: PMFilterOptionMode) => void;
  onRemove: (key: string) => void;
  onClearAll: () => void;
}) {
  return (
    <PMFilterBar
      definitions={definitions}
      values={values}
      conditions={conditions}
      onConditionChange={onConditionChange}
      optionModes={optionModes}
      onOptionModeChange={onOptionModeChange}
      visibleKeys={visibleKeys}
      onToggle={onToggle}
      onRemove={onRemove}
      onClearAll={onClearAll}
    />
  );
}
