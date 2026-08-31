import { Search01Icon } from '@/lib/icons';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

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
      <div className="relative w-full sm:w-[260px]">
        <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="search"
          placeholder="Search sprints..."
          value={searchQuery}
          onChange={(event) => onSearchQueryChange(event.target.value)}
          className="h-9 pl-8 text-sm"
        />
      </div>

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
