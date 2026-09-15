export interface EmailTemplate {
  id: string;
  workspace_id: string;
  owner_id: string;
  name: string;
  subject: string;
  body_html: string;
  shared: boolean;
  version: number;
  created_at: string;
  updated_at: string;
}
export interface SequenceStep {
  kind: "email" | "task";
  mode?: "automatic" | "review";
  delay_days: number;
  subject?: string;
  body_html?: string;
  task_name?: string;
  team_id?: string;
}
export interface EmailSequence {
  id: string;
  workspace_id: string;
  owner_id: string;
  name: string;
  status: "draft" | "active" | "paused" | "archived";
  version: number;
  steps: SequenceStep[];
  timezone: string;
  start_hour: number;
  end_hour: number;
  weekdays: boolean;
  include_signature: boolean;
  entry_stage_id: string;
  entry_account_id: string;
  entry_error?: string;
  daily_new_recipients?: number;
  created_at: string;
  updated_at: string;
}
export interface SequenceEnrollment {
  id: string;
  workspace_id: string;
  sequence_id: string;
  sequence_name: string;
  sequence_version: number;
  contact_id: string;
  contact_name: string;
  email: string;
  deal_id: string;
  account_id: string;
  owner_id: string;
  status: string;
  step_index: number;
  step_count: number;
  steps: SequenceStep[];
  next_at: string;
  error?: string;
  created_at: string;
  updated_at: string;
}
export interface SequenceDelivery {
  id: string;
  step_index: number;
  step_count: number;
  kind: string;
  status: string;
  subject: string;
  body_html: string;
  result_id?: string;
  error?: string;
  created_at: string;
  updated_at: string;
}
export interface EnrollmentRequest {
  account_id: string;
  contact_ids: string[];
  deal_id?: string;
  version: number;
}
export interface EnrollmentPreview {
  contact_id: string;
  contact_name: string;
  email: string;
  steps: SequenceStep[];
  error?: string;
}

export interface MailboxSendingPolicy {
  daily_limit: number;
  manual_reserve: number;
  min_interval_seconds: number;
}
export interface MailboxCapacity extends MailboxSendingPolicy {
  account_id: string;
  email: string;
  used: number;
  sent: number;
  remaining: number;
  sequence_remaining: number;
  queued: number;
  cooldown_until?: string;
  next_available_at?: string;
}
