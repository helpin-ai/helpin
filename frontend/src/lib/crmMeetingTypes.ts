import type { CRMAssociationEnriched, CRMPaginatedResponse } from './crmTypes';

export type CRMMeetingPlatform = 'google_meet' | 'zoom' | 'teams' | 'webex';
export type CRMMeetingStatus = 'scheduled' | 'joining' | 'waiting' | 'recording' | 'finalizing' | 'processing' | 'ready' | 'failed' | 'cancelled';
export type CRMMeetingSummaryStatus = 'pending' | 'processing' | 'ready' | 'blocked_usage' | 'failed' | 'not_requested';
export type CRMMeetingVisibility = 'workspace' | 'participants' | 'private';
export type CRMMeetingActionStatus = 'pending' | 'accepted' | 'dismissed';

export interface CRMMeeting {
  id: string;
  workspace_id: string;
  calendar_event_id?: string;
  activity_id?: string;
  owner_member_id?: string;
  title: string;
  meeting_url: string;
  platform: CRMMeetingPlatform;
  native_meeting_id: string;
  status: CRMMeetingStatus;
  summary_status: CRMMeetingSummaryStatus;
  visibility: CRMMeetingVisibility;
  record_audio: boolean;
  scheduled_start_at?: string;
  scheduled_end_at?: string;
  actual_start_at?: string;
  actual_end_at?: string;
  duration_seconds: number;
  participants: Array<{ name?: string; email?: string }>;
  failure_code?: string;
  failure_message?: string;
  recording_object_key?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface CRMMeetingCapture {
  id: string;
  workspace_id: string;
  meeting_id: string;
  status: CRMMeetingStatus;
  started_at?: string;
  ended_at?: string;
  failure_code?: string;
  failure_message?: string;
  created_at: string;
  updated_at: string;
}

export interface CRMMeetingTranscriptSegment {
  id: string;
  speaker_id?: string;
  speaker_name: string;
  text: string;
  start_seconds: number;
  end_seconds: number;
  language?: string;
  confidence?: number;
}

export interface CRMMeetingTranscript {
  id: string;
  workspace_id: string;
  meeting_id: string;
  capture_id: string;
  language?: string;
  plain_text: string;
  segments: CRMMeetingTranscriptSegment[];
  checksum: string;
  created_at: string;
  updated_at: string;
}

export interface CRMMeetingIntelligence {
  id: string;
  workspace_id: string;
  meeting_id: string;
  generation_version: string;
  transcript_checksum: string;
  summary_markdown: string;
  key_points: string[];
  decisions: string[];
  objections: string[];
  risks: string[];
  next_steps: string[];
  follow_up_draft: { subject?: string; body?: string };
  created_at: string;
  updated_at: string;
}

export interface CRMMeetingActionItem {
  id: string;
  workspace_id: string;
  meeting_id: string;
  position: number;
  title: string;
  details?: string;
  assignee_name?: string;
  assignee_member_id?: string;
  due_date?: string;
  evidence: { excerpt?: string };
  status: CRMMeetingActionStatus;
  task_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CRMMeetingDetail {
  meeting: CRMMeeting;
  capture?: CRMMeetingCapture;
  transcript?: CRMMeetingTranscript;
  intelligence?: CRMMeetingIntelligence;
  action_items: CRMMeetingActionItem[];
  associations: CRMAssociationEnriched[];
}

export interface CreateCRMMeetingRequest {
  workspace_id: string;
  title: string;
  meeting_url: string;
  calendar_event_id?: string;
  owner_member_id?: string;
  scheduled_start_at?: string;
  scheduled_end_at?: string;
  visibility?: CRMMeetingVisibility;
  record_audio?: boolean;
  start_now: boolean;
}

export interface UpdateCRMMeetingRequest {
  title?: string;
  calendar_event_id?: string;
  owner_member_id?: string;
  scheduled_start_at?: string;
  scheduled_end_at?: string;
  visibility?: CRMMeetingVisibility;
  record_audio?: boolean;
}

export interface CRMMeetingFilters {
  status?: string;
  owner_member_id?: string;
  company_id?: string;
  contact_id?: string;
  deal_id?: string;
  search?: string;
  start_after?: string;
  start_before?: string;
  page?: number;
  per_page?: number;
}

export interface CRMMeetingSettings {
  workspace_id: string;
  enabled: boolean;
  bot_name: string;
  auto_join_mode: 'manual' | 'external' | 'all';
  record_audio_by_default: boolean;
  default_visibility: CRMMeetingVisibility;
  include_internal: boolean;
  include_private: boolean;
  include_solo: boolean;
  transcript_retention_days: number;
  audio_retention_days: number;
  created_at?: string;
  updated_at?: string;
}

export type UpdateCRMMeetingSettingsRequest = Partial<Pick<CRMMeetingSettings,
  | 'enabled'
  | 'bot_name'
  | 'record_audio_by_default'
>>;

export interface CRMMeetingSettingsResponse {
  settings: CRMMeetingSettings;
}

export interface AcceptCRMMeetingActionItemRequest {
  team_id: string;
  workflow_id?: string;
  workflow_state_id?: string;
  owner_member_id?: string;
}

export type CRMMeetingListResponse = CRMPaginatedResponse<CRMMeeting[]>;
