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

export type CRMObjectType = 'contact' | 'company' | 'deal';

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
