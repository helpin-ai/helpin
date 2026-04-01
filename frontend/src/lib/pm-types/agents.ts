import type { SpecClarification } from './project';

// ── Agents ──────────────────────────────────────────────────────────

export type AgentPresetKey =
  | 'epic_planner'
  | 'story_planner'
  | 'crm_operator'
  | 'support_agent'
  | 'code_builder'
  | 'review_agent';
export type AgentStatus = 'idle' | 'working' | 'error' | 'paused';
export type AgentRunStatus = 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';
export type AgentRuntimeKind = 'opencode' | 'codex' | 'native_sdk';
export type AgentTriggerMode = 'manual' | 'auto_on_assignment' | 'auto_on_event';
export type AgentTargetType = 'story' | 'support_conversation' | 'epic' | 'document' | 'crm_deal';
export type AgentApprovalState = 'not_required' | 'pending' | 'approved' | 'rejected';
export type AgentApprovalMode = 'preset_default' | 'never' | 'always';
export type AgentModelProvider = 'anthropic' | 'openai' | 'openrouter';
export type AgentInvocationMode = 'interactive' | 'autonomous';
export type AgentRunPauseReason = 'none' | 'human_input' | 'human_approval' | 'authentication';
export type CodexAuthStateStatus = 'required' | 'pending' | 'connected' | 'failed' | 'cancelled';

export interface Agent {
  id: string;
  workspace_id: string;
  is_system: boolean;
  name: string;
  preset_key?: AgentPresetKey;
  preset_version_key?: string;
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
  pause_reason: AgentRunPauseReason;
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

export interface ResumeAgentRunRequest {
  intent: 'reply' | 'approve' | 'request_changes' | 'auth_completed';
  content?: string;
  send_message?: boolean;
}

export interface CodexAuthState {
  provider?: string;
  auth_mode?: string;
  state: CodexAuthStateStatus;
  login_id?: string;
  auth_url?: string;
  verification_url?: string;
  user_code?: string;
  plan_type?: string;
  error?: string;
  updated_at: string;
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
  preset_version_key?: string;
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
  preset_version_key?: string;
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
  family_key: AgentPresetKey;
  version_key: string;
  version_label: string;
  is_default_version: boolean;
  scope?: 'product' | 'workspace';
  workspace_id?: string;
  source_version_key?: string;
  provider?: AgentModelProvider;
  model?: string;
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

export interface CreateWorkspaceAgentPresetVersionRequest {
  workspace_id: string;
  family_key: AgentPresetKey;
  label: string;
  description?: string;
  source_version_key?: string;
  runtime_kind?: AgentRuntimeKind;
  provider?: AgentModelProvider;
  model?: string;
  system_prompt?: string;
  allowed_tools?: string[];
  supported_modes?: AgentInvocationMode[];
  approval_mode?: AgentApprovalMode;
  default_invocation_mode?: AgentInvocationMode;
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
