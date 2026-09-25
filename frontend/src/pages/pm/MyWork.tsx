import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import {
  ChartColumnIcon,
  ClipboardIcon,
  PencilEdit02Icon,
  UserGroupIcon,
} from '@/lib/icons';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { AISuggestions } from '@/components/pm/my-work/AISuggestions';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import type { Task } from '@/lib/pmTypes';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { MyWorkTaskList } from '@/components/pm/my-work/MyWorkTaskList';
import { loadMyWorkTasks } from '@/components/pm/my-work/myWorkModel';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { isAgentRunEventDetail, type AgentRunEventDetail } from '@/lib/agentRunRealtime';
import {
  QuietEmptyState,
  QuietMetricGrid,
  QuietPageHeader,
  QuietPageViewport,
  QuietStatusText,
  QuietTextAction,
} from '@/components/design-system/quiet';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

type TaskMode = 'assigned' | 'requested';
type Mode = TaskMode | 'suggestions';

function normalizePauseReason(value: AgentRunEventDetail['pause_reason']) {
  if (!value || value === 'none') return null;
  return value as Task['latest_run_pause_reason'];
}

export function MyWorkPage() {
  useTitle('My Work');
  const navigate = useNavigate();
  const location = useLocation();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const wsSlug = workspace?.slug ?? '';

  const { data: access, isLoading: accessLoading } = useWorkspaceAccess(workspaceId);
  const memberId = access?.membership?.id;
  const { has } = usePermissions(access);

  const { teams, hasTeams, isAdmin, findTeamName, loading: teamsLoading } = useAccessibleTeams(workspaceId);

  const [mode, setMode] = useState<Mode>('assigned');
  const membersQuery = useWorkspaceMembers(mode === 'suggestions' ? '' : workspaceId);
  const [tasks, setTasks] = useState<Task[]>([]);
  const loadScope = `${workspaceId}:${memberId}:${mode}`;
  const [loadedMode, setLoadedMode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (!workspaceId || !memberId || mode === 'suggestions') return;
    let ignore = false;
    const filters =
      mode === 'assigned'
        ? { owner_member_ids: memberId, archived: false as const }
        : { requester_member_id: memberId, archived: false as const };

    loadMyWorkTasks(workspaceId, filters, () => ignore)
      .then((loaded) => {
        if (ignore || !loaded) return;
        setError(null);
        setTasks(loaded);
        setLoadedMode(loadScope);
      })
      .catch((requestError: unknown) => {
        if (ignore) return;
        setError(requestError instanceof Error ? requestError.message : 'Unable to load your work.');
        setLoadedMode(loadScope);
      });
    return () => {
      ignore = true;
    };
  }, [workspaceId, memberId, mode, loadScope, refreshKey]);

  useEffect(() => {
    if (!workspaceId) return;
    const handleAgentRunEvent = (event: Event) => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (!isAgentRunEventDetail(detail) || detail.entity !== 'agent_run' || detail.parent_type !== 'task' || !detail.parent_id) return;

      setTasks((currentTasks) => currentTasks.map((task) => {
        if (task.id !== detail.parent_id) return task;
        const nextPauseReason =
          detail.status && detail.status !== 'paused'
            ? null
            : Object.prototype.hasOwnProperty.call(detail, 'pause_reason')
              ? normalizePauseReason(detail.pause_reason)
              : task.latest_run_pause_reason;

        const nextRunId = detail.entity_id || task.latest_run_id;
        const nextAgentId = detail.agent_id || task.latest_run_agent_id;
        const nextStatus = detail.status || task.latest_run_status;
        if (
          nextRunId === task.latest_run_id
          && nextAgentId === task.latest_run_agent_id
          && nextStatus === task.latest_run_status
          && nextPauseReason === task.latest_run_pause_reason
        ) return task;

        return {
          ...task,
          latest_run_id: nextRunId,
          latest_run_agent_id: nextAgentId,
          latest_run_status: nextStatus,
          latest_run_pause_reason: nextPauseReason,
          latest_run_at: detail.sent_at || new Date().toISOString(),
        };
      }));
    };

    window.addEventListener('agent_run-updated', handleAgentRunEvent);
    return () => window.removeEventListener('agent_run-updated', handleAgentRunEvent);
  }, [workspaceId]);

  // Refresh list when a task is updated or archived via the global panel
  useEffect(() => {
    const refresh = () => setRefreshKey((k) => k + 1);
    const handleTaskCreated = (e: Event) => {
      const detail = (e as CustomEvent).detail;
      // Navigate to the team's task board so the user sees their new task
      if (detail?.teamId && wsSlug) {
        navigate({ to: '/w/$slug/pm/tasks', params: { slug: wsSlug }, search: { team: detail.teamId } });
        return;
      }
      setRefreshKey((k) => k + 1);
    };
    window.addEventListener('task-panel-updated', refresh);
    window.addEventListener('task-panel-archived', refresh);
    window.addEventListener('task-created', handleTaskCreated);
    return () => {
      window.removeEventListener('task-panel-updated', refresh);
      window.removeEventListener('task-panel-archived', refresh);
      window.removeEventListener('task-created', handleTaskCreated);
    };
  }, [navigate, wsSlug]);

  // ── Render ────────────────────────────────────────────────────────

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  const openTask = (task: Task) => {
    if (!wsSlug) return;
    openTaskRoute(navigate as never, location as never, wsSlug, task.id);
  };

  const showingLoading = accessLoading || teamsLoading || loadedMode !== loadScope;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader
        variant="shell"
        title="My Work"
      />

      <Tabs
        value={mode}
        onValueChange={(value) => {
          setError(null);
          setMode(value as Mode);
        }}
        className="min-h-0 flex-1 gap-0"
      >
        <TabsList variant="quiet" aria-label="My Work views" className="w-full shrink-0 justify-start overflow-x-auto px-4 sm:px-6 lg:px-8">
          <TabsTrigger value="assigned">Assigned to me</TabsTrigger>
          <TabsTrigger value="requested">Requested by me</TabsTrigger>
          <TabsTrigger value="suggestions">AI suggestions</TabsTrigger>
        </TabsList>

        <TabsContent value={mode} className="flex min-h-0 flex-1 flex-col">
        <QuietPageViewport className="min-h-0 flex-1">
          {mode === 'suggestions' ? (
            accessLoading ? <MyWorkLoadingState /> : memberId && has('pm.read') ? <AISuggestions key={`${workspaceId}:${memberId}`} ws={workspaceId} memberId={memberId} slug={wsSlug} canEdit={has('pm.edit')} canReadCRM={has('crm.read')} /> : <QuietEmptyState title="AI suggestions unavailable" description="PM access is needed to view your suggestions." />
          ) : showingLoading ? (
            <MyWorkLoadingState />
          ) : !hasTeams && !isAdmin ? (
            <NoTeamEmptyState />
          ) : error ? (
            <QuietEmptyState
              title="Unable to load your work"
              description={error}
              action={(
                <QuietTextAction
                  onClick={() => {
                    setError(null);
                    setLoadedMode(null);
                    setRefreshKey((key) => key + 1);
                  }}
                >
                  Try again
                </QuietTextAction>
              )}
            />
          ) : tasks.length === 0 ? (
            <MyWorkEmptyState mode={mode} />
          ) : (
            <MyWorkTaskList key={`${workspaceId}:${memberId}:${mode}`} tasks={tasks} workspaceId={workspaceId} memberId={memberId || ''} assigned={mode === 'assigned'} members={membersQuery.data || []} teams={teams} findTeamName={findTeamName} canEdit={has('pm.edit')} onOpen={openTask} onChanged={() => setRefreshKey(key => key + 1)} />
          )}
        </QuietPageViewport>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ── No team empty state ──────────────────────────────────────────

function NoTeamEmptyState() {
  return (
    <QuietEmptyState
      title="No team assigned"
      description="You need to be added to a team to see work items. Ask a workspace administrator to add you to a team."
    >
      <QuietStatusText tone="blocker">Team access required</QuietStatusText>
    </QuietEmptyState>
  );
}

// ── Empty state ───────────────────────────────────────────────────

const WORKFLOW_STEPS = [
  { icon: PencilEdit02Icon, title: 'Create tasks', description: 'Describe work to be done — bugs, features, or tasks' },
  { icon: UserGroupIcon, title: 'Assign to team', description: 'Set an owner, priority, and deadline for each task' },
  { icon: ChartColumnIcon, title: 'Track progress', description: 'Tasks move through workflow states as work gets done' },
];

function MyWorkEmptyState({ mode }: { mode: TaskMode }) {
  return (
    <div className="flex flex-col items-center px-4 py-16">
      <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-full bg-blue-500/10">
        <ClipboardIcon className="h-7 w-7 text-blue-500" />
      </div>
      <h3 className="mb-1 text-base font-medium">
        {mode === 'assigned' ? 'No tasks assigned to you yet' : 'No tasks requested by you yet'}
      </h3>
      <p className="max-w-md text-center text-sm text-muted-foreground">
        {mode === 'assigned'
          ? 'When teammates assign tasks to you, they appear here — prioritized so you always know what to focus on first.'
          : 'Tasks you create or request will appear here so you can track their progress.'}
      </p>

      <div className="mt-10 w-full max-w-4xl">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {WORKFLOW_STEPS.map(({ icon: Icon, title, description }) => (
            <div key={title} className="flex flex-col items-center rounded-lg border border-border/50 bg-muted/30 p-6 text-center">
              <Icon className="mb-3 h-5 w-5 text-muted-foreground" />
              <p className="mb-1 text-sm font-medium">{title}</p>
              <p className="text-sm leading-relaxed text-muted-foreground">{description}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function MyWorkLoadingState() {
  return (
    <div className="space-y-5" aria-live="polite" aria-label="Loading your work">
      <QuietMetricGrid>
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={index} className="space-y-2 p-4 motion-safe:animate-pulse">
            <div className="h-2 w-20 bg-quiet-icon-well" />
            <div className="h-5 w-10 bg-quiet-icon-well" />
            <div className="h-2 w-32 max-w-full bg-quiet-icon-well" />
          </div>
        ))}
      </QuietMetricGrid>
      <div className="border-t border-quiet-divider-strong">
        {Array.from({ length: 5 }).map((_, index) => (
          <div key={index} className="space-y-2 border-b border-quiet-divider-light px-5 py-3 motion-safe:animate-pulse">
            <div className="h-2 w-24 bg-quiet-icon-well" />
            <div className="h-3 w-3/5 bg-quiet-icon-well" />
            <div className="h-2 w-2/5 bg-quiet-icon-well" />
          </div>
        ))}
      </div>
    </div>
  );
}
