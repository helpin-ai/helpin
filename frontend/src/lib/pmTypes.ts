export type StoryType = 'feature' | 'bug' | 'chore';
export type StateType = 'backlog' | 'unstarted' | 'started' | 'done';
export type Priority = 'none' | 'low' | 'medium' | 'high' | 'urgent';
export type Severity = 'none' | 'minor' | 'major' | 'critical';
export type EpicHealth = 'on_track' | 'at_risk' | 'off_track';
export type SprintStatus = 'unstarted' | 'started' | 'done';

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
  owner_id?: string;
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
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface EpicStats {
  story_count: number;
  done_story_count: number;
  total_points: number;
  done_points: number;
  in_progress_count: number;
  unstarted_count: number;
}

export interface EpicWithStats {
  epic: Epic;
  labels: Label[];
  stats: EpicStats;
}

export interface PMSprint {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  start_date: string;
  end_date: string;
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
  owner_id?: string;
  requester_id?: string;
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
  archived: boolean;
  template_id?: string;
  external_id?: string;
  created_at: string;
  updated_at: string;
  // Enriched by board endpoint
  epic_name?: string;
  owner_name?: string;
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
  labels: Label[];
  epic_name?: string;
  sprint_name?: string;
  state?: WorkflowState;
}

export interface StoryStateColumn {
  state: WorkflowState;
  stories: Story[];
  story_count: number;
  point_total: number;
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
  name: string;
  description?: string;
  color?: string;
}

export interface UpdateLabelRequest {
  name?: string;
  description?: string;
  color?: string;
  archived?: boolean;
}

export interface CreateEpicRequest {
  workspace_id: string;
  name: string;
  description?: string;
  epic_state_id?: string;
  owner_id?: string;
  team_id?: string;
  planned_start_date?: string;
  deadline?: string;
  position?: number;
  color?: string;
  health?: EpicHealth;
  health_comment?: string;
  label_ids?: string[];
}

export interface UpdateEpicRequest {
  name?: string;
  description?: string;
  epic_state_id?: string;
  owner_id?: string;
  team_id?: string;
  planned_start_date?: string;
  deadline?: string;
  position?: number;
  color?: string;
  archived?: boolean;
  health?: EpicHealth;
  health_comment?: string;
  label_ids?: string[];
}

export interface UpdateEpicHealthRequest {
  health: EpicHealth;
  comment?: string;
}

export interface CreateSprintRequest {
  workspace_id: string;
  name: string;
  description?: string;
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
  story_type?: StoryType;
  workflow_id?: string;
  workflow_state_id?: string;
  epic_id?: string;
  sprint_id?: string;
  team_id?: string;
  owner_id?: string;
  requester_id?: string;
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
  owner_id?: string;
  requester_id?: string;
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
  user_id: string;
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
  created_at: string;
  updated_at: string;
}

export interface CreateChecklistItemRequest {
  text: string;
  position?: number;
}

export interface UpdateChecklistItemRequest {
  text?: string;
  completed?: boolean;
  position?: number;
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
  entity_type: 'story' | 'epic' | 'comment';
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
  entity_type: 'story' | 'epic' | 'comment' | 'editor_upload';
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
}

export interface UpdateCommentRequest {
  body: string;
}

// ── Objectives ──────────────────────────────────────────────────────

export type ObjectiveType = 'tactical' | 'strategic';
export type ObjectiveState = 'to_do' | 'in_progress' | 'done';
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
  position: number;
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
  labels: Label[];
  key_results: KeyResult[];
  epics: EpicWithStats[];
  stats: ObjectiveStats;
}

export interface CreateObjectiveRequest {
  workspace_id: string;
  name: string;
  description?: string;
  objective_type: ObjectiveType;
  state?: ObjectiveState;
  planned_start_date?: string;
  deadline?: string;
  position?: number;
  team_ids?: string[];
  owner_ids?: string[];
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
  position?: number;
  archived?: boolean;
  team_ids?: string[];
  owner_ids?: string[];
  label_ids?: string[];
  epic_ids?: string[];
}

export interface CreateKeyResultRequest {
  name: string;
  result_type?: KeyResultType;
  initial_value?: number;
  current_value?: number;
  target_value?: number;
  position?: number;
}

export interface UpdateKeyResultRequest {
  name?: string;
  result_type?: KeyResultType;
  initial_value?: number;
  current_value?: number;
  target_value?: number;
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
