import type { ReactNode } from 'react';
import { QuietFilterDropdown, QuietSearchInput } from '@/components/design-system/quiet';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { PMFilterBar } from '@/components/pm/PMFilterControls';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';

export interface EpicFilterBarOption {
  value: string;
  label: string;
  leading?: ReactNode;
  labelClassName?: string;
}

export interface EpicFilterBarCategory {
  key: string;
  label: string;
  emptyLabel: string;
  options: EpicFilterBarOption[];
  selected: string[];
  icon?: ReactNode;
}

export interface EpicFilterBarGroupOption {
  value: string;
  label: string;
}

interface EpicFilterBarProps {
  /** Search text. */
  search: string;
  onSearchChange: (next: string) => void;

  /** Per-category multi-select filters, rendered in the order given. */
  categories: EpicFilterBarCategory[];
  onCategoryChange: (key: string, next: string[]) => void;
  onClearAll: () => void;
  ownerFilter?: ReactNode;

  /** "Show Archived" toggle (separate from the State filter's __archived__ value). */
  showArchived: boolean;
  onToggleShowArchived: (next: boolean) => void;

  /** Group-by control. */
  groupBy: string;
  groupByOptions: EpicFilterBarGroupOption[];
  onGroupByChange: (next: string) => void;

  /** Column visibility menu. */
  displayProperties: { key: string; label: string }[];
  visibleProperties: string[];
  onVisiblePropertiesChange: (next: string[]) => void;
}

/**
 * Filter and view controls, followed by the shared applied-filter pills.
 */
export function EpicFilterBar({
  search,
  onSearchChange,
  categories,
  onCategoryChange,
  onClearAll,
  ownerFilter,
  showArchived,
  onToggleShowArchived,
  groupBy,
  groupByOptions,
  onGroupByChange,
  displayProperties,
  visibleProperties,
  onVisiblePropertiesChange,
}: EpicFilterBarProps) {
  const visibleKeys = new Set(categories.filter((category) => category.selected.length > 0).map((category) => category.key));
  const values = Object.fromEntries(categories.map((category) => [category.key, category.selected]));
  const definitions = categories.map((category) => ({
    key: category.key,
    label: category.label,
    options: category.options.map((option) => ({
      value: option.value,
      label: option.label,
      icon: option.leading,
      labelClassName: option.labelClassName,
    })),
    searchableValues: ['owner', 'label', 'objective'].includes(category.key),
  }));
  const showToolbarClear = visibleKeys.size === 0 && (showArchived || search.trim().length > 0);

  return (
    <div className="flex flex-col">
      <div className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-4 pb-2 pt-2 md:px-6">
        <QuietSearchInput
          containerClassName="w-[220px] max-w-full"
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Search Epics…"
        />

        {categories.map((category) => category.key === 'owner' && ownerFilter ? (
          <div key={category.key} className="flex items-center gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">{category.label}</span>
            {ownerFilter}
          </div>
        ) : (
          <QuietFilterDropdown
            multiple
            key={category.key}
            label={category.label}
            emptyLabel={category.emptyLabel}
            icon={category.icon}
            options={category.options}
            selected={category.selected}
            onChange={(next) => onCategoryChange(category.key, next)}
          />
        ))}

        {showToolbarClear ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground"
            onClick={onClearAll}
          >
            Clear Filters
          </Button>
        ) : null}

        <label className="inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-md px-2 text-xs text-muted-foreground hover:bg-accent">
          <Checkbox
            checked={showArchived}
            onCheckedChange={(value) => onToggleShowArchived(value === true)}
            aria-label="Show archived"
          />
          Show Archived
        </label>

        <div className="ml-auto flex items-center gap-2">
          <QuietFilterDropdown label="Group by" showLabel="inline" value={groupBy} onChange={onGroupByChange} options={groupByOptions} />
          <DisplayPropertiesPopover
            allProperties={displayProperties}
            visible={visibleProperties}
            onChange={onVisiblePropertiesChange}
            iconOnly
          />
        </div>
      </div>
      <PMFilterBar
        definitions={definitions}
        values={values}
        visibleKeys={visibleKeys}
        onToggle={(key, value) => {
          const selected = values[key] ?? [];
          onCategoryChange(key, selected.includes(value)
            ? selected.filter((item) => item !== value)
            : [...selected, value]);
        }}
        onRemove={(key) => onCategoryChange(key, [])}
        onClearAll={onClearAll}
      />
    </div>
  );
}
