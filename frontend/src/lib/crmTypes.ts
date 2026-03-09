// CRM Module TypeScript interfaces

export type LifecycleStage =
  | 'subscriber'
  | 'lead'
  | 'marketing_qualified'
  | 'sales_qualified'
  | 'opportunity'
  | 'customer'
  | 'evangelist';

export type LeadStatus = 'new' | 'open' | 'in_progress' | 'unqualified';

export type CRMActivityType = 'note' | 'call' | 'meeting' | 'email' | 'task';

export type PipelineStageType = 'open' | 'won' | 'lost';

export type CRMObjectType = 'contact' | 'company' | 'deal' | 'epic' | 'story';

export interface CRMContact {
  id: string;
  workspace_id: string;
  display_id: string;
  first_name: string;
  last_name?: string;
  email?: string;
  phone?: string;
  job_title?: string;
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
  lifecycle_stage?: LifecycleStage;
  lead_status?: LeadStatus;
  owner_member_id?: string;
  avatar_url?: string;
  source?: string;
  custom_properties?: Record<string, unknown>;
}

export interface CRMCompany {
  id: string;
  workspace_id: string;
  display_id: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  owner_member_id?: string;
  custom_properties: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMCompanyRequest {
  workspace_id: string;
  name: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  owner_member_id?: string;
  custom_properties?: Record<string, unknown>;
}

export interface UpdateCRMCompanyRequest {
  name?: string;
  domain?: string;
  industry?: string;
  employee_count?: number;
  annual_revenue?: number;
  description?: string;
  logo_url?: string;
  owner_member_id?: string;
  custom_properties?: Record<string, unknown>;
}

export interface CRMPipelineStage {
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
  position: number;
  stages?: CRMPipelineStage[];
  created_at: string;
  updated_at: string;
}

export interface CreateCRMPipelineRequest {
  workspace_id: string;
  name: string;
  is_default?: boolean;
  stages?: {
    name: string;
    stage_type: PipelineStageType;
    position: number;
    probability: number;
  }[];
}

export interface UpdateCRMPipelineRequest {
  name?: string;
  is_default?: boolean;
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
  pipeline_id: string;
  stage_id: string;
  amount?: number;
  currency?: string;
  close_date?: string;
  owner_member_id?: string;
  probability?: number;
  custom_properties?: Record<string, unknown>;
}

export interface UpdateCRMDealRequest {
  name?: string;
  pipeline_id?: string;
  stage_id?: string;
  amount?: number;
  currency?: string;
  close_date?: string;
  owner_member_id?: string;
  probability?: number;
  custom_properties?: Record<string, unknown>;
}

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

// ── Phase 2: Properties, Lists, Import ──

export type CRMFieldType =
  | 'text'
  | 'number'
  | 'date'
  | 'select'
  | 'multiselect'
  | 'boolean'
  | 'url'
  | 'email'
  | 'phone'
  | 'currency';

export interface CRMPropertyDefinition {
  id: string;
  workspace_id: string;
  object_type: CRMObjectType;
  internal_name: string;
  label: string;
  field_type: CRMFieldType;
  options: Record<string, unknown>;
  group_name?: string;
  is_required: boolean;
  is_system: boolean;
  position: number;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMPropertyDefinitionRequest {
  workspace_id: string;
  object_type: CRMObjectType;
  internal_name: string;
  label: string;
  field_type: CRMFieldType;
  options?: Record<string, unknown>;
  group_name?: string;
  is_required?: boolean;
  position?: number;
}

export interface UpdateCRMPropertyDefinitionRequest {
  label?: string;
  field_type?: CRMFieldType;
  options?: Record<string, unknown>;
  group_name?: string;
  is_required?: boolean;
  position?: number;
}

export interface CRMPropertyGroup {
  id: string;
  workspace_id: string;
  object_type: CRMObjectType;
  name: string;
  position: number;
  is_system: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMPropertyGroupRequest {
  workspace_id: string;
  object_type: CRMObjectType;
  name: string;
  position?: number;
}

export interface UpdateCRMPropertyGroupRequest {
  name?: string;
  position?: number;
}

export type CRMListType = 'static' | 'smart';

export interface CRMList {
  id: string;
  workspace_id: string;
  name: string;
  list_type: CRMListType;
  object_type: CRMObjectType;
  filter_criteria: Record<string, unknown>;
  member_count: number;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMListRequest {
  workspace_id: string;
  name: string;
  list_type: CRMListType;
  object_type: CRMObjectType;
  filter_criteria?: Record<string, unknown>;
}

export interface UpdateCRMListRequest {
  name?: string;
  filter_criteria?: Record<string, unknown>;
}

export interface CRMListMember {
  id: string;
  list_id: string;
  object_id: string;
  created_at: string;
}

export type CRMImportSource = 'csv' | 'hubspot';
export type CRMImportStatus = 'pending' | 'processing' | 'completed' | 'failed';

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

export type CRMEmailProvider = 'gmail' | 'microsoft';
export type CRMEmailDirection = 'inbound' | 'outbound';

export interface CRMEmailAccount {
  id: string;
  workspace_id: string;
  member_id: string;
  provider: CRMEmailProvider;
  email_address: string;
  sync_state: Record<string, unknown>;
  last_synced_at?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
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
  contact_ids: Record<string, unknown>;
  deal_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CRMEmailMessage {
  id: string;
  workspace_id: string;
  email_account_id: string;
  thread_id?: string;
  message_external_id: string;
  from_address: string;
  from_name?: string;
  to_addresses: Record<string, unknown>;
  cc_addresses: Record<string, unknown>;
  subject: string;
  body_text?: string;
  body_html?: string;
  direction: CRMEmailDirection;
  sent_at: string;
  contact_id?: string;
  deal_id?: string;
  created_at: string;
}

export interface CreateCRMEmailMessageRequest {
  workspace_id: string;
  email_account_id: string;
  thread_id?: string;
  from_address: string;
  from_name?: string;
  to_addresses?: Record<string, unknown>;
  cc_addresses?: Record<string, unknown>;
  subject: string;
  body_text?: string;
  body_html?: string;
  direction: CRMEmailDirection;
  sent_at?: string;
  contact_id?: string;
  deal_id?: string;
}

export interface CRMCalendarEvent {
  id: string;
  workspace_id: string;
  email_account_id: string;
  external_event_id?: string;
  title: string;
  description?: string;
  start_time: string;
  end_time: string;
  location?: string;
  attendees: Record<string, unknown>;
  contact_ids: Record<string, unknown>;
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
  attendees?: Record<string, unknown>;
  contact_ids?: Record<string, unknown>;
  deal_id?: string;
}

export interface UpdateCRMCalendarEventRequest {
  title?: string;
  description?: string;
  start_time?: string;
  end_time?: string;
  location?: string;
  attendees?: Record<string, unknown>;
  contact_ids?: Record<string, unknown>;
  deal_id?: string;
}

// ── Phase 4: AI Intelligence ──

export type CRMEnrichmentSource = 'apollo' | 'ai' | 'manual';

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
  | 'buying_intent'
  | 'objection'
  | 'competitor_mention'
  | 'budget_signal'
  | 'timeline_signal'
  | 'champion_signal'
  | 'risk_signal';

export type CRMSignalSourceType = 'email' | 'meeting' | 'note' | 'manual';

export interface CRMBuyerSignal {
  id: string;
  workspace_id: string;
  contact_id?: string;
  deal_id?: string;
  signal_type: CRMSignalType;
  source_type: CRMSignalSourceType;
  source_id?: string;
  summary: string;
  confidence: number;
  detected_at: string;
  created_at: string;
}

export interface CreateCRMBuyerSignalRequest {
  workspace_id: string;
  contact_id?: string;
  deal_id?: string;
  signal_type: CRMSignalType;
  source_type?: CRMSignalSourceType;
  source_id?: string;
  summary: string;
  confidence?: number;
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
  | 'follow_up'
  | 'deal_create'
  | 'deal_advance'
  | 'enrichment'
  | 'risk_alert';

export type CRMSuggestionStatus = 'pending' | 'accepted' | 'dismissed';

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
  status: CRMSuggestionStatus;
  confidence: number;
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

// ── Phase 5: Sequences & Writing ──

export type CRMSequenceStatus = 'draft' | 'active' | 'paused';
export type CRMEnrollmentStatus =
  | 'active'
  | 'completed'
  | 'paused'
  | 'bounced'
  | 'unsubscribed'
  | 'exited';

export interface CRMSequence {
  id: string;
  workspace_id: string;
  name: string;
  description?: string;
  status: CRMSequenceStatus;
  steps: Record<string, unknown>;
  enrollment_count: number;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMSequenceRequest {
  workspace_id: string;
  name: string;
  description?: string;
  status?: CRMSequenceStatus;
  steps?: Record<string, unknown>;
}

export interface UpdateCRMSequenceRequest {
  name?: string;
  description?: string;
  status?: CRMSequenceStatus;
  steps?: Record<string, unknown>;
}

export interface CRMSequenceEnrollment {
  id: string;
  workspace_id: string;
  sequence_id: string;
  contact_id: string;
  current_step: number;
  status: CRMEnrollmentStatus;
  enrolled_at: string;
  completed_at?: string;
  exit_reason?: string;
  last_step_executed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCRMSequenceEnrollmentRequest {
  workspace_id: string;
  sequence_id: string;
  contact_id: string;
}

export interface UpdateCRMSequenceEnrollmentRequest {
  status?: CRMEnrollmentStatus;
  exit_reason?: string;
}

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

// ── Phase 6: Search ──

export interface CRMSearchResult {
  type: CRMObjectType;
  id: string;
  name: string;
  detail: string;
  object: CRMContact | CRMCompany | CRMDeal;
}
