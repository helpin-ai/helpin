import { useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { GitBranchIcon, Loading01Icon } from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { CommentThread } from '@/components/pm/CommentThread';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useMarkTaskUpdatesRead, useTaskUpdates } from '@/hooks/queries';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import type { AgentRun, CommentWithAuthor, TaskUpdateEntry, TaskUpdateFilter } from '@/lib/pmTypes';
import { taskUpdateAgentPresentation, taskUpdateEventLabel } from './taskUpdateEventLabel';

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

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function EmphasizedActivityLabel({ label, names }: { label: string; names: string[] }) {
  const emphasizedNames = [...new Set(names.map((name) => name.trim()).filter((name) => name && name !== 'Agent'))]
    .sort((left, right) => right.length - left.length);
  if (emphasizedNames.length === 0) return label;

  const nameSet = new Set(emphasizedNames);
  const parts = label.split(new RegExp(`(${emphasizedNames.map(escapeRegExp).join('|')})`, 'g'));
  return parts.map((part, index) => nameSet.has(part)
    ? <span key={`${part}-${index}`} className="font-semibold text-foreground/90">{part}</span>
    : part);
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
  const rowClassName = 'flex w-full items-center gap-3 px-2 py-3 text-left text-sm transition-colors';
  const content = (
    <>
      {agentActivity ? (
        <span className="flex h-6 w-6 shrink-0 items-center justify-center">
          {humanActor ? (
            <UserAvatar
              name={humanActorName}
              avatarUrl={humanActor.avatar_url}
              avatarStyle={humanActor.avatar_style}
              avatarSeed={humanActor.avatar_seed}
              avatarBackgroundMode={humanActor.avatar_background_mode}
              avatarBackgroundColor={humanActor.avatar_background_color}
              className="h-4 w-4"
              fallbackClassName="text-[7px]"
            />
          ) : (
            <AgentAvatar
              name={agentActivity.agentName}
              presetKey={agentActivity.agentPresetKey}
              className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none"
              genericBare
            />
          )}
        </span>
      ) : (
        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
          {entry.kind === 'git'
            ? <GitBranchIcon className="h-3.5 w-3.5" />
            : <span className="h-1.5 w-1.5 rounded-full bg-current" />}
        </span>
      )}
      <span className="min-w-0 flex-1 truncate text-foreground/70">
        <EmphasizedActivityLabel
          label={label}
          names={[humanActorName, agentActivity?.agentName ?? '']}
        />
        {agentActivity?.detail ? <span className="text-muted-foreground"> — {agentActivity.detail}</span> : null}
      </span>
      {agentActivity?.runId ? <span className="shrink-0 text-xs text-muted-foreground">View run →</span> : null}
      <time className="w-16 shrink-0 text-right text-xs text-muted-foreground">
        {formatDistanceToNow(new Date(entry.occurred_at), { addSuffix: true })}
      </time>
    </>
  );

  if (agentActivity?.runId) {
    return (
      <button
        type="button"
        className={`${rowClassName} hover:bg-muted/30`}
        onClick={() => onOpenDelivery(agentActivity.runId)}
      >
        {content}
      </button>
    );
  }

  return <div className={rowClassName}>{content}</div>;
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
    const systemEntries = (data?.data ?? []).filter((entry) => entry.kind !== 'comment');
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
    <div className="space-y-5">
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
        hideThreadList
        hideEmptyState
      />

      <div className="flex items-center gap-1">
        <Tabs value={filter} onValueChange={(value) => setFilter(value as TaskUpdateFilter)}>
          <TabsList aria-label="Update type">
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
        <div className="divide-y divide-border/50">
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
