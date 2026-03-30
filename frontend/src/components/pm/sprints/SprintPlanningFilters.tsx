import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
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
  teamName?: string;
  statusFilter: SprintStatusFilter;
  canEdit: boolean;
  onStatusFilterChange: (value: SprintStatusFilter) => void;
  onCreateSprint: () => void;
}

export function SprintPlanningFilters({
  teamName,
  statusFilter,
  canEdit,
  onStatusFilterChange,
  onCreateSprint,
}: SprintPlanningFiltersProps) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <h2 className="text-xl font-semibold">
          Sprints{teamName && <span className="font-normal text-muted-foreground"> ({teamName})</span>}
        </h2>
        <p className="text-sm text-muted-foreground">
          Plan and track work across time-boxed cycles.
        </p>
      </div>

      <div className="flex items-center gap-2">
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
          <Button size="sm" className="h-9 gap-2" onClick={onCreateSprint}>
            <Plus className="h-4 w-4" />
            Create Sprint
          </Button>
        )}
      </div>
    </div>
  );
}
