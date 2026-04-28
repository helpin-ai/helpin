import type { SpecClarification } from './project';
import type { AgentSkillRef } from './skills';
import type { AutomationRule } from './automations';

// ── Agents ──────────────────────────────────────────────────────────

export type AgentPresetKey =
  | 'epic_planner'
  | 'story_planner'
  | 'task_planner'
  | 'crm_operator'
  | 'support_agent'
  | 'code_builder'
  | 'review_agent';
export type AgentStatus = 'idle' | 'working' | 'error' | 'paused';
export type AgentRunStatus = 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';
export type AgentRuntimeKind = 'opencode' | 'codex' | 'native_sdk';
export type AgentTriggerMode = 'manual' | 'auto_on_assignment' | 'auto_on_event';
export type AgentTargetType = 'task' | 'support_conversation' | 'epic' | 'document' | 'crm_deal' | 'repository' | 'workspace';
export type AgentApprovalState = 'not_required' | 'pending' | 'approved' | 'rejected';
export type AgentApprovalMode = 'preset_default' | 'never' | 'always';
export type AgentModelProvider = 'anthropic' | 'openai' | 'openrouter';
export type AgentInvocationMode = 'interactive' | 'autonomous';
export type AgentRunPauseReason = 'none' | 'human_input' | 'human_approval' | 'authentication';
export type CodexAuthStateStatus = 'required' | 'pending' | 'connected' | 'failed' | 'cancelled';
export type AgentReasoningEffort = 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh';
export type AgentServiceTier = 'fast' | 'flex';

export interface AgentExecutionConfig {
  reasoning_effort?: AgentReasoningEffort;
  service_tier?: AgentServiceTier;
}

export interface Agent {
  id: string;
  workspace_id: string;
  is_system: boolean;
  name: string;
  preset_key?: AgentPresetKey;
  preset_version_key?: string;
  source_template_id?: string;
  source_template_key?: string;
  role: string;
  status: AgentStatus;
  runtime_kind: AgentRuntimeKind;
  skills: AgentSkillRef[];
  trigger_mode: AgentTriggerMode;
  provider?: AgentModelProvider;
  model?: string;
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  planning_notes?: string;
  tools: unknown[];
  monthly_token_budget?: number;
  tokens_used_this_month: number;
  active_task_id?: string;
  team_id?: string;
  allowed_tools: string[];
  allowed_commands: string[];
  allowed_targets: string[];
  target_selector?: Record<string, unknown>;
  trigger_events?: string[];
  approval_mode: AgentApprovalMode;
  max_concurrent_runs: number;
  default_invocation_mode: AgentInvocationMode;
  supported_modes?: Array<'interactive' | 'autonomous'>;
  created_at: string;
  updated_at: string;
}

export interface AgentTriggerUsage {
  id: string;
  kind: string;
  title: string;
  description: string;
  trigger_type?: string;
  enabled: boolean;
  reference_id?: string;
  reference_type?: string;
  manage_path?: string;
  execution_search?: import('../types').AutomationTriggerExecutionSearchPreset;
  last_triggered_at?: string;
  last_success_at?: string;
  last_error_at?: string;
  last_error?: string;
  recent_executions?: AgentTriggerExecutionSummary[];
}

export interface AgentTriggerExecutionSummary {
  execution_id: string;
  run_id?: string;
  status: string;
  target_type: string;
  target_id: string;
  fired_at: string;
  started_at?: string;
  completed_at?: string;
  error_message?: string;
  trigger_type?: string;
  reference_id?: string;
  reference_type?: string;
}

export interface AgentTriggerUsageSummary {
  agent_id: string;
  agent_name: string;
  items: AgentTriggerUsage[];
}

export interface AgentRun {
  id: string;
  workspace_id: string;
  agent_id: string;
  task_id?: string;
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
  cached_input_tokens: number;
  input_tokens: number;
  output_tokens: number;
  tokens_used: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  target_info?: AgentRunTarget;
}

export interface AgentRunTarget {
  target_type: string;
  target_id: string;
  title?: string;
  task_key?: string;
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
  allowed_tools?: string[];
  base_branch?: string;
  working_branch?: string;
}

export interface CommandBarPageContext {
  entity_type: 'task' | 'epic' | 'document' | 'crm_contact' | 'crm_deal' | 'workspace';
  entity_id: string;
  display_title: string;
  related_ids?: Record<string, string[]>;
  metadata?: Record<string, unknown>;
}

export interface CommandBarPlanStep {
  agent_id: string;
  agent_key?: string;
  agent_name: string;
  target: CommandBarPageContext;
  instructions: string;
  allowed_tools?: string[];
}

export interface CommandBarPlan {
  id?: string;
  steps: CommandBarPlanStep[];
  run_count: number;
  estimated_runs?: number;
  max_allowed_runs?: number;
  guardrails?: Array<{ type: string; severity: string; message: string }>;
}

export interface CommandBarAgentCandidate {
  id: string;
  name: string;
  preset_key?: string;
  role?: string;
  allowed_targets: string[];
  allowed_tools: string[];
}

export interface CommandBarParseRequest {
  text: string;
  page_context: CommandBarPageContext;
}

export type CommandBarParseResponse =
  | {
      status: 'plan';
      plan: CommandBarPlan;
      rationale?: string;
      candidates?: CommandBarAgentCandidate[];
    }
  | {
      status: 'no_matching_agent';
      reason: string;
      suggestions?: string[];
      candidates?: CommandBarAgentCandidate[];
    };

export interface CommandBarDispatchRequest {
  text: string;
  page_context: CommandBarPageContext;
  steps: CommandBarPlanStep[];
}

export interface CommandBarDispatchResponse {
  plan_id?: string;
  steps?: CommandBarPlanStep[];
  run_count?: number;
  runs: AgentRun[];
}

export interface CommandBarPlanSummary {
  id: string;
  status: 'running' | 'completed' | 'failed' | 'cancelled';
  prompt: string;
  page_context: CommandBarPageContext;
  steps: CommandBarPlanStep[];
  run_ids_by_step: Record<number, string>;
  current_step_index: number;
  run_count: number;
  error_message?: string;
  cancelled_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
  runs?: AgentRun[];
}

export interface CommandBarPlanListResponse {
  plans: CommandBarPlanSummary[];
}

export interface CommandBarCancelPlanResponse {
  plan: CommandBarPlanSummary;
  runs?: AgentRun[];
}

export interface CommandBarRetryPlanResponse {
  plan: CommandBarPlanSummary;
  run: AgentRun;
}

export interface PromoteCommandBarRunRequest {
  name: string;
  description?: string;
}

export interface PromoteCommandBarRunResponse {
  agent: Agent;
}

export interface SendAgentRunMessageRequest {
  content: string;
}

export interface ContinueAgentRunRequest {
  content?: string;
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

export interface TaskImplementationBrief {
  approach: string;
  files_to_modify: FileChange[];
  test_strategy: string;
  vertical_layers?: string[];
  depends_on_files?: string[];
}

/** @deprecated Use TaskImplementationBrief instead */
export type StoryImplementationBrief = TaskImplementationBrief;

export interface FileChange {
  path: string;
  action: 'create' | 'modify' | 'delete';
  description: string;
}

export interface VerticalCoverageEntry {
  behavior: string;
  task_refs: string[];
  story_refs?: string[];
  full_slice: boolean;
}

export interface ProposedTask {
  ref?: string;
  name: string;
  description: string;
  task_type: string;
  story_type?: string;
  estimate?: number;
  priority?: string;
  acceptance_criteria?: string[];
  dependency_refs?: string[];
  source_refs?: PlanningSourceRef[];
  assign_agent_id?: string;
  slice_type?: 'vertical' | 'enabler' | 'spike';
  implementation_brief?: TaskImplementationBrief;
}

/** @deprecated Use ProposedTask instead */
export type ProposedStory = ProposedTask;

export interface PlanningSourceRef {
  type: string;
  id?: string;
  title?: string;
}

export interface OrchestrationProposal {
  epic_id: string;
  summary: string;
  spec_version_id?: string;
  proposed_tasks: ProposedTask[];
  /** @deprecated Use proposed_tasks instead */
  proposed_stories?: ProposedTask[];
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
  skills?: AgentSkillRef[];
  trigger_mode?: AgentTriggerMode;
  provider?: AgentModelProvider;
  model?: string;
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  planning_notes?: string;
  tools?: unknown[];
  monthly_token_budget?: number;
  team_id?: string | null;
  allowed_tools?: string[];
  allowed_commands?: string[];
  allowed_targets?: string[];
  target_selector?: Record<string, unknown>;
  trigger_events?: string[];
  approval_mode?: AgentApprovalMode;
  max_concurrent_runs?: number;
  default_invocation_mode?: AgentInvocationMode;
}

export interface AgentTemplate {
  id: string;
  workspace_id?: string;
  key: string;
  name: string;
  description?: string;
  runtime_kind: AgentRuntimeKind;
  default_role: string;
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  planning_notes?: string;
  skills: AgentSkillRef[];
  allowed_tools: string[];
  allowed_commands: string[];
  allowed_targets: AgentTargetType[];
  required_context: string[];
  starter_flows: AgentTemplateStarterFlow[];
  approval_mode: AgentApprovalMode;
  default_invocation_mode: AgentInvocationMode;
  monthly_token_budget?: number;
  is_enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface AgentTemplateStarterFlow {
  key: string;
  label: string;
  description?: string;
  trigger_type: string;
  default_enabled: boolean;
  config_schema_key?: string;
  output_type?: string;
  fields?: AgentTemplateStarterFlowField[];
}

export interface AgentTemplateStarterFlowField {
  key: string;
  label: string;
  type: 'text' | 'number' | 'string_list' | 'team_select' | 'workflow_state_select' | string;
  required?: boolean;
  placeholder?: string;
  help_text?: string;
  default?: string | number | boolean | string[];
  min?: number;
  max?: number;
  depends_on?: string;
  options?: Array<{ value: string; label: string }>;
}

export interface CreateAgentFromTemplateOverrides {
  role?: string;
  runtime_kind?: AgentRuntimeKind;
  skills?: AgentSkillRef[];
  provider?: AgentModelProvider;
  model?: string;
  monthly_token_budget?: number;
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  planning_notes?: string;
  allowed_tools?: string[];
  allowed_commands?: string[];
  allowed_targets?: AgentTargetType[];
  approval_mode?: AgentApprovalMode;
  max_concurrent_runs?: number;
  default_invocation_mode?: AgentInvocationMode;
}

export interface CreateAgentFromTemplateFlow {
  flow_key?: string;
  flow_input?: Record<string, unknown>;
  repository_id?: string;
  repo_full_name?: string;
  release_kinds?: string[];
  include_prerelease?: boolean;
  tag_pattern?: string;
  space_id?: string;
  collection_id?: string;
}

export interface CreateAgentFromTemplateRequest {
  name?: string;
  team_id?: string | null;
  overrides?: CreateAgentFromTemplateOverrides;
  create_flow?: boolean;
  flow?: CreateAgentFromTemplateFlow;
}

export interface CreateAgentFromTemplateResponse {
  agent: Agent;
  flow?: AutomationRule;
}

export interface UpdateAgentRequest {
  name?: string;
  preset_key?: AgentPresetKey;
  preset_version_key?: string;
  role?: string;
  status?: AgentStatus;
  runtime_kind?: AgentRuntimeKind;
  skills?: AgentSkillRef[];
  trigger_mode?: AgentTriggerMode;
  provider?: AgentModelProvider;
  model?: string;
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  planning_notes?: string;
  tools?: unknown[];
  monthly_token_budget?: number;
  active_task_id?: string;
  team_id?: string | null;
  allowed_tools?: string[];
  allowed_commands?: string[];
  allowed_targets?: string[];
  target_selector?: Record<string, unknown>;
  trigger_events?: string[];
  approval_mode?: AgentApprovalMode;
  max_concurrent_runs?: number;
  default_invocation_mode?: AgentInvocationMode;
}

export interface ApproveAgentRunRequest {
  content?: string;
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
  /** Workspace preset versions expose the underlying row id; product versions omit it. */
  id?: string;
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
  execution_config?: AgentExecutionConfig;
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
  instruction_preamble?: string;
  instruction_skills?: string[];
  instruction_template_version?: string;
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
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  instruction_preamble?: string;
  instruction_skills?: string[];
  allowed_tools?: string[];
  supported_modes?: AgentInvocationMode[];
  approval_mode?: AgentApprovalMode;
  default_invocation_mode?: AgentInvocationMode;
}

export interface UpdateWorkspaceAgentPresetVersionRequest {
  label?: string;
  description?: string;
  runtime_kind?: AgentRuntimeKind;
  provider?: AgentModelProvider;
  model?: string;
  execution_config?: AgentExecutionConfig;
  system_prompt?: string;
  instruction_preamble?: string;
  instruction_skills?: string[];
  allowed_tools?: string[];
  supported_modes?: AgentInvocationMode[];
  default_invocation_mode?: AgentInvocationMode;
}

export interface AgentModelProviderOption {
  value: AgentModelProvider;
  label: string;
  model_placeholder: string;
  supports_reasoning_effort: boolean;
  supported_reasoning_efforts?: AgentReasoningEffort[];
  supports_service_tier: boolean;
  supported_service_tiers?: AgentServiceTier[];
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
