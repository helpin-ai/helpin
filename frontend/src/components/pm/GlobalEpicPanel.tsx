import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { GitBranchIcon, Loading01Icon } from '@/lib/icons';
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Separator } from '@/components/ui/separator';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { agentService } from '@/lib/services/agentService';
import { gitService } from '@/lib/services/gitService';
import type { EpicDeliveryTarget, EpicWithStats, Task } from '@/lib/pmTypes';
import { getEpicDoneTaskCount, getEpicTaskCount } from '@/lib/pmTypes';
import { useEpicPanelStore } from '@/stores/epicPanelStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useRegisterPageContext } from '@/components/command-bar/pageContext';
import {
  closeEpicRoute,
  type EpicOverlayLocationLike,
} from '@/components/pm/epic-detail/epicRouteNavigation';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { AgentPickerCard } from '@/components/pm/AgentPickerCard';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { useUpdateEpic } from '@/hooks/queries/useEpics';
import { EpicColorControl } from './EpicColorControl';
import { SaveIndicator } from './SaveIndicator';

interface GlobalEpicPanelProps {
  workspaceId: string;
}

function formatHealth(value: string | undefined) {
  if (!value || value === 'no_health') return 'No health';
  return value.replaceAll('_', ' ');
}

function formatDate(value: string | null | undefined) {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

export function GlobalEpicPanel({ workspaceId }: GlobalEpicPanelProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const overlayLocation = location as EpicOverlayLocationLike;
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const contextualEpicId = useEpicPanelStore((s) => s.epicId);
  const requestKey = useEpicPanelStore((s) => s.requestKey);
  const activeEpicId = contextualEpicId;

  const [epic, setEpic] = useState<EpicWithStats | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [deliveryTarget, setDeliveryTarget] = useState<EpicDeliveryTarget | null>(null);
  const [loading, setLoading] = useState(false);
  const [startingAgentRun, setStartingAgentRun] = useState(false);
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);
  const updateEpic = useUpdateEpic(workspaceId);
  const resetEpicUpdate = updateEpic.reset;
  useEffect(() => resetEpicUpdate(), [activeEpicId, resetEpicUpdate]);
  const openedAtRef = useRef<number | null>(null);
  const locationRef = useRef(overlayLocation);
  const navigateRef = useRef(navigate);
  locationRef.current = overlayLocation;
  navigateRef.current = navigate;

  const commandBarContext = useMemo(() => {
    if (!activeEpicId || !epic) return null;
    return {
      entity_type: 'epic' as const,
      entity_id: activeEpicId,
      display_title: epic.epic.name,
      related_ids: { task_ids: tasks.map((task) => task.id) },
    };
  }, [activeEpicId, epic, tasks]);
  useRegisterPageContext(commandBarContext, 25);

  const handleClose = useCallback(() => {
    if (!workspaceSlug) return;
    closeEpicRoute(navigateRef.current as never, locationRef.current, workspaceSlug);
  }, [workspaceSlug]);

  useEffect(() => {
    if (!activeEpicId || !workspaceId) {
      setEpic(null);
      setTasks([]);
      setDeliveryTarget(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    Promise.all([
      pmEpicService.get(workspaceId, activeEpicId),
      pmEpicService.listTasks(workspaceId, activeEpicId),
      gitService.getEpicDeliveryTarget(workspaceId, activeEpicId).catch(() => ({ data: null, error: 'Failed to load epic delivery target', status: 0 })),
    ]).then(([epicRes, tasksRes, deliveryRes]) => {
      if (cancelled) return;
      if (epicRes.error || !epicRes.data) {
        toast.error(epicRes.error || 'Failed to load epic');
        handleClose();
        return;
      }
      setEpic(epicRes.data);
      setTasks(tasksRes.data ?? []);
      setDeliveryTarget(deliveryRes.data ?? null);
      setLoading(false);
    }).catch(() => {
      if (cancelled) return;
      toast.error('Failed to load epic');
      setDeliveryTarget(null);
      setLoading(false);
      handleClose();
    });

    return () => {
      cancelled = true;
    };
  }, [activeEpicId, handleClose, requestKey, workspaceId]);

  useEffect(() => {
    if (activeEpicId) {
      openedAtRef.current = Date.now();
      return;
    }
    openedAtRef.current = null;
  }, [activeEpicId]);

  const doneCount = epic ? getEpicDoneTaskCount(epic.stats) : 0;
  const taskCount = epic ? getEpicTaskCount(epic.stats) : 0;
  const progress = taskCount > 0 ? Math.round((doneCount / taskCount) * 100) : 0;

  const refreshEpic = useCallback(async () => {
    if (!activeEpicId) return;
    const { data, error } = await pmEpicService.get(workspaceId, activeEpicId);
    if (error || !data) {
      toast.error(error || 'Failed to refresh epic');
      return;
    }
    setEpic(data);
  }, [activeEpicId, workspaceId]);

  const updateAssignedAgent = useCallback(async (agentId: string | undefined) => {
    if (!activeEpicId || !epic) return;
    const previous = epic;
    setEpic({ ...epic, epic: { ...epic.epic, assigned_agent_id: agentId } });
    const { data, error } = await pmEpicService.update(workspaceId, activeEpicId, { assigned_agent_id: agentId ?? '' });
    if (error || !data) {
      setEpic(previous);
      toast.error(error || 'Failed to update agent');
      return;
    }
    setEpic(data);
  }, [activeEpicId, epic, workspaceId]);

  const runAssignedAgent = useCallback(async () => {
    if (!activeEpicId || !epic?.epic.assigned_agent_id || startingAgentRun) return;
    setStartingAgentRun(true);
    try {
      const { error } = await agentService.runEpic(workspaceId, activeEpicId, { agent_id: epic.epic.assigned_agent_id });
      if (error) throw new Error(error);
      toast.success('Agent run started');
      await refreshEpic();
    } catch (err) {
      const reason = getUpgradeRequiredReason(err);
      if (reason) {
        setUpgradeDialogReason(reason);
        return;
      }
      toast.error(err instanceof Error ? err.message : 'Failed to start agent');
    } finally {
      setStartingAgentRun(false);
    }
  }, [activeEpicId, epic?.epic.assigned_agent_id, refreshEpic, startingAgentRun, workspaceId]);

  return (
    <Sheet
      open={Boolean(activeEpicId)}
      onOpenChange={(isOpen) => {
        if (!isOpen) handleClose();
      }}
    >
      <SheetContent
        side="right"
        className="p-0 data-[side=right]:w-[74vw] data-[side=right]:!max-w-[960px]"
        showCloseButton={false}
        onOpenAutoFocus={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => {
          if (openedAtRef.current && Date.now() - openedAtRef.current < 250) {
            event.preventDefault();
          }
        }}
      >
        <SheetTitle className="sr-only">Epic Detail</SheetTitle>
        {loading || !epic ? (
          <div className="flex h-full items-center justify-center">
            <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="flex h-full min-h-0 flex-col">
            <header className="border-b border-border px-6 py-5">
              <div className="mb-3 flex items-center gap-2 text-xs font-medium uppercase text-muted-foreground">
                <EpicColorControl
                  key={epic.epic.id}
                  value={epic.epic.color}
                  disabled={updateEpic.isPending}
                  onChange={canEdit ? (color) => {
                    updateEpic.mutate({ id: epic.epic.id, color }, {
                      onSuccess: (data) => setEpic((current) => current?.epic.id === data.epic.id ? data : current),
                    });
                  } : undefined}
                />
                Epic
              </div>
              {(updateEpic.isPending || updateEpic.isError) && (
                <SaveIndicator saving={updateEpic.isPending} error={updateEpic.error?.message} presentation="quiet" />
              )}
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <h2 className="truncate text-xl font-semibold text-foreground">{epic.epic.name}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {formatHealth(epic.epic.health)}
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    if (!workspaceSlug || !activeEpicId) return;
                    navigate({ to: '/w/$slug/pm/epics/$epicId', params: { slug: workspaceSlug, epicId: activeEpicId } });
                  }}
                >
                  Full page
                </Button>
              </div>
            </header>

            <div className="min-h-0 flex-1 overflow-auto px-6 py-5">
              <section className="space-y-3">
                <div className="flex items-center justify-between text-sm">
                  <span className="font-medium text-foreground">Progress</span>
                  <span className="text-muted-foreground">{doneCount}/{taskCount} tasks</span>
                </div>
                <Progress value={progress} className="h-2" />
                <div className="grid gap-3 text-sm md:grid-cols-2">
                  <div>
                    <div className="text-xs uppercase text-muted-foreground">Start</div>
                    <div className="mt-1 text-foreground">{formatDate(epic.epic.planned_start_date) || 'Not set'}</div>
                  </div>
                  <div>
                    <div className="text-xs uppercase text-muted-foreground">Deadline</div>
                    <div className="mt-1 text-foreground">{formatDate(epic.epic.deadline) || 'Not set'}</div>
                  </div>
                </div>
              </section>

              {deliveryTarget && (deliveryTarget.repo_full_name || deliveryTarget.epic_branch || deliveryTarget.final_pr_url) && (
                <>
                  <Separator className="my-5" />
                  <section>
                    <div className="mb-3 flex items-center gap-2">
                      <GitBranchIcon className="h-4 w-4 text-muted-foreground" />
                      <h3 className="text-sm font-semibold text-foreground">Delivery</h3>
                    </div>
                    <div className="grid gap-3 text-sm md:grid-cols-2">
                      <div>
                        <div className="text-xs uppercase text-muted-foreground">Repository</div>
                        <div className="mt-1 truncate text-foreground">{deliveryTarget.repo_full_name || 'Not configured'}</div>
                      </div>
                      <div>
                        <div className="text-xs uppercase text-muted-foreground">Status</div>
                        <div className="mt-1 text-foreground">{deliveryTarget.delivery_state.replaceAll('_', ' ')}</div>
                      </div>
                      <div>
                        <div className="text-xs uppercase text-muted-foreground">Base branch</div>
                        <div className="mt-1 truncate font-mono text-xs text-foreground">{deliveryTarget.base_branch || 'Not set'}</div>
                      </div>
                      <div>
                        <div className="text-xs uppercase text-muted-foreground">Epic branch</div>
                        <div className="mt-1 truncate font-mono text-xs text-foreground">{deliveryTarget.epic_branch || 'Not set'}</div>
                      </div>
                    </div>
                    {deliveryTarget.final_pr_url && (
                      <Button variant="outline" size="sm" className="mt-3" asChild>
                        <a href={deliveryTarget.final_pr_url} target="_blank" rel="noreferrer">
                          Final PR #{deliveryTarget.final_pr_number ?? ''}
                        </a>
                      </Button>
                    )}
                  </section>
                </>
              )}

              {epic.epic.description && (
                <>
                  <Separator className="my-5" />
                  <section>
                    <h3 className="mb-2 text-sm font-semibold text-foreground">Description</h3>
                    <div className="prose prose-sm max-w-none text-muted-foreground" dangerouslySetInnerHTML={{ __html: epic.epic.description }} />
                  </section>
                </>
              )}

              <Separator className="my-5" />
              <section>
                <AgentPickerCard
                  workspaceId={workspaceId}
                  runnableTarget="epic"
                  targetTeamId={epic.epic.team_id ?? null}
                  value={epic.epic.assigned_agent_id}
                  onChange={updateAssignedAgent}
                  hasRepoContext={Boolean(epic.epic.planning_repository_id)}
                  disabled={!canEdit}
                  onRun={runAssignedAgent}
                  running={startingAgentRun}
                />
              </section>

              <Separator className="my-5" />
              <section>
                <div className="mb-3 flex items-center justify-between">
                  <h3 className="text-sm font-semibold text-foreground">Tasks</h3>
                  <span className="text-xs text-muted-foreground">{tasks.length}</span>
                </div>
                <div className="space-y-2">
                  {tasks.length === 0 ? (
                    <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">No tasks in this epic.</div>
                  ) : tasks.slice(0, 20).map((task) => (
                    <button
                      key={task.id}
                      type="button"
                      className="flex w-full items-center justify-between gap-3 rounded-md border border-border bg-background px-3 py-2 text-left text-sm hover:bg-muted/50"
                      onClick={() => {
                        if (!workspaceSlug) return;
                        openTaskRoute(navigate as never, location as never, workspaceSlug, task.id);
                      }}
                    >
                      <span className="min-w-0 truncate font-medium text-foreground">{task.name}</span>
                      <span className="shrink-0 text-xs text-muted-foreground">{task.task_key || `#${task.display_id}`}</span>
                    </button>
                  ))}
                </div>
              </section>
            </div>
          </div>
      )}
      </SheetContent>
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeDialogReason(null);
        }}
        reason={upgradeDialogReason}
      />
    </Sheet>
  );
}
