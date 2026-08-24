import type { AgentRun } from './agents';
import type { TaskGitLink } from './delivery';
import type { ActivityLogEntry, CommentWithAuthor } from './project';

export type TaskDetailView = 'overview' | 'delivery';
export type TaskUpdateFilter = 'all' | 'discussion' | 'changes';
export type TaskUpdateKind = 'comment' | 'change' | 'agent_run' | 'git';

export interface TaskUpdateEntry {
  id: string;
  kind: TaskUpdateKind;
  occurred_at: string;
  actor?: CommentWithAuthor['author'];
  comment?: CommentWithAuthor;
  activity?: ActivityLogEntry['activity'];
  agent_run?: AgentRun;
  agent_name?: string;
  git_link?: TaskGitLink;
}

export interface TaskUpdatesResponse {
  data: TaskUpdateEntry[];
  next_cursor?: string;
  high_water: string;
  unread_count: number;
  read_initialized: boolean;
}

export type TaskStandingBriefStatus = 'pending_refresh' | 'ready' | 'stale' | 'error';
export type TaskStandingBriefActionType =
  | 'retry_run'
  | 'reply_to_comment'
  | 'add_checklist_item'
  | 'open_related_object';

export interface TaskStandingBriefSuggestionAction {
  type: TaskStandingBriefActionType;
  target_id?: string;
  run_id?: string;
  comment_id?: string;
  text?: string;
}

export interface TaskStandingBriefSuggestion {
  key: string;
  label: string;
  action: TaskStandingBriefSuggestionAction;
  evidence?: string[];
}

export interface TaskStandingBriefEvidence {
  type: string;
  id: string;
  label: string;
  timestamp?: string;
}

export interface TaskStandingBrief {
  narrative: string;
  suggestions: TaskStandingBriefSuggestion[];
  evidence: TaskStandingBriefEvidence[];
  status: TaskStandingBriefStatus;
  is_stale: boolean;
  computed_at?: string;
  source_updated_at?: string;
}
