import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import {
  CalendarDays,
  Crosshair,
  Hexagon,
  Loader2,
  MoreHorizontal,
  Target,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useGlobalCreateStore } from '@/stores/globalCreateStore';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import type { ObjectiveState, ObjectiveWithDetails } from '@/lib/pmTypes';
import { OBJECTIVE_STATE_CONFIG } from '@/lib/pmConstants';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Archive } from 'lucide-react';

const healthConfig: Record<string, { label: string; className: string }> = {
  on_track: { label: 'On Track', className: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  at_risk: { label: 'At Risk', className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  off_track: { label: 'Off Track', className: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
};

export function ObjectivesPage() {
  useTitle('Objectives');

  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id;
  const navigate = useNavigate();
  const openCreate = useGlobalCreateStore((s) => s.openCreate);

  const [objectives, setObjectives] = useState<ObjectiveWithDetails[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    const { data } = await pmObjectiveService.list(workspaceId, { archived: false });
    setObjectives(data ?? []);
    setLoading(false);
  }, [workspaceId]);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    const handler = () => load();
    window.addEventListener('objective-created', handler);
    return () => window.removeEventListener('objective-created', handler);
  }, [load]);

  const handleArchive = async (id: string) => {
    if (!workspaceId) return;
    await pmObjectiveService.remove(workspaceId, id);
    load();
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (objectives.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <Target className="h-12 w-12 text-muted-foreground/40" />
        <h2 className="mt-4 text-xl font-semibold">No Objectives yet</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Create your first objective to start tracking goals.
        </p>
        <Button className="mt-4" size="sm" onClick={() => openCreate('objective')}>
          Create Objective
        </Button>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <h1 className="text-lg font-semibold">Objectives</h1>
        <Button size="sm" onClick={() => openCreate('objective')}>
          Create Objective
        </Button>
      </div>

      <div className="grid gap-4 sm:grid-cols-1 lg:grid-cols-2">
        {objectives.map((obj) => (
          <ObjectiveCard
            key={obj.objective.id}
            data={obj}
            onArchive={() => handleArchive(obj.objective.id)}
            onClick={() => navigate({ to: `/w/${workspace!.slug}/pm/objectives/${obj.objective.id}` } as any)}
          />
        ))}
      </div>
    </div>
  );
}

function ObjectiveCard({
  data,
  onArchive,
  onClick,
}: {
  data: ObjectiveWithDetails;
  onArchive: () => void;
  onClick: () => void;
}) {
  const { objective, stats, epics } = data;
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const isStrategic = objective.objective_type === 'strategic';
  const stateCfg = OBJECTIVE_STATE_CONFIG[objective.state as ObjectiveState] ?? OBJECTIVE_STATE_CONFIG.not_started;

  const krProgress = Math.round(stats.key_result_avg_pct);
  const epicProgress = Math.round(stats.epic_progress_pct);

  const dateRange = useMemo(() => {
    const parts: string[] = [];
    if (objective.planned_start_date) parts.push(objective.planned_start_date.slice(0, 10));
    if (objective.deadline) parts.push(objective.deadline.slice(0, 10));
    return parts.join(' → ');
  }, [objective.planned_start_date, objective.deadline]);

  return (
    <article className="rounded-lg border border-border/70 bg-card p-4 transition-shadow hover:shadow-sm">
      {/* Type + Title row */}
      <div className="flex items-start gap-2">
        <div className={`mt-0.5 flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium ${
          isStrategic
            ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
            : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
        }`}>
          {isStrategic ? <Crosshair className="h-3 w-3" /> : <Target className="h-3 w-3" />}
          {isStrategic ? 'Strategic' : 'Tactical'}
        </div>
        <div className="min-w-0 flex-1">
          <button
            type="button"
            className="text-sm font-semibold text-foreground hover:underline cursor-pointer text-left"
            onClick={onClick}
          >
            {objective.name}
          </button>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0 text-muted-foreground">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={onClick}>Edit</DropdownMenuItem>
            <DropdownMenuItem onClick={() => setArchiveConfirmOpen(true)}>
              <Archive className="mr-2 h-4 w-4 text-amber-500" />
              Archive
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Progress bars */}
      {stats.key_result_count > 0 ? (
        <div className="mt-3 grid grid-cols-2 gap-4">
          <div>
            <div className="flex items-center justify-between text-[10px] text-muted-foreground mb-1">
              <span>Outcome Progress</span>
              <span>{krProgress}%</span>
            </div>
            <Progress value={krProgress} className="h-1.5" />
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px] text-muted-foreground mb-1">
              <span>Execution Progress</span>
              <span>{epicProgress}%</span>
            </div>
            <Progress value={epicProgress} className="h-1.5" />
          </div>
        </div>
      ) : (
        <div className="mt-3">
          <div className="flex items-center justify-between text-[10px] text-muted-foreground mb-1">
            <span>Progress</span>
            <span>{epicProgress}%</span>
          </div>
          <Progress value={epicProgress} className="h-1.5" />
        </div>
      )}

      {/* State + health + date row */}
      <div className="mt-3 flex items-center gap-2 text-xs">
        <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${stateCfg.badge}`}>
          {stateCfg.label}
        </span>
        {objective.state !== 'closed' && (
          <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${(healthConfig[objective.health] ?? healthConfig.on_track).className}`}>
            {(healthConfig[objective.health] ?? healthConfig.on_track).label}
          </span>
        )}
        {dateRange && (
          <span className="flex items-center gap-1 text-muted-foreground">
            <CalendarDays className="h-3 w-3" />
            {dateRange}
          </span>
        )}
      </div>

      {/* Linked epics */}
      {epics.length > 0 && (
        <div className="mt-3 space-y-1">
          {epics.slice(0, 5).map((e) => {
            const epicPct = e.stats.story_count > 0
              ? Math.round((e.stats.done_story_count / e.stats.story_count) * 100)
              : 0;
            return (
              <div key={e.epic.id} className="flex items-center gap-2 text-xs">
                <Hexagon className="h-3 w-3 shrink-0 text-violet-500" />
                <span className="min-w-0 flex-1 truncate">{e.epic.name}</span>
                <div className="w-16">
                  <Progress value={epicPct} className="h-1" />
                </div>
                <span className="w-8 text-right text-muted-foreground">{epicPct}%</span>
              </div>
            );
          })}
          {epics.length > 5 && (
            <p className="text-[10px] text-muted-foreground">+{epics.length - 5} more</p>
          )}
        </div>
      )}
      <ConfirmDialog
        open={archiveConfirmOpen}
        onOpenChange={setArchiveConfirmOpen}
        title="Archive objective"
        description="This objective will be hidden from the list. You can restore it later from archived items."
        confirmLabel="Archive"
        variant="default"
        onConfirm={onArchive}
      />
    </article>
  );
}
