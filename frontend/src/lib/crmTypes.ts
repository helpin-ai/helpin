// CRM Module TypeScript interfaces

export type * from "./crmSituationTypes";

export type LifecycleStage =
  | "subscriber"
  | "lead"
  | "marketing_qualified"
  | "sales_qualified"
  | "opportunity"
  | "customer"
  | "evangelist";

export type LeadStatus = "new" | "open" | "in_progress" | "unqualified";

export type CRMActivityType = "note" | "call" | "meeting" | "email";

export type PipelineStageType = "open" | "won" | "lost";

export type CRMObjectType =
  | "contact"
  | "company"
  | "deal"
  | "meeting"
  | "epic"
  | "task"
  | "support_conversation";

export interface CRMContact {
  id: string;
  workspace_id: string;
  display_id: string;
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  job_title?: string;
  description?: string;
  labels?: string[];
  primary_location?: string;
  country_code?: string;
  country_name?: string;
  linkedin_url?: string;
  facebook_url?: string;
  instagram_url?: string;
  angellist_url?: string;
  x_url?: string;
  lifecycle_stage: LifecycleStage;
  lead_status: LeadStatus;
  owner_member_id?: string;
  avatar_url?: string;
  source?: string;
  custom_properties: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMContactRequest {
  workspace_id: string;
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  job_title?: string;
  description?: string;
  labels?: string[];
  primary_location?: string;
  country_code?: string;
  country_name?: string;
  linkedin_url?: string;
  facebook_url?: string;
  instagram_url?: string;
  angellist_url?: string;
  x_url?: string;
  lifecycle_stage?: LifecycleStage;
  lead_status?: LeadStatus;
  owner_member_id?: string;
  avatar_url?: string;
  source?: string;
  custom_properties?: Record<string, unknown>;
}

export interface UpdateCRMContactRequest {
  first_name?: string;
  last_name?: string;
  email?: string;
  phone?: string;
  job_title?: string;
  description?: string;
  labels?: string[];
  primary_location?: string;
  country_code?: string;
  country_name?: string;
  linkedin_url?: string;
  facebook_url?: string;
  instagram_url?: string;
  angellist_url?: string;
  x_url?: string;
  lifecycle_stage?: LifecycleStage;
  lead_status?: LeadStatus;
  owner_member_id?: string;
  avatar_url?: string;
  source?: string;
  custom_properties?: Record<string, unknown>;
}

export interface SeedCRMContactsRequest {
  workspace_id: string;
  count?: number;
}

export interface SeedCRMContactsResponse {
  created: number;
}

export interface CRMCompany {
  id: string;
  workspace_id: string;
  display_id: string;
  external_id?: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  linkedin_url?: string;
  headquarters?: string;
  owner_member_id?: string;
  customer_success_owner_member_id?: string;
  commercial_state_health?: {
    last_accepted_at?: string;
    last_rejected_at?: string;
    last_rejection_reason?: string;
    rejected_update_count: number;
  };
  custom_properties: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMCompanyRequest {
  workspace_id: string;
  external_id?: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  linkedin_url?: string;
  headquarters?: string;
  owner_member_id?: string;
  customer_success_owner_member_id?: string;
  custom_properties?: Record<string, unknown>;
}

export interface UpdateCRMCompanyRequest {
  external_id?: string;
  name?: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  linkedin_url?: string;
  headquarters?: string;
  owner_member_id?: string;
  customer_success_owner_member_id?: string;
  clear_customer_success_owner?: boolean;
  custom_properties?: Record<string, unknown>;
}

export interface CRMPipelineStage {
  deal_count?: number;
  id: string;
  pipeline_id: string;
  name: string;
  stage_type: PipelineStageType;
  position: number;
  probability: number;
  created_at: string;
  updated_at: string;
}

export interface CRMPipeline {
  id: string;
  workspace_id: string;
  name: string;
  is_default: boolean;
  default_commercial_motion: CRMDealCommercialMotion;
  position: number;
  deal_count: number;
  stages?: CRMPipelineStage[];
  created_at: string;
  updated_at: string;
}

export interface CreateCRMPipelineRequest {
  workspace_id: string;
  name: string;
  is_default?: boolean;
  default_commercial_motion?: CRMDealCommercialMotion;
  stages?: {
    name: string;
    stage_type: PipelineStageType;
    position: number;
    probability: number;
  }[];
}

export interface UpdateCRMPipelineRequest {
  expected_updated_at?: string;
  stage_migrations?: Record<string, string>;
  name?: string;
  is_default?: boolean;
  default_commercial_motion?: CRMDealCommercialMotion;
  stages?: {
    id?: string;
    name: string;
    stage_type: PipelineStageType;
    position: number;
    probability: number;
  }[];
}

export interface CRMDeal {
  id: string;
  workspace_id: string;
  display_id: string;
  name: string;
  pipeline_id: string;
  stage_id: string;
  amount?: number;
  currency: string;
  close_date?: string;
  owner_member_id?: string;
  commercial_motion?: CRMDealCommercialMotion;
  probability?: number;
  custom_properties: Record<string, unknown>;
  pipeline?: CRMPipeline;
  stage?: CRMPipelineStage;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMDealRequest {
  workspace_id: string;
  name: string;
  contact_id?: string;
  company_id?: string;
  pipeline_id: string;
  stage_id: string;
  amount?: number;
  currency?: string;
  close_date?: string;
  owner_member_id?: string;
  commercial_motion?: CRMDealCommercialMotion;
  probability?: number;
  custom_properties?: Record<string, unknown>;
}

export interface SetCRMDealCustomerRequest {
  workspace_id: string;
  contact_id?: string;
  company_id?: string;
}

export interface CRMDealCustomer {
  customer_type: 'contact' | 'company';
  customer_id: string;
  primary_contact_id?: string;
}

export interface UpdateCRMDealRequest {
  name?: string;
  pipeline_id?: string;
  stage_id?: string;
  amount?: number;
  currency?: string;
  close_date?: string;
  owner_member_id?: string;
  commercial_motion?: CRMDealCommercialMotion;
  clear_commercial_motion?: boolean;
  probability?: number;
  custom_properties?: Record<string, unknown>;
}

export type CRMDealCommercialMotion = "new_business" | "expansion" | "renewal";

export interface CRMAssociation {
  id: string;
  workspace_id: string;
  from_object_type: CRMObjectType;
  from_object_id: string;
  to_object_type: CRMObjectType;
  to_object_id: string;
  association_label?: string;
  created_at: string;
}

export interface CreateCRMAssociationRequest {
  workspace_id: string;
  from_object_type: CRMObjectType;
  from_object_id: string;
  to_object_type: CRMObjectType;
  to_object_id: string;
  association_label?: string;
}

export interface CRMAssociationEnriched extends CRMAssociation {
  linked_object_name: string;
  linked_object_display_id: string;
  linked_object_status?: string;
  linked_object_status_color?: string;
  inferred?: boolean;
  context_label?: string;
}

export interface CRMActivity {
  id: string;
  workspace_id: string;
  activity_type: CRMActivityType;
  contact_id?: string;
  company_id?: string;
  deal_id?: string;
  owner_member_id?: string;
  subject?: string;
  body?: string;
  occurred_at: string;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export type CRMTimelineFilter =
  | "all"
  | "note"
  | "email"
  | "call"
  | "meeting"
  | "task"
  | "deal"
  | "support";

export interface CRMTimelineReference {
  type: string;
  id: string;
  name: string;
  display_id?: string;
}

export interface CRMTimelineItem {
  id: string;
  kind: CRMActivityType | "task" | "deal" | "support" | "enrichment";
  event_type: string;
  source_type: string;
  source_id: string;
  title: string;
  description?: string;
  occurred_at: string;
  actor?: CRMTimelineReference;
  contact?: CRMTimelineReference;
  entity?: CRMTimelineReference;
  can_edit: boolean;
  can_delete: boolean;
}

export interface CRMTimelinePage {
  data: CRMTimelineItem[];
  next_cursor?: string;
}

export type CRMCompanyTimelineFilter = CRMTimelineFilter;
export type CRMCompanyTimelineReference = CRMTimelineReference;
export type CRMCompanyTimelineItem = CRMTimelineItem;
export type CRMCompanyTimelinePage = CRMTimelinePage;

export interface CreateCRMActivityRequest {
  workspace_id: string;
  activity_type: CRMActivityType;
  contact_id?: string;
  company_id?: string;
  deal_id?: string;
  owner_member_id?: string;
  subject?: string;
  body?: string;
  occurred_at?: string;
  metadata?: Record<string, unknown>;
}

export interface UpdateCRMActivityRequest {
  activity_type?: CRMActivityType;
  contact_id?: string;
  company_id?: string;
  deal_id?: string;
  subject?: string;
  body?: string;
  occurred_at?: string;
  metadata?: Record<string, unknown>;
}

export interface CRMPaginatedResponse<T> {
  data: T;
  total: number;
  page: number;
}

// ── Phase 2: Import ──

export type CRMImportSource = "csv" | "hubspot";
export type CRMImportStatus = "pending" | "processing" | "completed" | "failed";

export interface CRMImportJob {
  id: string;
  workspace_id: string;
  source: CRMImportSource;
  status: CRMImportStatus;
  object_type: CRMObjectType;
  file_url?: string;
  column_mapping: Record<string, unknown>;
  total_rows: number;
  processed_rows: number;
  created_rows: number;
  updated_rows: number;
  error_count: number;
  error_log: Record<string, unknown>;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMImportRequest {
  workspace_id: string;
  source: CRMImportSource;
  object_type: CRMObjectType;
  file_url?: string;
  column_mapping?: Record<string, unknown>;
  total_rows?: number;
}

export interface ImportColumnMapping {
  csv_column: string;
  crm_field: string;
  is_custom: boolean;
}

export interface ProcessCRMImportRequest {
  column_mapping: ImportColumnMapping[];
  csv_data: string[][];
}

// ── Phase 3: Email & Calendar ──

export type CRMEmailProvider = "gmail" | "microsoft";
export type CRMEmailDirection = "inbound" | "outbound";
export type CRMEmailAccountStatus =
  | "pending_oauth"
  | "connected"
  | "disconnected"
  | "error";

export interface CRMEmailAccount {
  id: string;
  workspace_id: string;
  member_id: string;
  provider: CRMEmailProvider;
  email_address: string;
  normalized_email_address?: string;
  sync_state: Record<string, unknown>;
  last_history_id?: string;
  last_synced_at?: string;
  is_active: boolean;
  status: CRMEmailAccountStatus;
  disconnected_at?: string;
  has_synced_data: boolean;
  can_send?: boolean;
  created_at: string;
  updated_at: string;
}

export interface CRMEmailSyncError {
  operation: string;
  code: string;
  message: string;
}

export interface CRMEmailSyncCycleStats {
  mode: string;
  started_at?: string;
  completed_at?: string;
  messages_seen: number;
  messages_stored: number;
  duplicates_skipped: number;
  filtered_skipped: number;
  internal_skipped: number;
  contacts_created: number;
  associations_written: number;
  threads_touched: number;
  recovery_triggered: boolean;
}

export interface CRMEmailAccountDiagnostics {
  account_id: string;
  workspace_id: string;
  member_id: string;
  provider: string;
  email_address: string;
  account_status: CRMEmailAccountStatus;
  is_active: boolean;
  disconnected_at?: string;
  last_synced_at?: string;
  last_history_id?: string;
  has_synced_data: boolean;
  sync: {
    status: string;
    phase: string;
    last_attempt_at?: string;
    last_success_at?: string;
    last_failure_at?: string;
    consecutive_failures: number;
    last_history_id?: string;
    last_error?: CRMEmailSyncError;
    last_cycle?: CRMEmailSyncCycleStats;
  };
  counts: {
    threads: number;
    messages: number;
    calendar_events: number;
  };
  association_health: {
    messages_missing_associations: number;
    threads_with_empty_contact_ids: number;
  };
}

export interface CreateCRMEmailAccountRequest {
  workspace_id: string;
  member_id: string;
  provider: CRMEmailProvider;
  email_address: string;
}

export interface CRMEmailThread {
  id: string;
  workspace_id: string;
  email_account_id: string;
  thread_external_id: string;
  subject: string;
  last_message_at: string;
  message_count: number;
  contact_ids: string[];
  deal_id?: string;
  created_at: string;
  updated_at: string;
  latest_message?: CRMEmailMessage;
  mailbox_email?: string;
  mailbox_provider?: CRMEmailProvider;
  mailbox_status?: CRMEmailAccountStatus;
  mailbox_last_synced_at?: string;
  can_reply?: boolean;
  needs_reply?: boolean;
  needs_reply_dismissed?: boolean;
}

export interface CRMEmailMessage {
  id: string;
  workspace_id: string;
  email_account_id: string;
  thread_id?: string;
  message_external_id: string;
  rfc_message_id?: string;
  in_reply_to?: string;
  references_header?: string;
  from_address: string;
  from_name?: string;
  to_addresses: string[];
  cc_addresses: string[];
  subject: string;
  body_text?: string;
  body_html?: string;
  direction: CRMEmailDirection;
  sent_at: string;
  contact_id?: string;
  contact_ids: string[];
  deal_id?: string;
  created_at: string;
  attachments?: CRMEmailAttachment[];
}

export interface CRMEmailAttachment {
  id: string;
  workspace_id: string;
  draft_id: string;
  message_id?: string;
  uploaded_by_id: string;
  file_name: string;
  file_size: number;
  content_type: string;
  is_uploaded: boolean;
  created_at: string;
}

export interface CRMEmailParticipant {
  email: string;
  name?: string;
  role: "from" | "to" | "cc" | "manual";
  contact_id?: string;
  contact_name?: string;
  company_id?: string;
  company_name?: string;
}

export interface CRMEmailThreadDetail {
  thread: CRMEmailThread;
  messages: CRMEmailMessage[];
  participants: CRMEmailParticipant[];
}

export interface CreateCRMEmailMessageRequest {
  workspace_id: string;
  email_account_id: string;
  thread_id?: string;
  from_address: string;
  from_name?: string;
  to_addresses?: string[];
  cc_addresses?: string[];
  subject: string;
  body_text?: string;
  body_html?: string;
  direction: CRMEmailDirection;
  sent_at?: string;
  contact_id?: string;
  deal_id?: string;
}

export interface CRMCalendarAttendee {
  email: string;
  name?: string;
  response_status?: string;
  organizer?: boolean;
  self?: boolean;
}

export interface CRMCalendarEvent {
  id: string;
  workspace_id: string;
  email_account_id: string;
  external_event_id?: string;
  recurring_series_id?: string;
  auto_join_override?: boolean;
  title: string;
  description?: string;
  start_time: string;
  end_time: string;
  location?: string;
  meeting_url?: string;
  organizer_email?: string;
  status: string;
  visibility: string;
  all_day: boolean;
  attendees: CRMCalendarAttendee[];
  contact_ids: string[];
  deal_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMCalendarEventRequest {
  workspace_id: string;
  email_account_id: string;
  title: string;
  description?: string;
  start_time: string;
  end_time: string;
  location?: string;
  meeting_url?: string;
  attendees?: CRMCalendarAttendee[];
  contact_ids?: string[];
  deal_id?: string;
}

export interface UpdateCRMCalendarEventRequest {
  title?: string;
  description?: string;
  start_time?: string;
  end_time?: string;
  location?: string;
  meeting_url?: string;
  attendees?: CRMCalendarAttendee[];
  contact_ids?: string[];
  deal_id?: string;
}

// ── Unified Activity Feed ──

export type UnifiedActivityItem =
  | { kind: "activity"; data: CRMActivity; timestamp: string; source: "manual" }
  | {
      kind: "email";
      data: CRMEmailMessage;
      timestamp: string;
      source: CRMEmailProvider | "unknown";
    }
  | {
      kind: "calendar";
      data: CRMCalendarEvent;
      timestamp: string;
      source: CRMEmailProvider | "unknown";
    };

// ── Phase 4: AI Intelligence ──

export type CRMEnrichmentSource = "apollo" | "ai" | "manual";

export interface CRMEnrichmentResult {
  id: string;
  workspace_id: string;
  object_type: CRMObjectType;
  object_id: string;
  source: CRMEnrichmentSource;
  data: Record<string, unknown>;
  confidence: number;
  created_at: string;
}

export interface CreateCRMEnrichmentRequest {
  workspace_id: string;
  object_type: CRMObjectType;
  object_id: string;
  source: CRMEnrichmentSource;
  data?: Record<string, unknown>;
  confidence?: number;
}

export type CRMSignalType =
  | "buying_intent"
  | "objection"
  | "competitor_mention"
  | "budget_signal"
  | "timeline_signal"
  | "champion_signal"
  | "risk_signal";

export type CRMSignalSourceType =
  | "email"
  | "meeting"
  | "note"
  | "call"
  | "manual"
  | "support"
  | "crm"
  | "pm"
  | "web_behavior"
  | "product_usage"
  | "external";

export type CRMSignalDomain =
  | "conversation"
  | "web_behavior"
  | "product_usage"
  | "support"
  | "delivery"
  | "relationship"
  | "market";
export type CRMSignalPolarity = "positive" | "negative" | "neutral";
export type CRMSignalDismissalReason =
  | "incorrect_evidence"
  | "wrong_entity"
  | "duplicate"
  | "irrelevant"
  | "handled"
  | "bad_timing";
export type CRMSignalSeverity = "low" | "medium" | "high";
export type CRMCommercialMotion =
  | "prospecting"
  | "conversion"
  | "onboarding"
  | "adoption"
  | "expansion"
  | "renewal"
  | "retention"
  | "needs_context";

export interface CRMSignalMetadata {
  commercial_relevance?: "relevant";
  commercial_event?: string;
  commercial_consequence?: string;
  offering_match?: string;
  customer_relationship?: "customer" | "prospect" | "unknown";
  needs_customer_context?: boolean;
  message_direction?: string;
  participant_count?: number;
  thread_external_id?: string;
  ingestion_version?: string;
  detector_version?: string;
  source_content_hash?: string;
  evidence_verified?: boolean;
  skip_reason?: string;
  mailbox_email?: string;
}

export interface CRMSignal {
  id: string;
  workspace_id: string;
  contact_id?: string;
  deal_id?: string;
  company_id?: string;
  contact_name?: string;
  deal_name?: string;
  deal_display_id?: string;
  account_name?: string;
  account_domain?: string;
  owner_member_id?: string;
  signal_type: CRMSignalType;
  source_type: CRMSignalSourceType;
  source_id?: string;
  source_thread_id?: string;
  summary: string;
  evidence_excerpt?: string;
  metadata?: CRMSignalMetadata;
  confidence: number;
  detected_at: string;
  detector_kind?: "llm_extracted" | "rule_derived" | "manual";
  signal_domain?: CRMSignalDomain;
  polarity?: CRMSignalPolarity;
  rule_key?: string;
  rule_version?: number;
  evidence_identity_method?: string;
  evidence_identity_trust?: string;
  business_priority?: number;
  signed_impact?: number;
  severity?: CRMSignalSeverity;
  score_version?: number;
  score_factors?: Record<string, unknown>;
  evidence_fingerprint?: string;
  observation_id?: string;
  commercial_motion: CRMCommercialMotion;
  interpretation_version: number;
  interpretation_snapshot?: Record<string, unknown>;
  meaning_fingerprint?: string;
  recommended_action_key?: string;
  recommended_action_label?: string;
  replay_calibration_excluded?: boolean;
  superseded_at?: string;
  superseded_reason?: string;
  direction_changed_by_supersession?: boolean;
  dismissed_at?: string;
  dismissed_by_member_id?: string;
  dismissal_reason?: CRMSignalDismissalReason;
  reviewed_at?: string;
  acted_at?: string;
  activation_eligible?: boolean;
  activation_blockers?: string[];
  existing_open_task_id?: string;
  created_at: string;
}

export type CRMExternalEvidenceType =
  | "funding"
  | "hiring"
  | "job_change"
  | "technology"
  | "leadership"
  | "third_party_intent";

export interface CRMSignalExternalEvidence {
  id: string;
  workspace_id: string;
  provider: string;
  provider_evidence_id: string;
  evidence_type: CRMExternalEvidenceType;
  rule_key: string;
  rule_version: number;
  signal_type: CRMSignalType;
  signal_domain: CRMSignalDomain;
  polarity: CRMSignalPolarity;
  summary: string;
  evidence_excerpt?: string;
  source_url?: string;
  contact_id?: string;
  deal_id?: string;
  company_id?: string;
  identity_method: string;
  identity_trust: string;
  provenance: Record<string, unknown>;
  observed_at: string;
  signal_id?: string;
  created_at: string;
}

export type IngestCRMSignalExternalEvidenceRequest = Omit<
  CRMSignalExternalEvidence,
  "id" | "signal_id" | "created_at"
>;

export interface CRMSignalAccountStory {
  id: string;
  entity_type: "company" | "deal" | "contact" | "unresolved";
  entity_id: string;
  account_name: string;
  account_domain?: string;
  owner_member_id?: string;
  commercial_motion: CRMCommercialMotion;
  other_active_motions?: CRMCommercialMotion[];
  priority: number;
  signed_impact: number;
  positive_strength: number;
  negative_strength: number;
  needs_judgment: boolean;
  recommended_action_key?: string;
  recommended_action_label?: string;
  direction_changed_by_supersession: boolean;
  severity: CRMSignalSeverity;
  polarity: CRMSignalPolarity;
  domains: CRMSignalDomain[];
  latest_detected_at: string;
  changed_since: number;
  evidence_source_count: number;
  changed_evidence_source_count: number;
  change_summary: string;
  score_version: number;
  score_factors: Record<string, unknown>;
  signals: CRMSignal[];
}

export interface CRMSignalWorkspaceFeed {
  data: CRMSignalAccountStory[];
  lanes: CRMSignalLane[];
  total: number;
  page: number;
  score_version: number;
  heuristic: boolean;
  minimum_lane_priority: number;
  rollout_mode: "shadow" | "live";
}

export interface CRMSignalLane {
  commercial_motion: CRMCommercialMotion;
  data: CRMSignalAccountStory[];
  total: number;
  page: number;
  per_page: number;
}

export interface CRMSignalRuleConfig {
  id: string;
  workspace_id?: string;
  rule_key: string;
  version: number;
  cadence: "daily" | "micro_batch" | "event_driven";
  enabled: boolean;
  shadow_mode: boolean;
  activation_eligible: boolean;
  business_weight: number;
}

export interface CRMSignalPrecisionRow {
  workspace_id: string;
  rule_key: string;
  rule_version: number;
  signal_domain: CRMSignalDomain;
  identity_method: string;
  reviewed_count: number;
  valid_count: number;
  incorrect_count: number;
  acted_count: number;
  precision: number;
  average_review_millis: number;
  average_action_millis: number;
}

export interface CRMSignalOutcomeCalibrationRow {
  workspace_id: string;
  rule_key: string;
  rule_version: number;
  commercial_motion: CRMCommercialMotion;
  identity_method: string;
  matured_signals: number;
  outcome_matched: number;
  outcome_precision: number;
  horizon_days: number;
}

export interface CRMSignalRoutingSettings {
  workspace_id: string;
  default_signal_owner_member_id?: string;
  minimum_lane_priority: number;
}

export interface CRMSignalRolloutSettings {
  workspace_id: string;
  mode: "shadow" | "live";
  activated_at?: string;
  activated_by_member_id?: string;
}

export interface CRMSignalShadowGate {
  eligible: boolean;
  observation_count: number;
  unmapped_observation_rate: number;
  duplicate_fingerprint_rate: number;
  immutable_meaning_violations: number;
}

export interface CRMSignalRoutingPolicy {
  id: string;
  workspace_id: string;
  version: number;
  enabled: boolean;
  minimum_priority: number;
  required_trust: string;
  route_to_owner: boolean;
  destination_team_id?: string;
  channels: string[];
  created_at: string;
}

export interface CRMSignalFeedFilters {
  include_context?: boolean;
  owner_member_id?: string;
  account_id?: string;
  domain?: CRMSignalDomain;
  polarity?: CRMSignalPolarity;
  motion?: CRMCommercialMotion;
  severity?: CRMSignalSeverity;
  trust?: string;
  status?: "active" | "dismissed" | "superseded" | "all";
  max_age_days?: number;
  page?: number;
  per_page?: number;
  filters?: string;
  lane_pages?: Partial<Record<CRMCommercialMotion, number>>;
}

export interface CreateCRMSignalRequest {
  workspace_id: string;
  contact_id?: string;
  deal_id?: string;
  company_id?: string;
  signal_type: CRMSignalType;
  summary: string;
  evidence_excerpt?: string;
  metadata?: Record<string, unknown>;
  confidence?: number;
}

export type CRMEntitySummaryStatus =
  | "pending_refresh"
  | "ready"
  | "stale"
  | "error";
export type CRMSummaryHighlightKind =
  | "momentum"
  | "risk"
  | "next_step"
  | "stakeholder"
  | "signal";

export interface SummaryHighlight {
  kind: CRMSummaryHighlightKind;
  text: string;
}

export interface CRMEntitySummaryMetadata {
  source_email_count?: number;
  source_signal_count?: number;
  generation_version?: string;
  source_contact_count?: number;
  source_deal_count?: number;
  source_task_count?: number;
  source_support_count?: number;
  source_activity_count?: number;
  source_activity_counts?: Record<string, number>;
  source_window_days?: number;
}

export interface CRMSummaryReadinessStep {
  key: string;
  label: string;
  complete: boolean;
}

export interface CRMSummaryReadiness {
  count: number;
  threshold: number;
  ready: boolean;
  steps: CRMSummaryReadinessStep[];
}

export interface CRMSummarySource {
  key: string;
  type: string;
  source_id: string;
  thread_id?: string;
  entity_type?: string;
  entity_id?: string;
  label: string;
  occurred_at: string;
  email_account_id?: string;
}

export interface CRMEntitySummary {
  id: string;
  workspace_id: string;
  entity_type: "contact" | "company" | "deal";
  entity_id: string;
  summary_markdown: string;
  highlights: SummaryHighlight[];
  status: CRMEntitySummaryStatus;
  computed_at?: string;
  source_window_start?: string;
  source_window_end?: string;
  last_triggered_at?: string;
  last_error?: string;
  metadata?: CRMEntitySummaryMetadata;
  readiness?: CRMSummaryReadiness;
  sources?: CRMSummarySource[];
  next_step?: string;
  created_at: string;
  updated_at: string;
}

export interface CRMIntelligenceRefreshResult {
  summary: CRMEntitySummary;
  sources_analyzed: number;
  signals_detected: number;
  warnings: string[];
}

export interface CRMDealHealthScore {
  id: string;
  workspace_id: string;
  deal_id: string;
  score: number;
  factors: Record<string, unknown>;
  calculated_at: string;
  created_at: string;
}

export interface CreateCRMDealHealthScoreRequest {
  workspace_id: string;
  deal_id: string;
  score: number;
  factors?: Record<string, unknown>;
}

export type CRMSuggestionType =
  | "playbook_action"
  | "follow_up"
  | "deal_create"
  | "deal_advance"
  | "enrichment"
  | "risk_alert";

export type CRMSuggestionStatus = "pending" | "accepted" | "dismissed" | "superseded" | "expired";

export interface CRMSuggestion {
  id: string;
  workspace_id: string;
  user_id?: string;
  suggestion_type: CRMSuggestionType;
  object_type?: CRMObjectType;
  object_id?: string;
  title: string;
  description?: string;
  context: Record<string, unknown>;
  signal_ids?: string[];
  signals?: CRMSignal[];
  status: CRMSuggestionStatus;
  dismissal_reason?: CRMSignalDismissalReason;
  confidence: number;
  execution_status?: "pending" | "in_progress" | "manual_required" | "succeeded" | "failed";
  revision?: string;
  assignee_member_id?: string | null;
  assignee_available?: boolean;
  executed_at?: string;
  execution_error?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMSuggestionRequest {
  workspace_id: string;
  user_id?: string;
  suggestion_type: CRMSuggestionType;
  object_type?: CRMObjectType;
  object_id?: string;
  title: string;
  description?: string;
  context?: Record<string, unknown>;
  confidence?: number;
}

export interface UpdateCRMSuggestionRequest {
  status?: CRMSuggestionStatus;
}

// ── Phase 5: Writing ──

export interface CRMWritingProfile {
  id: string;
  workspace_id: string;
  member_id: string;
  style_attributes: Record<string, unknown>;
  sample_count: number;
  last_analyzed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMWritingProfileRequest {
  workspace_id: string;
  member_id: string;
  style_attributes?: Record<string, unknown>;
}

export interface UpdateCRMWritingProfileRequest {
  style_attributes?: Record<string, unknown>;
  sample_count?: number;
}

// ── Email Sync Settings ──

export type CRMFilterMode = "blocklist" | "allowlist";
export type CRMRecordCreationMode = "disabled" | "selective" | "always";
export type CRMInternalExclusion = "none" | "exclude";

export interface CRMEmailSyncSettings {
  id: string;
  workspace_id: string;
  historical_sync_days: number;
  filter_mode: CRMFilterMode;
  filter_patterns: string[];
  internal_exclusion: CRMInternalExclusion;
  include_private_meetings: boolean;
  include_solo_meetings: boolean;
  record_creation_mode: CRMRecordCreationMode;
  blocked_record_prefixes: string[];
  created_at: string;
  updated_at: string;
}

export interface UpdateCRMEmailSyncSettingsRequest {
  historical_sync_days?: number;
  filter_mode?: CRMFilterMode;
  filter_patterns?: string[];
  internal_exclusion?: CRMInternalExclusion;
  include_private_meetings?: boolean;
  include_solo_meetings?: boolean;
  record_creation_mode?: CRMRecordCreationMode;
  blocked_record_prefixes?: string[];
}

// ── Phase 6: Search ──

export interface CRMSearchResult {
  type: CRMObjectType;
  id: string;
  name: string;
  detail: string;
  object: CRMContact | CRMCompany | CRMDeal;
}
