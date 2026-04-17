import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { useQueryClient } from '@tanstack/react-query';

import { ArrowDown01Icon, CheckmarkSquare02Icon, Loading01Icon, PlusSignIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { CreateTaskModal } from '@/components/pm/CreateTaskModal';
import { useTasks, useCreateTask } from '@/hooks/queries';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { associationsService } from '@/lib/services/associationsService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import type { CreateTaskRequest, Task, WorkflowWithStates } from '@/lib/pmTypes';
import type { CRMObjectType } from '@/lib/crmTypes';

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
  const addTaskDisabledReason = !canEdit
    ? 'You need PM edit access to create linked tasks.'
    : teams.length === 0
      ? 'Join a team to create linked tasks from CRM.'
      : null;

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
    <div className="rounded-md border border-border/60">
      <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-sm font-medium">
            <CheckmarkSquare02Icon className="h-4 w-4 text-muted-foreground" />
            <span>Tasks</span>
            {tasks.length > 0 && (
              <span className="text-xs text-muted-foreground">({tasks.length})</span>
            )}
          </div>
        </div>
        {teams.length > 1 ? (
          <div className="flex" title={addTaskDisabledReason ?? undefined}>
            <Button
              size="sm"
              className="h-7 gap-1.5 rounded-r-none text-xs"
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
                  size="sm"
                  className="h-7 rounded-l-none border-l border-primary-foreground/20 px-1.5"
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
            size="sm"
            className="h-7 gap-1.5 text-xs"
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

      <div className="divide-y divide-border/60">
        {isLoading ? (
          <div className="px-4 py-6 text-center text-xs text-muted-foreground">Loading…</div>
        ) : tasks.length === 0 ? (
          <div className="px-4 py-6 text-center text-xs text-muted-foreground">
            {canCreateTask ? 'No linked tasks yet.' : (addTaskDisabledReason ?? 'No linked tasks yet.')}
          </div>
        ) : (
          tasks.map((task) => (
            <button
              key={task.id}
              type="button"
              onClick={() =>
                openTaskRoute(navigate as never, location as never, workspaceSlug, task.id)
              }
              className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-muted/40"
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
                  {task.owner_name && <span>· {task.owner_name}</span>}
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
    </div>
  );
}
