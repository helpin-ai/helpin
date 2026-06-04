import { PlusSignIcon, Search01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

export type SprintStatusFilter = 'all' | 'upcoming' | 'active' | 'completed' | 'archived';

const STATUS_FILTER_OPTIONS: Array<{ value: SprintStatusFilter; label: string }> = [
  { value: 'all', label: 'All sprints' },
  { value: 'upcoming', label: 'Upcoming' },
  { value: 'active', label: 'Active' },
  { value: 'completed', label: 'Completed' },
  { value: 'archived', label: 'Archived' },
];

interface SprintPlanningFiltersProps {
  teamName?: string;
  statusFilter: SprintStatusFilter;
  searchQuery: string;
  canEdit: boolean;
  canCreateSprint: boolean;
  onStatusFilterChange: (value: SprintStatusFilter) => void;
  onSearchQueryChange: (value: string) => void;
  onCreateSprint: () => void;
}

export function SprintPlanningFilters({
  teamName,
  statusFilter,
  searchQuery,
  canEdit,
  canCreateSprint,
  onStatusFilterChange,
  onSearchQueryChange,
  onCreateSprint,
}: SprintPlanningFiltersProps) {
  return (
    <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h2 className="text-xl font-semibold">
          Sprints{teamName && <span className="font-normal text-muted-foreground"> ({teamName})</span>}
        </h2>
        <p className="text-sm text-muted-foreground">
          Plan and track work across time-boxed cycles.
        </p>
      </div>

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

        {canEdit && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="inline-flex">
                  <Button size="sm" className="h-9 gap-2" onClick={onCreateSprint} disabled={!canCreateSprint}>
                    <PlusSignIcon className="h-4 w-4" />
                    Create Sprint
                  </Button>
                </span>
              </TooltipTrigger>
              {!canCreateSprint && (
                <TooltipContent side="top" className="max-w-[260px] text-xs">
                  Only team managers can create sprints. Ask your team manager for access.
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        )}
      </div>
    </div>
  );
}
