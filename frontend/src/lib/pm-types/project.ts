import type { AssignableMember } from '../types';
import type { StoryImplementationBrief } from './agents';
import type { Objective } from './objectives';

export type TaskType = 'feature' | 'bug' | 'chore';
export type StoryType = TaskType;
export type StateType = 'backlog' | 'unstarted' | 'started' | 'done';
export type Priority = 'none' | 'low' | 'medium' | 'high' | 'urgent';
export type Severity = 'none' | 'minor' | 'major' | 'critical';
export type EpicHealth = 'no_health' | 'on_track' | 'at_risk' | 'off_track';
export type SprintStatus = 'unstarted' | 'started' | 'done';
export type RecurringTemplateStatus = 'active' | 'paused' | 'stopped' | 'failed';
export type RecurringScheduleType = 'time' | 'completion';
export type RecurringFrequency = 'daily' | 'weekly' | 'monthly' | 'yearly';
export type RecurringCompletionEvent = 'completed' | 'done_state';
export type RecurringDueDateMode = 'none' | 'scheduled_date' | 'offset_days';
export type RecurringSprintAssignmentMode = 'none' | 'current_sprint' | 'by_due_date';
export type RecurringRunStatus = 'succeeded' | 'failed' | 'skipped';
export type RecurringRunTrigger = 'manual_seed' | 'schedule' | 'completion' | 'generate_now';

export interface Workflow {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  team_id?: string;
  default_state_id?: string;
  auto_assign_owner: boolean;
  created_at: string;
  updated_at: string;
}

export interface WorkflowState {
  id: string;
  workflow_id: string;
  name: string;
  state_type: StateType;
  position: number;
  color?: string;
  description?: string;
  wip_limit?: number;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface EpicWorkflowState {
  id: string;
  workspace_id: string;
  name: string;
  state_type: Exclude<StateType, 'backlog'>;
  position: number;
  color?: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface WorkflowWithStates {
  workflow: Workflow;
  states: WorkflowState[];
}

export interface Label {
  id: string;
  workspace_id: string;
  team_id?: string;
  name: string;
  description?: string;
  color?: string;
  archived: boolean;
  created_at: string;
  updated_at: string;
}

export interface Epic {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  epic_state_id?: string;
  owner_member_id?: string;
  team_id?: string;
  planned_start_date?: string;
  deadline?: string;
  started: boolean;
  started_at?: string;
  completed: boolean;
  completed_at?: string;
  position: number;
  color?: string;
  health: EpicHealth;
  health_comment?: string;
  archived: boolean;
  spec_document_id?: string;
  planning_repository_id?: string;
  planning_state: string;
  spec_clarifications?: SpecClarification[];
  spec_clarified_at?: string;
  spec_clarified_by?: string;
  approved_spec_version_id?: string;
  last_planning_run_id?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface SpecClarification {
  id: string;
  kind: 'open_question' | 'assumption';
  prompt: string;
  disposition?: 'pending' | 'answered' | 'accepted' | 'rejected';
  response?: string;
}

export interface EpicStats {
  story_count: number;
  done_story_count: number;
  total_points: number;
  done_points: number;
  in_progress_count: number;
  unstarted_count: number;
}

export interface RoadmapObjectiveRef {
  id: string;
  name: string;
}

export interface EpicWithStats {
  epic: Epic;
  labels: Label[];
  objectives: RoadmapObjectiveRef[];
  stats: EpicStats;
  suggested_health: EpicHealth;
}

export type RoadmapEpic = EpicWithStats;

export interface RoadmapData {
  epics: EpicWithStats[];
  objectives: Objective[];
}

export interface PMSprint {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  start_date: string | null;
  end_date: string | null;
  status: SprintStatus;
  team_id?: string;
  archived: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface PMSprintStats {
  story_count: number;
  done_story_count: number;
  total_points: number;
  done_points: number;
}

export interface SprintWithStats {
  sprint: PMSprint;
  labels: Label[];
  stats: PMSprintStats;
}

export interface SprintPlanningStoryPreview {
  id: string;
  display_id: number;
  name: string;
  workflow_state_id: string;
  state_name?: string;
  state_type?: StateType;
  owner_member_id?: string;
  estimate?: number;
  priority: Priority;
  sprint_id?: string;
  team_id?: string;
}

export interface SprintPlanningCard {
  sprint: PMSprint;
  stats: PMSprintStats;
  preview_stories: SprintPlanningStoryPreview[];
  story_preview_overflow: number;
}

export interface SprintPlanningBucket {
  key: 'active' | 'upcoming' | 'completed';
  label: string;
  sprints: SprintPlanningCard[];
}

export interface SprintPlanningWorkspace {
  buckets: SprintPlanningBucket[];
  backlog_stories: SprintPlanningStoryPreview[];
  backlog_total: number;
}

export interface Task {
  id: string;
  workspace_id: string;
  display_id: number;
  name: string;
  description?: string;
  task_type: TaskType;
  workflow_id: string;
  workflow_state_id: string;
  epic_id?: string;
  sprint_id?: string;
  team_id?: string;
  owner_member_id?: string;
  requester_member_id?: string;
  estimate?: number;
  priority: Priority;
  severity: Severity;
  deadline?: string;
  position: number;
  started: boolean;
  started_at?: string;
  completed: boolean;
  completed_at?: string;
  moved_at?: string;
  blocked: boolean;
  blocker?: string;
  is_blocked_by_task?: boolean;
  blocked_by_count?: number;
  is_blocking_other_task?: boolean;
  blocking_count?: number;
  blocked_by_tasks?: TaskDependencyTask[];
  blocking_tasks?: TaskDependencyTask[];
  archived: boolean;
  assigned_agent_id?: string;
  template_id?: string;
  recurring_template_id?: string;
  recurring_run_id?: string;
  recurring_occurrence_number?: number;
  external_id?: string;
  slice_type?: string;
  implementation_brief?: StoryImplementationBrief;
  created_at: string;
  updated_at: string;
  // Enriched by board/list endpoints
  epic_name?: string;
  sprint_name?: string;
  owner_name?: string;
  state_name?: string;
  state_type?: StateType;
  state_color?: string;
  labels?: Label[];
  contacts?: AssociationObjectSummary[];
  companies?: AssociationObjectSummary[];
  deals?: AssociationObjectSummary[];
  support_conversations?: AssociationObjectSummary[];
  /** @deprecated Use task_type */
  story_type?: TaskType;
  /** @deprecated Use is_blocked_by_task */
  is_blocked_by_story?: boolean;
  /** @deprecated Use is_blocking_other_task */
  is_blocking_other_story?: boolean;
  /** @deprecated Use blocked_by_tasks */
  blocked_by_stories?: TaskDependencyTask[];
  /** @deprecated Use blocking_tasks */
  blocking_stories?: TaskDependencyTask[];
}

/** @deprecated Use Task */
export type Story = Task;

export interface TaskDependencyTask {
  id: string;
  display_id: number;
  name: string;
  workflow_state_id: string;
  completed: boolean;
}

/** @deprecated Use TaskDependencyTask */
export type StoryDependencyStory = TaskDependencyTask;

export type AssociationEntityType =
  | 'task'
  | 'story'
  | 'epic'
  | 'support_conversation'
  | 'contact'
  | 'company'
  | 'deal'
  | 'document';

export type TaskRelationshipAction =
  | 'relates_to'
  | 'blocks'
  | 'is_blocked_by'
  | 'duplicates'
  | 'is_duplicated_by';

export type StoryRelationshipAction = TaskRelationshipAction;

export interface AssociationObjectSummary {
  association_id?: string;
  object_type: AssociationEntityType | string;
  object_id: string;
  display_id?: string;
  title: string;
  status?: string;
  workflow_state_id?: string;
  completed?: boolean;
  task_type?: TaskType;
  /** @deprecated Use task_type */
  story_type?: StoryType;
}

export interface StoryRelationshipSummary {
  relationship_id: string;
  link_type: string;
  is_active: boolean;
  story: AssociationObjectSummary;
}

export type TaskRelationshipSummary = StoryRelationshipSummary;

export interface StoryRelationshipGroups {
  blocked_by: StoryRelationshipSummary[];
  blocking: StoryRelationshipSummary[];
  relates_to: StoryRelationshipSummary[];
  related_by: StoryRelationshipSummary[];
  duplicates: StoryRelationshipSummary[];
  duplicated_by: StoryRelationshipSummary[];
}

export type TaskRelationshipGroups = StoryRelationshipGroups;

export interface GroupedAssociations {
  task_relationships: TaskRelationshipGroups;
  /** @deprecated Use task_relationships */
  story_relationships?: StoryRelationshipGroups;
  stories: AssociationObjectSummary[];
  support_conversations: AssociationObjectSummary[];
  crm_records: AssociationObjectSummary[];
  docs: AssociationObjectSummary[];
}

export type GroupedTaskAssociations = GroupedAssociations;

export interface CreateTaskRelationshipRequest {
  relationship_type: TaskRelationshipAction;
  other_task_id: string;
}

/** @deprecated Use CreateTaskRelationshipRequest */
export type CreateStoryRelationshipRequest = CreateTaskRelationshipRequest;

export type CreateTaskRelationshipPayload = CreateTaskRelationshipRequest;

export interface TaskDetail {
  task: Task;
  /** @deprecated Use task */
  story?: Task;
  owners: Array<{
    id: string;
    email: string;
    full_name: string;
    avatar_url?: string;
    created_at: string;
    updated_at: string;
  }>;
  followers: Array<{
    id: string;
    email: string;
    full_name: string;
    avatar_url?: string;
    created_at: string;
    updated_at: string;
  }>;
  owner_member?: AssignableMember;
  requester_member?: AssignableMember;
  labels: Label[];
  epic_name?: string;
  sprint_name?: string;
  objective_name?: string;
  objective_id?: string;
  state?: WorkflowState;
}

export type StoryDetail = TaskDetail;

export interface TaskStateColumn {
  state: WorkflowState;
  stories: Task[];
  story_groups?: TaskGroup[];
  story_count: number;
  point_total: number;
  has_more: boolean;
}

export type StoryStateColumn = TaskStateColumn;

export interface TaskGroup {
  key: string;
  label: string;
  stories: Task[];
}

export type StoryGroup = TaskGroup;

export interface ColumnTasksResponse {
  stories: Task[];
  story_groups?: TaskGroup[];
  total: number;
}

export type ColumnStoriesResponse = ColumnTasksResponse;

export interface TaskMemberColumn {
  member: AssignableMember | null;
  stories: Task[];
  story_count: number;
  point_total: number;
  has_more: boolean;
}

export type StoryMemberColumn = TaskMemberColumn;

export interface StoryStateCount {
  state_id: string;
  state_name: string;
  state_type: StateType;
  story_count: number;
}

export type TaskStateCount = StoryStateCount;

export interface Comment {
  id: string;
  entity_type: 'task' | 'epic' | 'doc';
  entity_id: string;
  author_id: string;
  body: string;
  parent_id?: string;
  created_at: string;
  updated_at: string;
}

export interface ReactionSummary {
  emoji: string;
  count: number;
  user_ids: string[];
}

export interface CommentWithAuthor {
  comment: Comment;
  author: {
    id: string;
    email: string;
    full_name: string;
    avatar_url?: string;
    created_at: string;
    updated_at: string;
  };
  reply_count: number;
  replies?: CommentWithAuthor[];
  reactions?: ReactionSummary[];
  attachments?: AttachmentResponse[];
}

export interface PMActivity {
  id: string;
  workspace_id: string;
  entity_type: string;
  entity_id: string;
  actor_id?: string;
  action: string;
  field_name?: string;
  old_value?: string;
  new_value?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface ActivityLogEntry {
  activity: PMActivity;
  actor?: {
    id: string;
    email: string;
    full_name: string;
    avatar_url?: string;
    created_at: string;
    updated_at: string;
  };
}

export interface PaginatedResponse<T> {
  data: T;
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

export interface CreateWorkflowRequest {
  workspace_id: string;
  name: string;
  description?: string;
  team_id?: string;
  auto_assign_owner?: boolean;
}

export interface UpdateWorkflowRequest {
  name?: string;
  description?: string;
  team_id?: string;
  default_state_id?: string;
  auto_assign_owner?: boolean;
}

export interface CreateWorkflowStateRequest {
  name: string;
  state_type: StateType;
  position?: number;
  color?: string;
  description?: string;
  wip_limit?: number;
  is_default?: boolean;
}

export interface UpdateWorkflowStateRequest {
  name?: string;
  state_type?: StateType;
  position?: number;
  color?: string;
  description?: string;
  wip_limit?: number;
  is_default?: boolean;
}

export interface LabelStats {
  story_count: number;
  done_story_count: number;
  total_points: number;
  done_points: number;
  epic_count: number;
  done_epic_count: number;
}

export interface LabelWithStats {
  label: Label;
  stats: LabelStats;
}

export interface CreateLabelRequest {
  workspace_id: string;
  team_id?: string;
  name: string;
  description?: string;
  color?: string;
}

export interface UpdateLabelRequest {
  team_id?: string;
  name?: string;
  description?: string;
  color?: string;
  archived?: boolean;
}

// ── Story Templates ─────────────────────────────────────────────────

export interface TemplateChecklistItem {
  text: string;
  position?: number;
}

export interface TaskTemplate {
  id: string;
  workspace_id: string;
  team_id?: string;
  name: string;
  description?: string;
  task_type?: TaskType;
  priority?: Priority;
  severity?: Severity;
  estimate?: number;
  label_ids?: string;
  owner_member_id?: string;
  epic_id?: string;
  sprint_id?: string;
  deadline?: string;
  checklist_items?: string;
  external_links?: string;
  archived: boolean;
  created_at: string;
  updated_at: string;
}

/** @deprecated Use TaskTemplate */
export type StoryTemplate = TaskTemplate;

export interface RecurringTemplateConfig {
  schedule_type: RecurringScheduleType;
  frequency?: RecurringFrequency;
  interval?: number;
  weekdays?: number[];
  day_of_month?: number;
  completion_event?: RecurringCompletionEvent;
  completion_state_ids?: string[];
  due_date_mode?: RecurringDueDateMode;
  due_offset_days?: number;
  starts_on?: string;
  ends_on?: string;
  ends_after_occurrences?: number;
  sprint_assignment_mode?: RecurringSprintAssignmentMode;
}

export interface RecurringStorySeed {
  name: string;
  description?: string;
  task_type?: TaskType;
  workflow_id: string;
  workflow_state_id: string;
  epic_id?: string;
  team_id?: string;
  owner_member_id?: string;
  requester_member_id?: string;
  estimate?: number;
  priority?: Priority;
  severity?: Severity;
  owner_ids?: string[];
  follower_ids?: string[];
  label_ids?: string[];
  checklist_items?: CreateChecklistItemRequest[];
  external_links?: CreateExternalLinkRequest[];
}

export type RecurringTaskSeed = RecurringStorySeed;

export interface RecurringTemplate {
  id: string;
  workspace_id: string;
  team_id?: string;
  title: string;
  description?: string;
  status: RecurringTemplateStatus;
  owner_member_id?: string;
  created_from_story_id?: string;
  seed_payload: string;
  config: string;
  start_date?: string;
  end_date?: string;
  ends_after_occurrences?: number;
  next_run_at?: string;
  last_run_at?: string;
  last_generated_story_id?: string;
  last_error?: string;
  failure_count: number;
  generated_count: number;
  skip_next_run: boolean;
  created_by_id?: string;
  updated_by_id?: string;
  created_at: string;
  updated_at: string;
}

export interface RecurringTaskTemplate extends Omit<RecurringTemplate, 'created_from_story_id' | 'last_generated_story_id'> {
  created_from_task_id?: string;
  last_generated_task_id?: string;
}

export interface RecurringRun {
  id: string;
  workspace_id: string;
  template_id: string;
  occurrence_number: number;
  trigger_type: RecurringRunTrigger;
  scheduled_for?: string;
  started_at?: string;
  finished_at?: string;
  status: RecurringRunStatus;
  generated_story_id?: string;
  dedupe_key: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface RecurringTaskRun extends Omit<RecurringRun, 'generated_story_id'> {
  generated_task_id?: string;
}

export interface RecurringTemplateDetail {
  template: RecurringTemplate;
  config: RecurringTemplateConfig;
  seed: RecurringStorySeed;
  rule_summary: string;
  last_generated_story?: Story;
  runs?: RecurringRun[];
}

export interface RecurringTaskTemplateDetail extends Omit<RecurringTemplateDetail, 'seed' | 'last_generated_story'> {
  seed: RecurringTaskSeed;
  last_generated_task?: Task;
}

export interface StoryRecurringSummary {
  template_id: string;
  template_title: string;
  status: RecurringTemplateStatus;
  occurrence_number: number;
  generated_count: number;
  rule_summary: string;
  next_run_at?: string;
  last_error?: string;
  last_generated_story?: Story;
  config: RecurringTemplateConfig;
}

export interface TaskRecurringSummary extends Omit<StoryRecurringSummary, 'last_generated_story'> {
  last_generated_task?: Task;
}

export interface CreateRecurringTemplateRequest {
  workspace_id: string;
  title: string;
  description?: string;
  task_id: string;
  /** @deprecated Use task_id */
  story_id?: string;
  config: RecurringTemplateConfig;
}

export interface CreateRecurringTaskTemplateRequest extends Omit<CreateRecurringTemplateRequest, 'story_id'> {}

export interface UpdateRecurringTemplateRequest {
  title?: string;
  description?: string;
  task_id?: string;
  /** @deprecated Use task_id */
  story_id?: string;
  config?: RecurringTemplateConfig;
}

export interface UpdateRecurringTaskTemplateRequest extends Omit<UpdateRecurringTemplateRequest, 'story_id'> {}

export interface CreateTaskTemplateRequest {
  workspace_id: string;
  team_id?: string;
  name: string;
  description?: string;
  task_type?: TaskType;
  priority?: Priority;
  severity?: Severity;
  estimate?: number;
  label_ids?: string;
  owner_member_id?: string;
  epic_id?: string;
  sprint_id?: string;
  deadline?: string;
  checklist_items?: string;
  external_links?: string;
}

export type CreateStoryTemplateRequest = CreateTaskTemplateRequest;

export interface UpdateTaskTemplateRequest {
  team_id?: string;
  name?: string;
  description?: string;
  task_type?: TaskType;
  priority?: Priority;
  severity?: Severity;
  estimate?: number;
  label_ids?: string;
  owner_member_id?: string;
  epic_id?: string;
  sprint_id?: string;
  deadline?: string;
  checklist_items?: string;
  external_links?: string;
  archived?: boolean;
}

export type UpdateStoryTemplateRequest = UpdateTaskTemplateRequest;

export interface CreateEpicRequest {
  workspace_id: string;
  name: string;
  description?: string;
  attachment_ids?: string[];
  epic_state_id?: string;
  owner_member_id?: string;
  team_id?: string;
  planned_start_date?: string;
  deadline?: string;
  position?: number;
  color?: string;
  health?: EpicHealth;
  health_comment?: string;
  label_ids?: string[];
  planning_repository_id?: string;
}

export interface UpdateEpicRequest {
  name?: string;
  description?: string;
  epic_state_id?: string;
  owner_member_id?: string;
  team_id?: string;
  planned_start_date?: string;
  deadline?: string;
  position?: number;
  color?: string;
  archived?: boolean;
  health?: EpicHealth;
  health_comment?: string;
  label_ids?: string[];
  planning_repository_id?: string;
}

export interface UpdateEpicHealthRequest {
  health: EpicHealth;
  comment?: string;
}

export interface CreateSprintRequest {
  workspace_id: string;
  name: string;
  description?: string;
  attachment_ids?: string[];
  start_date: string;
  end_date: string;
  team_id?: string;
  label_ids?: string[];
}

export interface UpdateSprintRequest {
  name?: string;
  description?: string;
  start_date?: string;
  end_date?: string;
  team_id?: string;
  archived?: boolean;
  label_ids?: string[];
}

export interface SprintPlanningFilters {
  team_id?: string;
  include_completed?: boolean;
}

export interface CreateTaskRequest {
  workspace_id: string;
  name: string;
  description?: string;
  attachment_ids?: string[];
  task_type?: TaskType;
  workflow_id?: string;
  workflow_state_id?: string;
  epic_id?: string;
  sprint_id?: string;
  team_id?: string;
  owner_member_id?: string;
  requester_member_id?: string;
  estimate?: number;
  priority?: Priority;
  severity?: Severity;
  deadline?: string;
  position?: number;
  blocked?: boolean;
  blocker?: string;
  template_id?: string;
  external_id?: string;
  owner_ids?: string[];
  follower_ids?: string[];
  label_ids?: string[];
  checklist_items?: { text: string; position?: number }[];
  external_links?: { url: string; title?: string }[];
}

export type CreateStoryRequest = CreateTaskRequest;

export interface UpdateTaskRequest {
  name?: string;
  description?: string;
  task_type?: TaskType;
  workflow_id?: string;
  workflow_state_id?: string;
  epic_id?: string;
  sprint_id?: string;
  team_id?: string;
  owner_member_id?: string;
  requester_member_id?: string;
  estimate?: number;
  priority?: Priority;
  severity?: Severity;
  deadline?: string;
  position?: number;
  blocked?: boolean;
  blocker?: string;
  archived?: boolean;
  template_id?: string;
  external_id?: string;
  owner_ids?: string[];
  follower_ids?: string[];
  label_ids?: string[];
}

export type UpdateStoryRequest = UpdateTaskRequest;

export interface MoveTaskRequest {
  state_id: string;
  position?: number;
  debug_trace_id?: string;
}

export type MoveStoryRequest = MoveTaskRequest;

export interface ReorderTaskRequest {
  position: number;
  debug_trace_id?: string;
}

export type ReorderStoryRequest = ReorderTaskRequest;

export interface TaskUserLinkRequest {
  user_id?: string;
  workspace_member_id?: string;
}

export type StoryUserLinkRequest = TaskUserLinkRequest;

export interface TaskLabelLinkRequest {
  label_id: string;
}

export type StoryLabelLinkRequest = TaskLabelLinkRequest;

// ── Checklist Items ─────────────────────────────────────────────────

export interface ChecklistItem {
  id: string;
  task_id: string;
  /** @deprecated Use task_id */
  story_id?: string;
  text: string;
  completed: boolean;
  position: number;
  assignee_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateChecklistItemRequest {
  text: string;
  position?: number;
  assignee_id?: string;
}

export interface UpdateChecklistItemRequest {
  text?: string;
  completed?: boolean;
  position?: number;
  assignee_id?: string;
}

// ── External Links ──────────────────────────────────────────────────

export interface ExternalLink {
  id: string;
  task_id: string;
  /** @deprecated Use task_id */
  story_id?: string;
  title: string;
  url: string;
  created_by_id: string;
  created_at: string;
  updated_at: string;
}

export interface CreateExternalLinkRequest {
  url: string;
}

export interface UpdateExternalLinkRequest {
  url?: string;
  title?: string;
}

// ── Attachments ─────────────────────────────────────────────────────

export interface Attachment {
  id: string;
  workspace_id: string;
  entity_type: 'task' | 'epic' | 'objective' | 'sprint' | 'comment' | 'editor_upload';
  entity_id: string;
  file_name: string;
  file_size: number;
  content_type: string;
  storage_key: string;
  is_uploaded: boolean;
  uploaded_by_id: string;
  created_at: string;
}

export interface AttachmentResponse {
  attachment: Attachment;
  url: string;
  public_url?: string;
}

export interface CreateAttachmentRequest {
  entity_type: 'task' | 'epic' | 'objective' | 'sprint' | 'comment' | 'editor_upload';
  entity_id: string;
  file_name: string;
  file_size: number;
  content_type: string;
}

export interface CreateCommentRequest {
  entity_type: 'task' | 'epic' | 'doc';
  entity_id: string;
  body: string;
  parent_id?: string;
  attachment_ids?: string[];
}

export interface UpdateCommentRequest {
  body: string;
}

// Task is now the canonical type, Story is an alias (defined above)
