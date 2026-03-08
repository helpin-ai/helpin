import type { AssignableMember } from './types';

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
  orchestrator_agent_id?: string;
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
  suggested_health: EpicHealth;
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
  archived: boolean;
  assigned_agent_id?: string;
  template_id?: string;
  external_id?: string;
  created_at: string;
  updated_at: string;
  // Enriched by board/list endpoints
  epic_name?: string;
  owner_name?: string;
  labels?: Label[];
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
  archived: boolean;
  created_at: string;
  updated_at: string;
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
  archived?: boolean;
}

export interface CreateEpicRequest {
  workspace_id: string;
  name: string;
  description?: string;
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

export type AgentKind = 'human' | 'llm';
export type AgentStatus = 'idle' | 'working' | 'error' | 'paused';
export type AgentRunStatus = 'queued' | 'running' | 'awaiting_approval' | 'completed' | 'failed' | 'cancelled';
export type AgentRuntimeKind = 'native_claude' | 'claude_code' | 'openclaw' | 'zeroclaw';
export type AgentTriggerMode = 'manual' | 'auto_on_assignment' | 'auto_on_event';
export type AgentTargetType = 'story' | 'support_ticket' | 'epic_review' | 'document';
export type AgentApprovalState = 'not_required' | 'pending' | 'approved' | 'rejected';

export interface Agent {
  id: string;
  workspace_id: string;
  name: string;
  agent_kind: AgentKind;
  role: string;
  status: AgentStatus;
  backing_user_id?: string;
  runtime_kind: AgentRuntimeKind;
  capability_profile: string;
  skills: string[];
  trigger_mode: AgentTriggerMode;
  model?: string;
  system_prompt?: string;
  tools: unknown[];
  monthly_token_budget?: number;
  tokens_used_this_month: number;
  active_story_id?: string;
  created_at: string;
  updated_at: string;
}

export interface AgentRun {
  id: string;
  workspace_id: string;
  agent_id: string;
  story_id?: string;
  ticket_id?: string;
  target_type: AgentTargetType;
  target_id: string;
  runtime_kind: AgentRuntimeKind;
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
  agent_kind: AgentKind;
  role: string;
  backing_user_id?: string;
  runtime_kind?: AgentRuntimeKind;
  capability_profile?: string;
  skills?: string[];
  trigger_mode?: AgentTriggerMode;
  model?: string;
  system_prompt?: string;
  tools?: unknown[];
  monthly_token_budget?: number;
}

export interface UpdateAgentRequest {
  name?: string;
  role?: string;
  status?: AgentStatus;
  backing_user_id?: string;
  runtime_kind?: AgentRuntimeKind;
  capability_profile?: string;
  skills?: string[];
  trigger_mode?: AgentTriggerMode;
  model?: string;
  system_prompt?: string;
  tools?: unknown[];
  monthly_token_budget?: number;
  active_story_id?: string;
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

export interface RuntimeProfile {
  name: string;
  runtime_kind: AgentRuntimeKind;
  description: string;
  allowed_tools: string[];
  allowed_commands: string[];
  approval_required: boolean;
  requires_repo: boolean;
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

export type TicketStatus = 'open' | 'in_progress' | 'waiting' | 'resolved' | 'closed';
export type TicketPriority = 'low' | 'medium' | 'high' | 'urgent';
export type TicketSource = 'widget' | 'internal' | 'email' | 'api';
export type MessageSenderType = 'customer' | 'user' | 'agent';

export interface SupportTicket {
  id: string;
  workspace_id: string;
  display_id: number;
  subject: string;
  status: TicketStatus;
  priority: TicketPriority;
  customer_name?: string;
  customer_email?: string;
  opened_by_user_id?: string;
  assigned_agent_id?: string;
  linked_story_id?: string;
  source: TicketSource;
  created_at: string;
  updated_at: string;
}

export interface SupportMessage {
  id: string;
  workspace_id: string;
  ticket_id: string;
  sender_type: MessageSenderType;
  sender_user_id?: string;
  sender_agent_id?: string;
  sender_display_name?: string;
  content: string;
  is_internal: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateTicketRequest {
  subject: string;
  priority?: TicketPriority;
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

export interface AssignTicketAgentRequest {
  agent_id: string;
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

export interface ProposedStory {
  name: string;
  description: string;
  story_type: string;
  estimate?: number;
  assign_agent_id?: string;
}

export interface OrchestrationProposal {
  epic_id: string;
  summary: string;
  proposed_stories: ProposedStory[];
  tokens_used: number;
}

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
