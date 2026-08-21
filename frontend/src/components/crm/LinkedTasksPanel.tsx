import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { useQueryClient } from '@tanstack/react-query';

import {
  ArrowDown01Icon,
  CheckmarkSquare02Icon,
  Link01Icon,
  Loading01Icon,
  PlusSignIcon,
  Search01Icon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { useTasks, useCreateTask } from '@/hooks/queries';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { associationsService } from '@/lib/services/associationsService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { searchService, type SearchResult } from '@/lib/services/searchService';
import type { CreateTaskRequest, Task, WorkflowWithStates } from '@/lib/pmTypes';
import type { CRMObjectType } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';

const CRM_TASK_DEFAULT_TEAM_KEY_PREFIX = 'crm-task-default-team:';

function defaultTeamStorageKey(workspaceId: string) {
  return `${CRM_TASK_DEFAULT_TEAM_KEY_PREFIX}${workspaceId}`;
}

function readStoredDefaultTeamId(workspaceId: string): string | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.localStorage.getItem(defaultTeamStorageKey(workspaceId));
  } catch {
    return null;
  }
}

function writeStoredDefaultTeamId(workspaceId: string, teamId: string) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(defaultTeamStorageKey(workspaceId), teamId);
  } catch {
    // Ignore storage failures; the picker still works for the current session.
  }
}

interface LinkedTasksPanelProps {
  workspaceId: string;
  workspaceSlug: string;
  contactId?: string;
  companyId?: string;
  dealId?: string;
  presentation?: 'default' | 'borderless';
}

function resolveObject(props: LinkedTasksPanelProps): { id: string; type: CRMObjectType } | null {
  if (props.contactId) return { id: props.contactId, type: 'contact' };
  if (props.companyId) return { id: props.companyId, type: 'company' };
  if (props.dealId) return { id: props.dealId, type: 'deal' };
  return null;
}

export function LinkedTasksPanel(props: LinkedTasksPanelProps) {
  const { workspaceId, workspaceSlug } = props;
  const target = resolveObject(props);
  const borderless = props.presentation === 'borderless';
  const navigate = useNavigate();
  const location = useLocation();
  const qc = useQueryClient();
  const createTask = useCreateTask(workspaceId);
  const { teams, allTeams, loading: teamsLoading } = useAccessibleTeams(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { canEdit } = usePermissions(access);
  const [selectedTeamId, setSelectedTeamId] = useState('');
  const [createTaskOpen, setCreateTaskOpen] = useState(false);
  const [createWorkflow, setCreateWorkflow] = useState<WorkflowWithStates | null>(null);
  const [openingCreate, setOpeningCreate] = useState(false);
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);
  const [linkQuery, setLinkQuery] = useState('');
  const [linkResults, setLinkResults] = useState<SearchResult[]>([]);
  const [linkSearching, setLinkSearching] = useState(false);
  const [linkingTaskId, setLinkingTaskId] = useState<string | null>(null);

  const { data, isLoading } = useTasks(workspaceId, {
    contact_id: props.contactId,
    company_id: props.companyId,
    deal_id: props.dealId,
  });

  const tasks: Task[] = data?.data ?? [];
  const teamNameById = useMemo(
    () => new Map(allTeams.map((team) => [team.id, team.name])),
    [allTeams],
  );
  const canCreateTask = canEdit && teams.length > 0;
  const targetLabel = target?.type === 'company' ? 'company' : target?.type === 'deal' ? 'deal' : 'contact';
  const addTaskDisabledReason = !canEdit
    ? 'You need PM edit access to create linked tasks.'
    : teams.length === 0
      ? 'Join a team to create linked tasks from CRM.'
      : null;

  useEffect(() => {
    if (!linkDialogOpen) {
      setLinkQuery('');
      setLinkResults([]);
      setLinkSearching(false);
      return;
    }
    if (linkQuery.trim().length < 2) {
      setLinkResults([]);
      return;
    }
    const handle = window.setTimeout(async () => {
      setLinkSearching(true);
      const response = await searchService.search(workspaceId, linkQuery.trim());
      const linkedIds = new Set(tasks.map((task) => task.id));
      setLinkResults((response.data?.tasks ?? []).filter((task) => !linkedIds.has(task.id)));
      setLinkSearching(false);
    }, 250);
    return () => window.clearTimeout(handle);
  }, [linkDialogOpen, linkQuery, workspaceId, tasks]);

  useEffect(() => {
    if (teams.length === 0) {
      setSelectedTeamId('');
      return;
    }

    setSelectedTeamId((current) => {
      if (current && teams.some((team) => team.id === current)) {
        return current;
      }

      const stored = readStoredDefaultTeamId(workspaceId);
      if (stored && teams.some((team) => team.id === stored)) {
        return stored;
      }

      return teams[0]?.id ?? '';
    });
  }, [teams, workspaceId]);

  if (!target) {
    return null;
  }
  const associationTarget = target;

  async function handleStartCreate(teamId?: string) {
    if (!canCreateTask || teamsLoading) return;

    const nextTeamId =
      teamId && teams.some((team) => team.id === teamId)
        ? teamId
        : selectedTeamId || teams[0]?.id || '';

    if (!nextTeamId) {
      toast.error('No team available for task creation');
      return;
    }

    setSelectedTeamId(nextTeamId);
    writeStoredDefaultTeamId(workspaceId, nextTeamId);
    setOpeningCreate(true);
    try {
      const workflow = await pmWorkflowService.resolveTeamWorkflow(workspaceId, nextTeamId);
      if (workflow.error || !workflow.data) {
        throw new Error(workflow.error ?? 'Could not resolve team workflow');
      }
      setCreateWorkflow(workflow.data);
      setCreateTaskOpen(true);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to open task creator';
      toast.error(msg);
    } finally {
      setOpeningCreate(false);
    }
  }

  async function handleLinkExistingTask(taskId: string) {
    if (linkingTaskId) return;
    setLinkingTaskId(taskId);
    try {
      const assoc = await associationsService.createAssociation({
        workspace_id: workspaceId,
        from_object_type: 'task',
        from_object_id: taskId,
        to_object_type: associationTarget.type,
        to_object_id: associationTarget.id,
      });
      if (assoc.error) {
        throw new Error(assoc.error);
      }
      toast.success('Task linked');
      qc.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks'] });
      setLinkDialogOpen(false);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to link task';
      toast.error(msg);
    } finally {
      setLinkingTaskId(null);
    }
  }

  async function handleCreateAndLinkTask(payload: CreateTaskRequest) {
    try {
      const created = await createTask.mutateAsync(payload);
      if (!created?.task?.id) {
        throw new Error('task create returned no data');
      }
      const assoc = await associationsService.createAssociation({
        workspace_id: workspaceId,
        from_object_type: 'task',
        from_object_id: created.task.id,
        to_object_type: associationTarget.type,
        to_object_id: associationTarget.id,
      });
      if (assoc.error) {
        throw new Error(assoc.error);
      }
      qc.invalidateQueries({ queryKey: ['pm', workspaceId, 'tasks'] });
      return created.task
        ? {
            id: created.task.id,
            task: {
              id: created.task.id,
              name: created.task.name,
              display_id: created.task.display_id,
              task_key: created.task.task_key,
            },
          }
        : undefined;
    } catch (err) {
      throw new Error(err instanceof Error ? err.message : 'Failed to create task');
    }
  }

  return (
    <div className={cn(borderless ? 'border-y border-border/60' : 'rounded-md border border-border/60')}>
      <div className={cn(
        'flex items-center justify-between border-b border-border/60 px-4 py-3',
        borderless && 'px-4 sm:px-6 lg:px-10',
      )}>
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium">
            <CheckmarkSquare02Icon className="h-4 w-4 text-muted-foreground" />
            <span>Tasks</span>
            {tasks.length > 0 && (
              <span className="text-xs text-muted-foreground">({tasks.length})</span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={borderless ? 'ghost' : 'outline'}
            size="sm"
            className={cn('h-7 gap-1.5 text-xs', borderless && 'text-muted-foreground hover:text-foreground')}
            onClick={() => setLinkDialogOpen(true)}
            disabled={!canEdit}
            title={!canEdit ? 'You need PM edit access to link tasks.' : undefined}
          >
            <Link01Icon className="h-3 w-3" />
            Link existing
          </Button>
          {teams.length > 1 ? (
          <div className="flex" title={addTaskDisabledReason ?? undefined}>
            <Button
              variant={borderless ? 'ghost' : 'default'}
              size="sm"
              className={cn('h-7 gap-1.5 rounded-r-none text-xs', borderless && 'text-muted-foreground hover:text-foreground')}
              onClick={() => void handleStartCreate()}
              disabled={!canCreateTask || teamsLoading || openingCreate}
            >
              {openingCreate ? (
                <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <PlusSignIcon className="h-3 w-3" />
              )}
              Add task
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant={borderless ? 'ghost' : 'default'}
                  size="sm"
                  className={cn(
                    'h-7 rounded-l-none border-l px-1.5',
                    borderless ? 'border-border/60 text-muted-foreground' : 'border-primary-foreground/20',
                  )}
                  disabled={!canCreateTask || teamsLoading || openingCreate}
                  aria-label="Choose team for task creation"
                >
                  <ArrowDown01Icon className="h-3 w-3" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-44">
                {teams.map((team) => (
                  <DropdownMenuItem
                    key={team.id}
                    onClick={() => void handleStartCreate(team.id)}
                  >
                    <span className="flex min-w-0 flex-1 items-center justify-between gap-3">
                      <span className="truncate">{team.name}</span>
                      {team.id === selectedTeamId && (
                        <span className="text-[10px] uppercase tracking-wide text-muted-foreground">
                          Default
                        </span>
                      )}
                    </span>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        ) : (
          <Button
            variant={borderless ? 'ghost' : 'default'}
            size="sm"
            className={cn('h-7 gap-1.5 text-xs', borderless && 'text-muted-foreground hover:text-foreground')}
            onClick={() => void handleStartCreate()}
            disabled={!canCreateTask || teamsLoading || openingCreate}
            title={addTaskDisabledReason ?? undefined}
          >
            {openingCreate ? (
              <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <PlusSignIcon className="h-3 w-3" />
            )}
            Add task
          </Button>
        )}
        </div>
      </div>

      <div className="divide-y divide-border/60">
        {isLoading ? (
          <div className="px-4 py-6 text-center text-xs text-muted-foreground">Loading…</div>
        ) : tasks.length === 0 ? (
          <div className={cn('flex flex-col items-center justify-center px-6 py-10 text-center', borderless && 'py-11')}>
            <div className={cn(!borderless && 'rounded-full bg-muted p-2.5')}>
              <CheckmarkSquare02Icon className={cn('h-5 w-5 text-muted-foreground', borderless && 'text-muted-foreground/45')} />
            </div>
            <p className="mt-3 text-sm font-medium">No linked tasks yet</p>
            <p className="mt-1 max-w-xs text-xs text-muted-foreground">
              {canCreateTask
                ? `Link or create a task to track follow-ups for this ${targetLabel}.`
                : (addTaskDisabledReason ?? 'No linked tasks yet.')}
            </p>
          </div>
        ) : (
          tasks.map((task) => (
            <button
              key={task.id}
              type="button"
              onClick={() =>
                openTaskRoute(navigate as never, location as never, workspaceSlug, task.id)
              }
              className={cn(
                'flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-muted/40',
                borderless && 'sm:px-6 lg:px-10',
              )}
            >
              <CheckmarkSquare02Icon
                className={`h-4 w-4 shrink-0 ${task.completed ? 'text-emerald-500' : 'text-muted-foreground'}`}
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1 truncate font-medium text-foreground">{task.name}</div>
                  {(task.team_name || (task.team_id ? teamNameById.get(task.team_id) : null)) && (
                    <span className="shrink-0 rounded-sm bg-muted px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-foreground/80">
                      {task.team_name || (task.team_id ? teamNameById.get(task.team_id) : null)}
                    </span>
                  )}
                </div>
                <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
                  <span>{task.task_key}</span>
                  {task.state_name && <span>· {task.state_name}</span>}
                </div>
              </div>
            </button>
          ))
        )}
      </div>

      {createWorkflow && (
        <CreateTaskModal
          open={createTaskOpen}
          onOpenChange={setCreateTaskOpen}
          workspaceId={workspaceId}
          workflow={createWorkflow}
          initialStateId={
            createWorkflow.workflow.default_state_id ??
            createWorkflow.states.find((state) => state.is_default)?.id ??
            createWorkflow.states[0]?.id ??
            ''
          }
          initialTeamId={selectedTeamId || createWorkflow.workflow.team_id}
          onCreate={handleCreateAndLinkTask}
        />
      )}

      <Dialog open={linkDialogOpen} onOpenChange={setLinkDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">Link existing task</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="relative">
              <Search01Icon className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                value={linkQuery}
                onChange={(event) => setLinkQuery(event.target.value)}
                placeholder="Search tasks by name or key"
                className="pl-9"
                autoFocus
              />
            </div>
            <div className="max-h-64 space-y-1 overflow-y-auto">
              {linkSearching && (
                <div className="flex items-center justify-center gap-2 py-4 text-sm text-muted-foreground">
                  <Loading01Icon className="h-4 w-4 animate-spin" />
                  Searching…
                </div>
              )}
              {!linkSearching && linkResults.map((result) => (
                <button
                  key={result.id}
                  type="button"
                  disabled={linkingTaskId !== null}
                  className="flex w-full items-center gap-2 rounded-md border border-border/60 px-3 py-2 text-left text-sm transition hover:bg-accent disabled:opacity-60"
                  onClick={() => void handleLinkExistingTask(result.id)}
                >
                  <CheckmarkSquare02Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate font-medium">{result.name}</span>
                  {result.display_id && (
                    <Badge variant="outline" className="h-5 shrink-0 px-1.5 text-[10px]">
                      #{result.display_id}
                    </Badge>
                  )}
                  {linkingTaskId === result.id && (
                    <Loading01Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" />
                  )}
                </button>
              ))}
              {!linkSearching && linkQuery.trim().length >= 2 && linkResults.length === 0 && (
                <p className="py-4 text-center text-sm text-muted-foreground">No tasks found</p>
              )}
              {!linkSearching && linkQuery.trim().length < 2 && (
                <p className="py-4 text-center text-sm text-muted-foreground">
                  Type at least 2 characters to search
                </p>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
