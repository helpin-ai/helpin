import { useMemo } from 'react';
import { PMFilterBar, PMFilterTrigger, type PMFilterDefinition } from '@/components/pm/PMFilterControls';
import type { CompanyFilterValues } from '@/lib/crmCompanyFilters';

interface CompaniesFilterControlsProps {
  definitions: PMFilterDefinition<string>[];
  values: CompanyFilterValues;
  visibleKeys: Set<string>;
  onAdd: (key: string) => void;
  onToggle: (key: string, value: string) => void;
}

/** Toolbar trigger that adds a filter field, then chooses its values. */
export function CompaniesFilterTrigger({ definitions, values, visibleKeys, onAdd, onToggle }: CompaniesFilterControlsProps) {
  const activeCount = useMemo(
    () => Object.values(values).filter((selected) => selected.length > 0).length,
    [values],
  );
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
  visibleKeys,
  onToggle,
  onRemove,
  onClearAll,
}: Omit<CompaniesFilterControlsProps, 'onAdd'> & { onRemove: (key: string) => void; onClearAll: () => void }) {
  return (
    <PMFilterBar
      definitions={definitions}
      values={values}
      visibleKeys={visibleKeys}
      onToggle={onToggle}
      onRemove={onRemove}
      onClearAll={onClearAll}
    />
  );
}
