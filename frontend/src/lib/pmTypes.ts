import type { AssignableMember } from './types';

export type StoryType = 'feature' | 'bug' | 'chore';
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

export interface Story {
  id: string;
  workspace_id: string;
  display_id: number;
  name: string;
  description?: string;
  story_type: StoryType;
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
  is_blocked_by_story?: boolean;
  blocked_by_count?: number;
  is_blocking_other_story?: boolean;
  blocking_count?: number;
  blocked_by_stories?: StoryDependencyStory[];
  blocking_stories?: StoryDependencyStory[];
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
}

export interface StoryDependencyStory {
  id: string;
  display_id: number;
  name: string;
  workflow_state_id: string;
  completed: boolean;
}

export type AssociationEntityType =
  | 'story'
  | 'epic'
  | 'support_conversation'
  | 'contact'
  | 'company'
  | 'deal'
  | 'document';

export type StoryRelationshipAction =
  | 'relates_to'
  | 'blocks'
  | 'is_blocked_by'
  | 'duplicates'
  | 'is_duplicated_by';

export interface AssociationObjectSummary {
  association_id?: string;
  object_type: AssociationEntityType | string;
  object_id: string;
  display_id?: string;
  title: string;
  status?: string;
  workflow_state_id?: string;
  completed?: boolean;
  story_type?: StoryType;
}

export interface StoryRelationshipSummary {
  relationship_id: string;
  link_type: string;
  is_active: boolean;
  story: AssociationObjectSummary;
}

export interface StoryRelationshipGroups {
  blocked_by: StoryRelationshipSummary[];
  blocking: StoryRelationshipSummary[];
  relates_to: StoryRelationshipSummary[];
  related_by: StoryRelationshipSummary[];
  duplicates: StoryRelationshipSummary[];
  duplicated_by: StoryRelationshipSummary[];
}

export interface GroupedAssociations {
  story_relationships: StoryRelationshipGroups;
  stories: AssociationObjectSummary[];
  support_conversations: AssociationObjectSummary[];
  crm_records: AssociationObjectSummary[];
  docs: AssociationObjectSummary[];
}

export interface CreateStoryRelationshipRequest {
  relationship_type: StoryRelationshipAction;
  other_story_id: string;
}

export interface StoryDetail {
  story: Story;
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

export interface StoryStateColumn {
  state: WorkflowState;
  stories: Story[];
  story_groups?: StoryGroup[];
  story_count: number;
  point_total: number;
  has_more: boolean;
}

export interface StoryGroup {
  key: string;
  label: string;
  stories: Story[];
}

export interface ColumnStoriesResponse {
  stories: Story[];
  story_groups?: StoryGroup[];
  total: number;
}

export interface StoryMemberColumn {
  member: AssignableMember | null;
  stories: Story[];
  story_count: number;
  point_total: number;
  has_more: boolean;
}

export interface StoryStateCount {
  state_id: string;
  state_name: string;
  state_type: StateType;
  story_count: number;
}

export interface Comment {
  id: string;
  entity_type: 'story' | 'epic' | 'doc';
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

export interface StoryTemplate {
  id: string;
  workspace_id: string;
  team_id?: string;
  name: string;
  description?: string;
  story_type?: StoryType;
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
  story_type?: StoryType;
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

export interface RecurringTemplateDetail {
  template: RecurringTemplate;
  config: RecurringTemplateConfig;
  seed: RecurringStorySeed;
  rule_summary: string;
  last_generated_story?: Story;
  runs?: RecurringRun[];
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

export interface CreateRecurringTemplateRequest {
  workspace_id: string;
  title: string;
  description?: string;
  story_id: string;
  config: RecurringTemplateConfig;
}

export interface UpdateRecurringTemplateRequest {
  title?: string;
  description?: string;
  story_id?: string;
  config?: RecurringTemplateConfig;
}

export interface CreateStoryTemplateRequest {
  workspace_id: string;
  team_id?: string;
  name: string;
  description?: string;
  story_type?: StoryType;
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

export interface UpdateStoryTemplateRequest {
  team_id?: string;
  name?: string;
  description?: string;
  story_type?: StoryType;
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

export interface CreateStoryRequest {
  workspace_id: string;
  name: string;
  description?: string;
  attachment_ids?: string[];
  story_type?: StoryType;
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

export interface UpdateStoryRequest {
  name?: string;
  description?: string;
  story_type?: StoryType;
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

export interface MoveStoryRequest {
  state_id: string;
  position?: number;
}

export interface ReorderStoryRequest {
  position: number;
}

export interface StoryUserLinkRequest {
  user_id?: string;
  workspace_member_id?: string;
}

export interface StoryLabelLinkRequest {
  label_id: string;
}

// ── Checklist Items ─────────────────────────────────────────────────

export interface ChecklistItem {
  id: string;
  story_id: string;
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
  story_id: string;
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
  entity_type: 'story' | 'epic' | 'objective' | 'sprint' | 'comment' | 'editor_upload';
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
  entity_type: 'story' | 'epic' | 'objective' | 'sprint' | 'comment' | 'editor_upload';
  entity_id: string;
  file_name: string;
  file_size: number;
  content_type: string;
}

export interface CreateCommentRequest {
  entity_type: 'story' | 'epic' | 'doc';
  entity_id: string;
  body: string;
  parent_id?: string;
  attachment_ids?: string[];
}

export interface UpdateCommentRequest {
  body: string;
}

// ── Objectives ──────────────────────────────────────────────────────

export type ObjectiveType = 'tactical' | 'strategic';
export type ObjectiveState = 'not_started' | 'active' | 'closed';
export type ObjectiveHealth = 'on_track' | 'at_risk' | 'off_track';
export type KeyResultType = 'boolean' | 'percent' | 'numeric';

export interface Objective {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  objective_type: ObjectiveType;
  state: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  health: ObjectiveHealth;
  health_comment?: string;
  position: number;
  archived: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface KeyResult {
  id: string;
  objective_id: string;
  name: string;
  result_type: KeyResultType;
  initial_value: number;
  current_value: number;
  target_value: number;
  progress: number;
  note?: string;
  note_updated_by?: string;
  note_updated_at?: string;
  position: number;
  updated_by?: string;
  created_at: string;
  updated_at: string;
}

export interface ObjectiveStats {
  key_result_count: number;
  key_result_avg_pct: number;
  epic_count: number;
  epic_done_count: number;
  epic_story_count: number;
  epic_done_stories: number;
  epic_progress_pct: number;
}

export interface ObjectiveWithDetails {
  objective: Objective;
  teams: string[];
  owners: string[];
  owner_member_ids?: string[];
  labels: Label[];
  key_results: KeyResult[];
  epics: EpicWithStats[];
  stats: ObjectiveStats;
  suggested_health: ObjectiveHealth;
}

export interface CreateObjectiveRequest {
  workspace_id: string;
  name: string;
  description?: string;
  attachment_ids?: string[];
  objective_type: ObjectiveType;
  state?: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  health?: ObjectiveHealth;
  health_comment?: string;
  position?: number;
  team_ids?: string[];
  owner_ids?: string[];
  owner_member_ids?: string[];
  label_ids?: string[];
  epic_ids?: string[];
}

export interface UpdateObjectiveRequest {
  name?: string;
  description?: string;
  objective_type?: ObjectiveType;
  state?: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  health?: ObjectiveHealth;
  health_comment?: string;
  position?: number;
  archived?: boolean;
  team_ids?: string[];
  owner_ids?: string[];
  owner_member_ids?: string[];
  label_ids?: string[];
  epic_ids?: string[];
}

export interface CreateKeyResultRequest {
  name: string;
  result_type?: KeyResultType;
  initial_value?: number;
  current_value?: number;
  target_value?: number;
  note?: string;
  position?: number;
}

export interface UpdateKeyResultRequest {
  name?: string;
  result_type?: KeyResultType;
  initial_value?: number;
  current_value?: number;
  target_value?: number;
  note?: string;
  position?: number;
}

// ── Automations ─────────────────────────────────────────────────────

export type AutomationType = 'epic_auto_start' | 'epic_auto_complete' | 'sprint_auto_create' | 'sprint_move_unfinished';

export interface PMAutomation {
  id: string;
  workspace_id: string;
  automation_type: AutomationType;
  enabled: boolean;
  team_id?: string;
  config_state_id?: string;
  config_int?: number;
  config_int2?: number;
  config_int3?: number;
  created_at: string;
  updated_at: string;
}

export interface UpsertAutomationRequest {
  workspace_id: string;
  automation_type: AutomationType;
  enabled: boolean;
  team_id?: string;
  config_state_id?: string;
  config_int?: number;
  config_int2?: number;
  config_int3?: number;
}

export interface DeleteAutomationRequest {
  workspace_id: string;
  automation_type: AutomationType;
  team_id?: string;
}

// ── Automation Rules ────────────────────────────────────────────────

export interface AutomationRule {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  enabled: boolean;
  team_id?: string;
  workflow_id?: string;
  trigger_type: string;
  trigger_config: Record<string, string>;
  action_type: string;
  action_config: Record<string, unknown>;
  position: number;
  stop_on_match: boolean;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateAutomationRuleRequest {
  workspace_id: string;
  name: string;
  description?: string;
  team_id?: string;
  workflow_id?: string;
  trigger_type: string;
  trigger_config: Record<string, string>;
  action_type: string;
  action_config: Record<string, unknown>;
  position?: number;
  stop_on_match?: boolean;
}

export interface UpdateAutomationRuleRequest {
  name?: string;
  description?: string;
  enabled?: boolean;
  trigger_type?: string;
  trigger_config?: Record<string, string>;
  action_type?: string;
  action_config?: Record<string, unknown>;
  position?: number;
  stop_on_match?: boolean;
}

// ── Views (Spaces) ──────────────────────────────────────────────────

export interface PMView {
  id: string;
  workspace_id: string;
  name: string;
  filters: Record<string, string>;
  is_shared: boolean;
  is_pinned: boolean;
  position: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateViewRequest {
  name: string;
  filters: Record<string, string>;
  is_shared: boolean;
  is_pinned: boolean;
}

export interface UpdateViewRequest {
  name?: string;
  filters?: Record<string, string>;
  is_shared?: boolean;
  is_pinned?: boolean;
  position?: number;
}

// ── Agents ──────────────────────────────────────────────────────────

export type AgentPresetKey =
  | 'epic_planner'
  | 'story_planner'
  | 'crm_operator'
  | 'support_agent'
  | 'code_builder'
  | 'review_agent';
export type AgentStatus = 'idle' | 'working' | 'error' | 'paused';
export type AgentRunStatus = 'queued' | 'running' | 'awaiting_input' | 'awaiting_approval' | 'completed' | 'failed' | 'cancelled';
export type AgentRuntimeKind = 'opencode' | 'native_sdk';
export type AgentTriggerMode = 'manual' | 'auto_on_assignment' | 'auto_on_event';
export type AgentTargetType = 'story' | 'support_conversation' | 'epic' | 'document' | 'crm_deal';
export type AgentApprovalState = 'not_required' | 'pending' | 'approved' | 'rejected';
export type AgentApprovalMode = 'preset_default' | 'never' | 'always';
export type AgentModelProvider = 'anthropic' | 'openai' | 'openrouter';
export type AgentInvocationMode = 'interactive' | 'autonomous';

export interface Agent {
  id: string;
  workspace_id: string;
  is_system: boolean;
  name: string;
  preset_key?: AgentPresetKey;
  role: string;
  status: AgentStatus;
  runtime_kind: AgentRuntimeKind;
  skills: string[];
  trigger_mode: AgentTriggerMode;
  provider?: AgentModelProvider;
  model?: string;
  system_prompt?: string;
  planning_notes?: string;
  tools: unknown[];
  monthly_token_budget?: number;
  tokens_used_this_month: number;
  active_story_id?: string;
  team_id?: string;
  allowed_tools: string[];
  allowed_commands: string[];
  allowed_targets: string[];
  schedule?: string;
  target_selector?: Record<string, unknown>;
  trigger_events: string[];
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: number;
  default_invocation_mode: AgentInvocationMode;
  supported_modes?: Array<'interactive' | 'autonomous'>;
  created_at: string;
  updated_at: string;
}

export interface AgentRun {
  id: string;
  workspace_id: string;
  agent_id: string;
  story_id?: string;
  conversation_id?: string;
  target_type: AgentTargetType;
  target_id: string;
  runtime_kind: AgentRuntimeKind;
  invocation_mode: AgentInvocationMode;
  parent_run_id?: string;
  handoff_state?: string;
  approval_state: AgentApprovalState;
  triggered_by_user_id?: string;
  status: AgentRunStatus;
  workflow_id?: string;
  workflow_run_id?: string;
  task_queue?: string;
  runner_pool?: string;
  repository_id?: string;
  repo_full_name?: string;
  base_branch?: string;
  working_branch?: string;
  delivery_target_id?: string;
  execution_stage?: string;
  last_heartbeat_at?: string;
  input: Record<string, unknown>;
  output_summary: Record<string, unknown>;
  tokens_used: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AgentRunMessage {
  id: string;
  workspace_id: string;
  run_id: string;
  role: string;
  content: string;
  message_type: string;
  content_blocks?: Array<Record<string, unknown>>;
  tool_invocations?: Array<Record<string, unknown>>;
  token_usage?: Record<string, unknown>;
  sequence_no: number;
  created_at: string;
}

export interface AgentRunStreamEvent {
  event_id?: string;
  sent_at?: string;
  type: string;
  run_id: string;
  message_id?: string;
  text?: string;
  tool_call_id?: string;
  tool_name?: string;
  tool_input?: string;
  output_summary?: string;
  duration_ms?: number;
  error?: string;
}

export interface StartAgentRunRequest {
  agent_id?: string;
  additional_context?: string;
}

export interface SendAgentRunMessageRequest {
  content: string;
}

export interface SendAgentRunRequestChangesRequest {
  content: string;
}

export interface StoryImplementationBrief {
  approach: string;
  files_to_modify: FileChange[];
  test_strategy: string;
  vertical_layers?: string[];
  depends_on_files?: string[];
}

export interface FileChange {
  path: string;
  action: 'create' | 'modify' | 'delete';
  description: string;
}

export interface VerticalCoverageEntry {
  behavior: string;
  story_refs: string[];
  full_slice: boolean;
}

export interface ProposedStory {
  ref?: string;
  name: string;
  description: string;
  story_type: string;
  estimate?: number;
  priority?: string;
  acceptance_criteria?: string[];
  dependency_refs?: string[];
  source_refs?: PlanningSourceRef[];
  assign_agent_id?: string;
  slice_type?: 'vertical' | 'enabler' | 'spike';
  implementation_brief?: StoryImplementationBrief;
}

export interface PlanningSourceRef {
  type: string;
  id?: string;
  title?: string;
}

export interface OrchestrationProposal {
  epic_id: string;
  summary: string;
  spec_version_id?: string;
  proposed_stories: ProposedStory[];
  open_questions?: string[];
  risks?: string[];
  vertical_coverage?: VerticalCoverageEntry[];
  tokens_used: number;
}

export interface ApprovedSpecSummary {
  stage: string;
  spec_document_id: string;
  spec_version_id?: string;
  summary?: string;
  clarifications?: SpecClarification[];
  pending_clarify_count?: number;
}

export interface AgentRunArtifact {
  id: string;
  workspace_id: string;
  run_id: string;
  artifact_type: string;
  format: string;
  storage_mode: string;
  inline_content?: string;
  object_key?: string;
  metadata: Record<string, unknown>;
  sequence_no: number;
  created_at: string;
}

export interface CreateAgentRequest {
  workspace_id: string;
  name: string;
  preset_key?: AgentPresetKey;
  role?: string;
  runtime_kind?: AgentRuntimeKind;
  skills?: string[];
  trigger_mode?: AgentTriggerMode;
  provider?: AgentModelProvider;
  model?: string;
  system_prompt?: string;
  planning_notes?: string;
  tools?: unknown[];
  monthly_token_budget?: number;
  team_id?: string | null;
  allowed_tools?: string[];
  allowed_commands?: string[];
  allowed_targets?: string[];
  schedule?: string;
  target_selector?: Record<string, unknown>;
  trigger_events?: string[];
  approval_mode?: AgentApprovalMode;
  max_concurrent_runs?: number;
  default_invocation_mode?: AgentInvocationMode;
}

export interface UpdateAgentRequest {
  name?: string;
  preset_key?: AgentPresetKey;
  role?: string;
  status?: AgentStatus;
  runtime_kind?: AgentRuntimeKind;
  skills?: string[];
  trigger_mode?: AgentTriggerMode;
  provider?: AgentModelProvider;
  model?: string;
  system_prompt?: string;
  planning_notes?: string;
  tools?: unknown[];
  monthly_token_budget?: number;
  active_story_id?: string;
  team_id?: string | null;
  allowed_tools?: string[];
  allowed_commands?: string[];
  allowed_targets?: string[];
  schedule?: string;
  target_selector?: Record<string, unknown>;
  trigger_events?: string[];
  approval_mode?: AgentApprovalMode;
  max_concurrent_runs?: number;
  default_invocation_mode?: AgentInvocationMode;
}

export interface AssignAgentRequest {
  agent_id: string;
}

export interface ApproveAgentRunRequest {
  send_message?: boolean;
}

export interface HandoffAgentRunRequest {
  to_agent_id?: string;
  to_user_id?: string;
  handoff_state?: string;
  reason: string;
  context?: Record<string, unknown>;
}

export interface AgentPresetDefinition {
  key: AgentPresetKey;
  label: string;
  description: string;
  default_role: string;
  runtime_kind: AgentRuntimeKind;
  default_trigger_mode: AgentTriggerMode;
  allowed_trigger_modes: AgentTriggerMode[];
  allowed_tools: string[];
  allowed_commands: string[];
  allowed_target_types: AgentTargetType[];
  approval_mode: AgentApprovalMode;
  default_invocation_mode: AgentInvocationMode;
  supported_modes: AgentInvocationMode[];
  system_prompt?: string;
}

export interface AgentModelProviderOption {
  value: AgentModelProvider;
  label: string;
  model_placeholder: string;
}

export interface RunnerHealth {
  namespace?: string;
  temporal_configured: boolean;
  generated_at?: string;
  queues: RunnerQueueHealth[];
  active_runs: RunnerActiveRun[];
}

export interface RunnerQueueHealth {
  name: string;
  concurrency: number;
  queued_runs: number;
  running_runs: number;
  awaiting_approval_runs: number;
  active_runs: number;
  latest_heartbeat_at?: string;
}

export interface RunnerActiveRun {
  id: string;
  agent_id: string;
  target_type: string;
  target_id: string;
  status: AgentRunStatus;
  task_queue: string;
  runner_pool: string;
  execution_stage?: string;
  last_heartbeat_at?: string;
  started_at?: string;
  created_at: string;
  workflow_id?: string;
  stale: boolean;
}

// ── Support ─────────────────────────────────────────────────────────

export type ConversationStatus = 'open' | 'in_progress' | 'waiting' | 'resolved' | 'closed' | 'spam';
export type ConversationPriority = 'low' | 'medium' | 'high' | 'urgent';
export type TicketSource = 'widget' | 'internal' | 'email' | 'api';
export type MessageSenderType = 'customer' | 'user' | 'agent' | 'ai';

export interface SupportConversation {
  id: string;
  workspace_id: string;
  display_id: number;
  subject: string;
  status: ConversationStatus;
  priority: ConversationPriority;
  customer_name?: string;
  customer_email?: string;
  anonymous_id?: string;
  opened_by_user_id?: string;
  assigned_agent_id?: string;
  linked_story_id?: string;
  source: TicketSource;
  crm_contact_id?: string;
  ai_state?: 'pending' | 'resolved' | 'escalated' | null;
  ai_resolved_at?: string;
  ai_escalated_at?: string;
  ai_resolution_type?: 'confirmed' | 'assumed' | null;
  ai_turn_count?: number;
  customer_requested_human_at?: string;
  last_message?: string;
  unread_count?: number;
  team_last_seen_at?: string;
  contact_last_seen_at?: string;
  created_at: string;
  updated_at: string;
}

export interface UnreadStats {
  total: number;
  my_inbox: number;
  unassigned: number;
}

export interface ConversationListMeta {
  unread: UnreadStats;
}

export interface ConversationListResponse {
  data: SupportConversation[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
  meta: ConversationListMeta;
}

export interface SupportMessage {
  id: string;
  workspace_id: string;
  conversation_id: string;
  sender_type: MessageSenderType;
  sender_user_id?: string;
  sender_agent_id?: string;
  sender_display_name?: string;
  sender_avatar_url?: string;
  content: string;
  message_type?: string;
  is_internal: boolean;
  metadata?: string;
  via_channel?: 'email' | 'widget' | null;
  email_notified_at?: string;
  created_at: string;
  updated_at: string;
}

export interface AIMessageMetadata {
  ai_auto_reply: boolean;
  ai_sources: Array<{
    docId: string;
    title: string;
    snippet: string;
    confidence: number;
  }>;
  ai_confidence: number;
  ai_model: string;
  ai_tokens_used: number;
  ai_agent_id: string;
}

export interface AgentKnowledgeSource {
  id: string;
  agent_id: string;
  space_id: string;
  workspace_id: string;
  sync_status: 'queued' | 'running' | 'ready' | 'failed' | 'stale' | 'disabled';
  sync_progress: number;
  indexed_documents: number;
  indexed_chunks: number;
  last_sync_error?: string | null;
  last_sync_started_at?: string | null;
  last_sync_completed_at?: string | null;
  space_name?: string;
  space_type?: string;
  created_at: string;
  updated_at: string;
}

export interface SupportContentSource {
  id: string;
  workspace_id: string;
  name: string;
  start_url: string;
  crawl_limit: number;
  crawl_depth: number;
  crawl_source: 'all' | 'sitemaps' | 'links';
  formats: string[];
  render: boolean;
  include_external_links: boolean;
  include_subdomains: boolean;
  include_patterns: string[];
  exclude_patterns: string[];
  crawl_purposes: string[];
  max_age_seconds: number;
  modified_since?: string | null;
  json_prompt?: string | null;
  json_response_format?: unknown;
  sync_status: 'queued' | 'running' | 'ready' | 'failed' | 'stale' | 'disabled';
  sync_progress: number;
  indexed_pages: number;
  indexed_chunks: number;
  last_sync_error?: string | null;
  last_crawl_job_id?: string | null;
  last_sync_started_at?: string | null;
  last_sync_completed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface SupportContentPage {
  id: string;
  workspace_id: string;
  content_source_id: string;
  url: string;
  title: string;
  http_status: number;
  content_format: string;
  content_hash: string;
  last_crawled_at: string;
  created_at: string;
  updated_at: string;
}

export interface CreateSupportContentSourceRequest {
  name: string;
  start_url: string;
  crawl_limit: number;
  crawl_depth: number;
  crawl_source: 'all' | 'sitemaps' | 'links';
  formats: string[];
  render: boolean;
  include_external_links: boolean;
  include_subdomains: boolean;
  include_patterns: string[];
  exclude_patterns: string[];
  crawl_purposes: string[];
  max_age_seconds: number;
  modified_since?: string | null;
  json_prompt?: string | null;
  json_response_format?: unknown;
}

export interface UpdateSupportContentSourceRequest {
  name?: string;
  start_url?: string;
  crawl_limit?: number;
  crawl_depth?: number;
  crawl_source?: 'all' | 'sitemaps' | 'links';
  formats?: string[];
  render?: boolean;
  include_external_links?: boolean;
  include_subdomains?: boolean;
  include_patterns?: string[];
  exclude_patterns?: string[];
  crawl_purposes?: string[];
  max_age_seconds?: number;
  modified_since?: string | null;
  json_prompt?: string | null;
  json_response_format?: unknown;
}

export interface CreateConversationRequest {
  subject: string;
  priority?: ConversationPriority;
  customer_name?: string;
  customer_email?: string;
  source?: TicketSource;
}

export interface CreateMessageRequest {
  content: string;
  is_internal?: boolean;
}

export interface LinkStoryRequest {
  story_id: string;
}

export interface AssignConversationAgentRequest {
  agent_id: string;
}

// ── Support Installation Settings ───────────────────────────────────

export interface BusinessHoursDay {
  start: string;
  end: string;
  enabled: boolean;
}

export interface SupportInboxSettings {
  require_email_before_chat: boolean;
  require_phone_after_email: boolean;
  welcome_message: string;
  ai_enabled: boolean;
  ai_agent_id: string | null;
  ai_confidence_threshold: number;
  ai_response_mode: string;
  ai_max_followups: number;
  ai_auto_resolve_timeout: number;
  show_talk_to_human: boolean;
  handoff_behavior: string;
  handoff_team_id: string | null;
  business_hours_enabled: boolean;
  business_hours_timezone: string;
  business_hours_schedule: Record<string, BusinessHoursDay>;
  outside_hours_message: string;
  email_fallback_enabled: boolean;
  email_fallback_delay_secs: number;
  email_fallback_from_name: string;
  brand_color: string;
  show_branding: boolean;
  color_scheme: string;
  button_color: string;
  button_icon_color: string;
  logo_url: string;
  launcher_position: string;
  launcher_icon: string;
  widget_name: string;
  widget_avatar_url: string;
  widget_help_space_ids: string[];
  csat_enabled: boolean;
}

export interface SupportInstallationResponse {
  id: string;
  workspace_id: string;
  widget_key: string;
  settings: SupportInboxSettings;
  active: boolean;
  created_at: string;
  updated_at: string;
}

// ── Git Integration ─────────────────────────────────────────────────

export interface GitIntegration {
  id: string;
  workspace_id: string;
  provider: string;
  display_name: string;
  credential_mode?: string;
  account_login?: string;
  base_url?: string;
  installation_id?: string;
  app_id?: string;
  last_synced_at?: string;
  last_sync_error?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface GitRepository {
  id: string;
  workspace_id: string;
  integration_id: string;
  provider: string;
  external_id: string;
  full_name: string;
  default_branch: string;
  permissions: Record<string, unknown>;
  private: boolean;
  archived: boolean;
  selected: boolean;
  created_at: string;
  updated_at: string;
}

export interface StoryDeliveryTarget {
  id: string;
  workspace_id: string;
  story_id: string;
  repository_id?: string;
  repo_full_name?: string;
  integration_id?: string;
  base_branch?: string;
  working_branch?: string;
  delivery_state: string;
  active_pr_number?: number;
  active_pr_title?: string;
  active_pr_url?: string;
  active_pr_status?: string;
  last_commit_sha?: string;
  last_run_id?: string;
  last_synced_at?: string;
  created_at: string;
  updated_at: string;
}

export interface StoryGitLink {
  id: string;
  workspace_id: string;
  story_id: string;
  integration_id: string;
  repository_id?: string;
  run_id?: string;
  provider: string;
  repo: string;
  branch?: string;
  pr_number?: number;
  pr_title?: string;
  pr_url?: string;
  pr_status?: string;
  commit_sha?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateGitIntegrationRequest {
  provider: string;
  display_name: string;
  credential_mode?: string;
  account_login?: string;
  base_url?: string;
  installation_id?: string;
  app_id?: string;
  webhook_secret?: string;
  access_token?: string;
}

export interface GitHubInstallURLResponse {
  install_url: string;
}

export interface UpdateGitRepositoryRequest {
  selected: boolean;
}

export interface CreateBranchRequest {
  integration_id: string;
  repo: string;
  branch_name: string;
}

export interface UpdateStoryDeliveryTargetRequest {
  repository_id?: string;
  base_branch?: string;
  working_branch?: string;
}

// ── Orchestration ──────────────────────────────────────────────────


export interface AgentHandoff {
  id: string;
  workspace_id: string;
  from_agent_id?: string;
  to_agent_id?: string;
  to_user_id?: string;
  story_id?: string;
  epic_id?: string;
  run_id?: string;
  handoff_type: string;
  reason: string;
  context: Record<string, unknown>;
  created_at: string;
}

export interface StructuredQuestionOption {
  value: string;
  label: string;
  freetext?: boolean;
}

export interface StructuredQuestion {
  id: string;
  type?: 'single_select';
  text: string;
  options: StructuredQuestionOption[];
}

export interface ToolInputSchemaProperty {
  type: string;
  description?: string;
  items?: Record<string, unknown>;
}

export interface ToolInputSchema {
  type: string;
  properties: Record<string, ToolInputSchemaProperty>;
  required?: string[];
}

export interface ToolCatalogEntry {
  name: string;
  description: string;
  category: string;
  input_schema: ToolInputSchema;
  presets: AgentPresetKey[];
}

export interface ToolCatalogResponse {
  tools: ToolCatalogEntry[];
  categories: string[];
}

// ── Visitor Context ─────────────────────────────────────────────────

export interface VisitorDeviceInfo {
  browser: string;
  browser_version: string;
  os: string;
  os_version: string;
  device_type: string;
}

export interface VisitorLocation {
  timezone: string | null;
  locale: string | null;
  last_page_url: string | null;
}

export interface VisitorContactData {
  id: string;
  name: string | null;
  email: string | null;
  phone: string | null;
  job_title: string | null;
  lifecycle_stage: string;
  lead_status: string;
  source: string;
  custom_properties?: Record<string, string>;
}

export interface VisitorOtherConversation {
  id: string;
  display_id: number;
  subject: string;
  status: ConversationStatus;
  created_at: string;
}

export interface VisitorContextResponse {
  device: VisitorDeviceInfo | null;
  location: VisitorLocation | null;
  contact: VisitorContactData | null;
  other_conversations: VisitorOtherConversation[];
  total_conversations: number;
  session_created_at: string | null;
}
