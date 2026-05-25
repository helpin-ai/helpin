import type { ReactNode } from 'react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CategoryFilterChip } from '@/components/pm/CategoryFilterChip';
import { DisplayPropertiesPopover } from '@/components/pm/DisplayPropertiesPopover';
import { Search01Icon } from '@/lib/icons';

export interface EpicFilterBarOption {
  value: string;
  label: string;
  leading?: ReactNode;
  labelClassName?: string;
}

export interface EpicFilterBarCategory {
  key: string;
  label: string;
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

  /** Per-category multi-select chips, rendered in the order given. */
  categories: EpicFilterBarCategory[];
  onCategoryChange: (key: string, next: string[]) => void;
  onClearAll: () => void;

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
 * The combined filter + view-controls row that sits above the Epics table.
 *
 * Layout (single row, wraps as needed):
 *   [🔍 Search] [Category chips…] [Clear Filters] [☐ Show Archived]
 *                                                  → Group By  Display
 */
export function EpicFilterBar({
  search,
  onSearchChange,
  categories,
  onCategoryChange,
  onClearAll,
  showArchived,
  onToggleShowArchived,
  groupBy,
  groupByOptions,
  onGroupByChange,
  displayProperties,
  visibleProperties,
  onVisiblePropertiesChange,
}: EpicFilterBarProps) {
  const hasAnyFilter = categories.some((cat) => cat.selected.length > 0) || search.length > 0;

  return (
    <div className="ui-divider-bottom-fade flex flex-col gap-1 px-4 pb-2 pt-2 md:px-6">
      <div className="flex flex-wrap items-end gap-2">
        <div className="flex flex-col gap-0.5">
          <span className="text-[11px] font-medium text-muted-foreground/0">
            &nbsp;
          </span>
          <div className="relative">
            <Search01Icon className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(event) => onSearchChange(event.target.value)}
              placeholder="Search Epics…"
              className="h-7 w-[220px] pl-7 text-xs"
            />
          </div>
        </div>

        {categories.map((category) => (
          <CategoryFilterChip
            key={category.key}
            label={category.label}
            icon={category.icon}
            options={category.options}
            selected={category.selected}
            onChange={(next) => onCategoryChange(category.key, next)}
          />
        ))}

        <div className="flex flex-col gap-0.5">
          <span className="text-[11px] font-medium text-muted-foreground/0">
            &nbsp;
          </span>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground"
            onClick={onClearAll}
            disabled={!hasAnyFilter}
          >
            Clear Filters
          </Button>
        </div>

        <div className="flex flex-col gap-0.5">
          <span className="text-[11px] font-medium text-muted-foreground/0">
            &nbsp;
          </span>
          <label className="inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-md px-2 text-xs text-muted-foreground hover:bg-accent">
            <Checkbox
              checked={showArchived}
              onCheckedChange={(value) => onToggleShowArchived(value === true)}
              aria-label="Show archived"
            />
            Show Archived
          </label>
        </div>

        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Group by:</span>
          <Select value={groupBy} onValueChange={onGroupByChange}>
            <SelectTrigger className="h-7 w-[140px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {groupByOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <DisplayPropertiesPopover
            allProperties={displayProperties}
            visible={visibleProperties}
            onChange={onVisiblePropertiesChange}
            iconOnly
          />
        </div>
      </div>
    </div>
  );
}
