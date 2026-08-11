import { useEffect, useMemo, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { BotIcon, GitBranchIcon, Loading01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { CommentThread } from '@/components/pm/CommentThread';
import { useMarkTaskUpdatesRead, useTaskUpdates } from '@/hooks/queries';
import type { AssignableMember, WorkspaceTeam } from '@/lib/types';
import type { CommentWithAuthor, TaskUpdateEntry, TaskUpdateFilter } from '@/lib/pmTypes';
import { taskUpdateEventLabel } from './taskUpdateEventLabel';

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
        {(['all', 'discussion', 'changes'] as TaskUpdateFilter[]).map((value) => (
          <Button key={value} type="button" variant={filter === value ? 'secondary' : 'ghost'} size="xs" className="capitalize" onClick={() => setFilter(value)}>
            {value}
          </Button>
        ))}
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
            <button
              key={entry.id}
              type="button"
              className="flex w-full items-center gap-3 px-2 py-3 text-left text-sm transition-colors hover:bg-muted/30"
              onClick={() => entry.kind === 'agent_run' ? props.onOpenDelivery(entry.agent_run?.id) : undefined}
            >
              <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full ${entry.kind === 'agent_run' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}>
                {entry.kind === 'agent_run' ? <BotIcon className="h-3.5 w-3.5" /> : entry.kind === 'git' ? <GitBranchIcon className="h-3.5 w-3.5" /> : <span className="h-1.5 w-1.5 rounded-full bg-current" />}
              </span>
              <span className="min-w-0 flex-1 text-foreground/75">{taskUpdateEventLabel(entry)}</span>
              {entry.kind === 'agent_run' && <span className="text-xs text-muted-foreground">View run →</span>}
              <time className="w-16 shrink-0 text-right text-xs text-muted-foreground">{formatDistanceToNow(new Date(entry.occurred_at), { addSuffix: true })}</time>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
