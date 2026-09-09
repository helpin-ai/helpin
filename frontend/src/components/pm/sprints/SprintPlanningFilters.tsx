import { QuietSearchInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';

export type SprintStatusFilter = 'all' | 'upcoming' | 'active' | 'completed' | 'archived';

const STATUS_FILTER_OPTIONS: Array<{ value: SprintStatusFilter; label: string }> = [
  { value: 'all', label: 'All sprints' },
  { value: 'upcoming', label: 'Upcoming' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
  { value: 'archived', label: 'Archived' },
];

interface SprintPlanningFiltersProps {
  statusFilter: SprintStatusFilter;
  searchQuery: string;
  onStatusFilterChange: (value: SprintStatusFilter) => void;
  onSearchQueryChange: (value: string) => void;
}

export function SprintPlanningFilters({
  statusFilter,
  searchQuery,
  onStatusFilterChange,
  onSearchQueryChange,
}: SprintPlanningFiltersProps) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
      <QuietSearchInput
          containerClassName="w-full sm:w-[260px]"
          placeholder="Search sprints..."
          value={searchQuery}
          onChange={(event) => onSearchQueryChange(event.target.value)}
      />

      <Select value={statusFilter} onValueChange={(v) => onStatusFilterChange(v as SprintStatusFilter)}>
        <SelectTrigger className="h-9 w-[150px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {STATUS_FILTER_OPTIONS.map((opt) => (
            <SelectItem key={opt.value} value={opt.value}>
              {opt.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
