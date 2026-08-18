import { useMemo, useState } from "react";

import { CommentThread } from "@/components/pm/CommentThread";
import { UpdateActivityRow } from "@/components/pm/UpdateActivityRow";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Loading01Icon } from "@/lib/icons";
import type { ActivityLogEntry, CommentWithAuthor } from "@/lib/pmTypes";
import type { AssignableMember, WorkspaceTeam } from "@/lib/types";
import {
  epicActivityPresentation,
  filterRedundantEpicActivity,
} from "./epicUpdateEventLabel";

export type EpicUpdateFilter = "all" | "discussion" | "changes";

export type EpicUpdateEntry =
  | {
      id: string;
      kind: "comment";
      occurredAt: string;
      comment: CommentWithAuthor;
    }
  | {
      id: string;
      kind: "change";
      occurredAt: string;
      activity: ActivityLogEntry;
    };

const UPDATE_FILTERS: Array<{ value: EpicUpdateFilter; label: string }> = [
  { value: "all", label: "All" },
  { value: "discussion", label: "Discussion" },
  { value: "changes", label: "Changes" },
];

function isCommentActivity(entry: ActivityLogEntry) {
  return entry.activity.action?.startsWith("comment_") ?? false;
}

function occurredAtTimestamp(value: string) {
  const timestamp = Date.parse(value);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

export function buildEpicUpdateEntries(
  comments: CommentWithAuthor[],
  activity: ActivityLogEntry[],
  filter: EpicUpdateFilter,
): EpicUpdateEntry[] {
  const commentEntries: EpicUpdateEntry[] =
    filter === "changes"
      ? []
      : comments.map((comment) => ({
          id: `comment:${comment.comment.id}`,
          kind: "comment",
          occurredAt: comment.comment.created_at,
          comment,
        }));
  const changeEntries: EpicUpdateEntry[] =
    filter === "discussion"
      ? []
      : filterRedundantEpicActivity(activity)
          .filter((entry) => !isCommentActivity(entry))
          .map((entry) => ({
            id: `activity:${entry.activity.id}`,
            kind: "change",
            occurredAt: entry.activity.created_at,
            activity: entry,
          }));

  return [...commentEntries, ...changeEntries].sort(
    (left, right) =>
      occurredAtTimestamp(right.occurredAt) -
      occurredAtTimestamp(left.occurredAt),
  );
}

function EpicActivityRow({ entry }: { entry: ActivityLogEntry }) {
  const presentation = epicActivityPresentation(entry);
  const humanActor = presentation.agent?.automated ? undefined : entry.actor;

  return (
    <UpdateActivityRow
      label={presentation.title}
      occurredAt={entry.activity.created_at}
      emphasizedValues={presentation.emphasizedValues}
      detail={presentation.detail}
      humanActor={humanActor}
      agent={presentation.agent ? {
        name: presentation.agent.agentName,
        presetKey: presentation.agent.agentPresetKey,
      } : undefined}
    />
  );
}

interface EpicUpdatesViewProps {
  workspaceId: string;
  epicId: string;
  comments: CommentWithAuthor[];
  activity: ActivityLogEntry[];
  commentsLoading: boolean;
  activityLoading: boolean;
  currentUserId?: string;
  teams: Pick<WorkspaceTeam, "id" | "name" | "handle">[];
  members: AssignableMember[];
  onCommentsChange: (comments: CommentWithAuthor[]) => void;
}

export function EpicUpdatesView(props: EpicUpdatesViewProps) {
  const [filter, setFilter] = useState<EpicUpdateFilter>("all");
  const entries = useMemo(
    () => buildEpicUpdateEntries(props.comments, props.activity, filter),
    [props.activity, props.comments, filter],
  );
  const loading = props.commentsLoading || props.activityLoading;

  const updateSingleComment = (
    commentId: string,
    next: CommentWithAuthor[],
  ) => {
    const replacement = next.find((item) => item.comment.id === commentId);
    if (!replacement) {
      props.onCommentsChange(
        props.comments.filter((item) => item.comment.id !== commentId),
      );
      return;
    }
    props.onCommentsChange(
      props.comments.map((item) =>
        item.comment.id === commentId ? replacement : item,
      ),
    );
  };

  return (
    <div className="min-w-0 space-y-5">
      <CommentThread
        workspaceId={props.workspaceId}
        entityType="epic"
        entityId={props.epicId}
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
        <Tabs
          value={filter}
          onValueChange={(value) => setFilter(value as EpicUpdateFilter)}
        >
          <TabsList variant="line" aria-label="Update type" className="h-9 p-0">
            {UPDATE_FILTERS.map(({ value, label }) => (
              <TabsTrigger key={value} value={value}>
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <span className="ml-auto text-xs text-muted-foreground">
          Newest first
        </span>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
          <Loading01Icon className="h-4 w-4 animate-spin" />
          Loading updates…
        </div>
      ) : entries.length === 0 ? (
        <p className="py-8 text-sm text-muted-foreground">No updates yet.</p>
      ) : (
        <div className="min-w-0 divide-y divide-border/50 overflow-hidden">
          {entries.map((entry) =>
            entry.kind === "comment" ? (
              <div key={entry.id} className="py-2">
                <CommentThread
                  workspaceId={props.workspaceId}
                  entityType="epic"
                  entityId={props.epicId}
                  comments={[entry.comment]}
                  currentUserId={props.currentUserId}
                  teams={props.teams}
                  members={props.members}
                  onCommentsChange={(next) =>
                    updateSingleComment(entry.comment.comment.id, next)
                  }
                  composerVariant="update"
                  hideTopLevelComposer
                  hideEmptyState
                />
              </div>
            ) : (
              <EpicActivityRow key={entry.id} entry={entry.activity} />
            ),
          )}
        </div>
      )}
    </div>
  );
}
