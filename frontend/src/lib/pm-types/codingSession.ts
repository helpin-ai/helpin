import type { AgentRunDeliveryMode, AgentApprovalState, AgentInvocationMode, AgentRunPauseReason, AgentRunStatus, AgentRuntimeKind } from './agents';

export type CodingSessionInteractionKind =
  | 'request_user_input'
  | 'approval_request'
  | 'command_execution_approval'
  | 'file_change_approval'
  | 'permissions_approval'
  | 'review_checkpoint'
  | 'auth_required';

export type CodingSessionInteractionStatus = 'pending' | 'resolved' | 'cancelled';

export interface CodingSessionApprovalRequestPayload {
  phase?: string;
  preview_panel_key?: string;
  title?: string;
  summary?: string;
}

export interface CodingSessionApprovalResponsePayload {
  [key: string]: unknown;
  decision: 'approve' | 'request_changes';
  message?: string;
}

export interface CodingSessionReviewFinding {
  id: string;
  title: string;
  body: string;
  priority?: string;
  confidence?: string;
  code_location?: string;
}

export interface CodingSessionReviewCheckpointRequestPayload {
  phase?: string;
  title?: string;
  summary?: string;
  findings?: CodingSessionReviewFinding[];
  overall_correctness?: string;
  overall_explanation?: string;
  overall_confidence_score?: number;
}

export interface CodingSessionReviewCheckpointResponsePayload {
  [key: string]: unknown;
  decision: 'approve' | 'request_changes' | 'skip';
  message?: string;
  selection_mode?: 'all' | 'selected' | 'none';
  selected_finding_ids?: string[];
}

export interface CodingSessionInteraction {
  interaction_id: string;
  interaction_kind: CodingSessionInteractionKind;
  status: CodingSessionInteractionStatus;
  request_schema_version: string;
  response_schema_version?: string;
  request_payload: Record<string, unknown>;
  response_payload?: Record<string, unknown>;
  title?: string;
  summary?: string;
  request_id?: string;
  thread_id?: string;
  turn_id?: string;
  item_id?: string;
  approval_id?: string;
  assistant_message_sequence_no?: number;
  resolved_at?: string;
  resolved_by?: string;
}

export interface ResolveCodingSessionInteractionRequest {
  response_payload: Record<string, unknown>;
  followup_message?: string;
}

export interface CodingSessionCapabilities {
  live_text_streaming: boolean;
  tool_streaming: boolean;
  repo_diff_streaming: boolean;
  plan_streaming: boolean;
  approvals: boolean;
  human_input: boolean;
  authentication: boolean;
  previews: boolean;
  terminal_output: boolean;
  checkpoints: boolean;
}

export interface CodingSessionRepoFile {
  path: string;
  status: string;
}

export interface CodingSessionRepoState {
  repo_name?: string;
  branch?: string;
  is_dirty: boolean;
  changed_file_count: number;
  changed_files?: CodingSessionRepoFile[];
}

export interface CodingSessionDiff {
  path?: string;
  diff: string;
  is_truncated: boolean;
}

export interface CodingSession {
  execution_location?: 'local' | 'cloud';
  delivery_mode?: AgentRunDeliveryMode;
  id: string;
  run_id: string;
  parent_run_id?: string;
  workspace_id: string;
  target_type: string;
  target_id: string;
  agent_id: string;
  runtime_kind: AgentRuntimeKind;
  invocation_mode: AgentInvocationMode;
  status: AgentRunStatus;
  pause_reason: AgentRunPauseReason;
  approval_state: AgentApprovalState;
  error_message?: string;
  execution_stage?: string;
  last_heartbeat_at?: string;
  started_at?: string;
  title: string;
  summary?: string;
  system_prompt?: string;
  capabilities: CodingSessionCapabilities;
  repo: CodingSessionRepoState;
  cached_input_tokens: number;
  input_tokens: number;
  output_tokens: number;
  tokens_used: number;
  stream_state_snapshot?: CodingSessionStreamSnapshot;
  triggered_by_user?: CodingSessionActor;
  created_at: string;
  updated_at: string;
}

export interface CodingSessionActor {
  id: string;
  email: string;
  full_name: string;
  avatar_url?: string;
  avatar_style?: string;
  avatar_seed?: string;
  avatar_background_mode?: string;
  avatar_background_color?: string;
}

export interface CodingSessionEvent {
  id: string;
  session_id: string;
  run_id: string;
  sequence_no: number;
  timestamp: string;
  type: string;
  runtime_kind: AgentRuntimeKind;
  payload: Record<string, unknown>;
  runtime_metadata?: Record<string, unknown>;
}

export interface CodingSessionEventListResponse {
  events: CodingSessionEvent[];
  next_sequence_no: number;
  stream_state_snapshot?: CodingSessionStreamSnapshot;
}

export interface CodingSessionTranscriptMessage {
  event_id: string;
  message_id?: string;
  client_message_id?: string;
  delivery_status?: 'pending' | 'sent' | 'failed';
  role: 'assistant' | 'user';
  content: string;
  message_type?: string;
  timestamp: string;
  sequence_no: number;
  tool_calls?: CodingSessionLiveToolCall[];
  turn_segments?: CodingSessionLiveTurnSegment[];
  dock_work_summary?: {
    message_id: string;
    duration_ms: number;
    activity_count: number;
  };
  attachments?: Array<{ id: string; file_name: string; file_type: string; file_size: number }>;
  // For review_checkpoint_resolution / approval_request_resolution messages,
  // the workspace user who resolved the interaction (so the UI can render
  // their avatar and name).
  resolver_user_id?: string;
  /** Workspace user who authored this human message. */
  actor_user_id?: string;
}

export interface CodingSessionLiveToolResult {
  message_id?: string;
  content: string;
  output_summary?: string;
  error?: string;
}

export interface CodingSessionLiveToolCall {
  tool_call_id: string;
  parent_message_id?: string;
  tool_name: string;
  args_text: string;
  status: 'running' | 'completed' | 'failed';
  duration_ms?: number;
  started_at?: string;
  completed_at?: string;
  result?: CodingSessionLiveToolResult;
}

export interface CodingSessionLiveAssistantMessage {
  message_id: string;
  message_type?: string;
  content: string;
  started_at?: string;
  completed_at?: string;
  status: 'streaming' | 'completed';
  tool_calls: CodingSessionLiveToolCall[];
}

export interface CodingSessionLiveReasoningMessage {
  message_id: string;
  content: string;
  started_at?: string;
  completed_at?: string;
  status: 'streaming' | 'completed';
  encrypted_value?: string;
}

export interface CodingSessionLiveAssistantSegment {
  segment_id: string;
  kind: 'assistant_message';
  assistant_message: CodingSessionLiveAssistantMessage;
}

export interface CodingSessionLiveToolCallSegment {
  segment_id: string;
  kind: 'tool_call';
  tool_call: CodingSessionLiveToolCall;
}

export type CodingSessionLiveTurnSegment =
  | CodingSessionLiveAssistantSegment
  | CodingSessionLiveToolCallSegment;

export interface CodingSessionStreamSnapshot {
	turn_state?: CodingSessionTurnState;
  through_sequence?: number;
  live_assistant_message?: CodingSessionLiveAssistantMessage;
  live_reasoning_message?: CodingSessionLiveReasoningMessage;
  live_turn_segments?: CodingSessionLiveTurnSegment[];
  current_plan?: RunPlanArtifact;
  work_plans?: RunPlanArtifact[];
}

export interface CodingSessionTurnState {
  turn_id: string;
  phase: 'working' | 'answered' | 'waiting' | 'failed' | 'cancelled' | 'missing_answer';
  started_at: string;
  completed_at?: string;
  answer_message_id?: string;
}

export interface RunPlanStep {
  step: string;
  status: 'pending' | 'in_progress' | 'completed';
}

export interface RunPlanArtifact {
  origin?: { event_id: string; sequence_no?: number; turn_id?: string; created_at: string };
  note?: string;
  plan: RunPlanStep[];
}

export interface CodingSessionStreamState {
  turn_state?: CodingSessionTurnState;
  transcript_messages: CodingSessionTranscriptMessage[];
  live_assistant_message: CodingSessionLiveAssistantMessage | null;
  live_reasoning_message: CodingSessionLiveReasoningMessage | null;
  live_turn_segments: CodingSessionLiveTurnSegment[];
  activity_events: CodingSessionEvent[];
  current_plan: RunPlanArtifact | null;
  work_plans?: RunPlanArtifact[];
  completed_tool_calls: CodingSessionLiveToolCall[];
}
