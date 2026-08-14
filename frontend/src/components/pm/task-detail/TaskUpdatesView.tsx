import { useEffect, useMemo, useState } from 'react';
import { GitBranchIcon, Loading01Icon } from '@/lib/icons';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { CommentThread } from '@/components/pm/CommentThread';
import { UpdateActivityRow } from '@/components/pm/UpdateActivityRow';
import { useMarkTaskUpdatesRead, useTaskUpdates } from '@/hooks/queries';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import type { AgentRun, CommentWithAuthor, TaskUpdateEntry, TaskUpdateFilter } from '@/lib/pmTypes';
import { filterRedundantAgentLifecycleEntries, taskUpdateAgentPresentation, taskUpdateEventLabel } from './taskUpdateEventLabel';

interface TaskUpdatesViewProps {
  workspaceId: string;
  taskId: string;
  comments: CommentWithAuthor[];
  onCommentsChange: (comments: CommentWithAuthor[]) => void;
  currentUserId?: string;
  teams: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[];
  members: AssignableMember[];
  onOpenDelivery: (runId?: string) => void;
}

const UPDATE_FILTERS: Array<{ value: TaskUpdateFilter; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'discussion', label: 'Discussion' },
  { value: 'changes', label: 'Changes' },
];

function activityRunId(entry: TaskUpdateEntry) {
  const value = entry.activity?.metadata?.run_id;
  return typeof value === 'string' ? value : '';
}

function TaskSystemUpdateRow({
  entry,
  linkedRun,
  onOpenDelivery,
}: {
  entry: TaskUpdateEntry;
  linkedRun?: AgentRun;
  onOpenDelivery: (runId?: string) => void;
}) {
  const agentActivity = taskUpdateAgentPresentation(entry, linkedRun);
  const label = agentActivity?.title ?? taskUpdateEventLabel(entry, linkedRun);
  const humanActor = agentActivity && !agentActivity.automated && entry.kind === 'change'
    ? entry.actor
    : undefined;
  const humanActorName = humanActor?.full_name?.trim() || humanActor?.email?.trim() || '';
  return (
    <UpdateActivityRow
      label={label}
      occurredAt={entry.occurred_at}
      emphasizedValues={[humanActorName, agentActivity?.agentName ?? '']}
      detail={agentActivity?.detail}
      humanActor={humanActor}
      agent={agentActivity ? { name: agentActivity.agentName, presetKey: agentActivity.agentPresetKey } : undefined}
      fallbackIcon={entry.kind === 'git' ? <GitBranchIcon className="h-3.5 w-3.5" /> : undefined}
      actionLabel={agentActivity?.runId ? 'View run →' : undefined}
      onClick={agentActivity?.runId ? () => onOpenDelivery(agentActivity.runId) : undefined}
    />
  );
}

export function TaskUpdatesView(props: TaskUpdatesViewProps) {
  const [filter, setFilter] = useState<TaskUpdateFilter>('all');
  const updatesQuery = useTaskUpdates(props.workspaceId, props.taskId, filter);
  const markRead = useMarkTaskUpdatesRead(props.workspaceId, props.taskId);
  const data = updatesQuery.data;

  useEffect(() => {
    if (!data?.high_water || markRead.isPending) return;
    markRead.mutate(data.high_water);
  }, [data?.high_water]);

  const entries = useMemo(() => {
    const systemEntries = filterRedundantAgentLifecycleEntries(
      (data?.data ?? []).filter((entry) => entry.kind !== 'comment'),
    );
    const commentEntries: TaskUpdateEntry[] = filter === 'changes' ? [] : props.comments.map((comment) => ({
      id: `comment:${comment.comment.id}`,
      kind: 'comment',
      occurred_at: comment.comment.created_at,
      actor: comment.author,
      comment,
    }));
    return [...systemEntries, ...commentEntries].sort((a, b) => Date.parse(b.occurred_at) - Date.parse(a.occurred_at));
  }, [data?.data, filter, props.comments]);
  const agentRunsById = useMemo(() => new Map(
    (data?.data ?? [])
      .filter((entry): entry is TaskUpdateEntry & { agent_run: AgentRun } => entry.kind === 'agent_run' && !!entry.agent_run)
      .map((entry) => [entry.agent_run.id, entry.agent_run]),
  ), [data?.data]);

  const updateSingleComment = (commentId: string, next: CommentWithAuthor[]) => {
    const replacement = next.find((item) => item.comment.id === commentId);
    if (!replacement) {
      props.onCommentsChange(props.comments.filter((item) => item.comment.id !== commentId));
      return;
    }
    props.onCommentsChange(props.comments.map((item) => item.comment.id === commentId ? replacement : item));
  };

  return (
    <div className="min-w-0 space-y-5">
      <CommentThread
        workspaceId={props.workspaceId}
        entityType="task"
        entityId={props.taskId}
        comments={props.comments}
        currentUserId={props.currentUserId}
        teams={props.teams}
        members={props.members}
        onCommentsChange={props.onCommentsChange}
        composerPlacement="top"
        composerVariant="update"
        hideThreadList
        hideEmptyState
      />

      <div className="flex min-w-0 flex-wrap items-center gap-1 border-b border-border/50">
        <Tabs value={filter} onValueChange={(value) => setFilter(value as TaskUpdateFilter)}>
          <TabsList variant="line" aria-label="Update type" className="h-9 p-0">
            {UPDATE_FILTERS.map(({ value, label }) => (
              <TabsTrigger key={value} value={value}>
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <span className="ml-auto text-xs text-muted-foreground">Newest first</span>
      </div>

      {updatesQuery.isLoading ? (
        <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground"><Loading01Icon className="h-4 w-4 animate-spin" />Loading updates…</div>
      ) : entries.length === 0 ? (
        <p className="py-8 text-sm text-muted-foreground">No updates yet.</p>
      ) : (
        <div className="min-w-0 divide-y divide-border/50 overflow-hidden">
          {entries.map((entry) => entry.kind === 'comment' && entry.comment ? (
            <div key={entry.id} className="py-2">
              <CommentThread
                workspaceId={props.workspaceId}
                entityType="task"
                entityId={props.taskId}
                comments={[entry.comment]}
                currentUserId={props.currentUserId}
                teams={props.teams}
                members={props.members}
                onCommentsChange={(next) => updateSingleComment(entry.comment!.comment.id, next)}
                composerVariant="update"
                hideTopLevelComposer
                hideEmptyState
              />
            </div>
          ) : (
            <TaskSystemUpdateRow
              key={entry.id}
              entry={entry}
              linkedRun={agentRunsById.get(activityRunId(entry))}
              onOpenDelivery={props.onOpenDelivery}
            />
          ))}
        </div>
      )}
    </div>
  );
}
