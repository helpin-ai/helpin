import type { ConversationStatus } from './support-types'

export interface VisitorDeviceInfo {
  browser: string
  browser_version: string
  os: string
  os_version: string
  device_type: string
}

export interface VisitorLocation {
  timezone: string | null
  locale: string | null
  last_page_url: string | null
  country_code: string | null
  country_name: string | null
  region_name: string | null
  city_name: string | null
}

export interface VisitorContactData {
  id: string
  name: string | null
  email: string | null
  phone: string | null
  job_title: string | null
  lifecycle_stage: string
  lead_status: string
  source: string
  custom_properties?: Record<string, unknown>
}
export interface VisitorCompanyData {
  id: string
  display_id: string
  external_id?: string | null
  name: string
  domain?: string | null
  industry?: string | null
  employee_count?: number | null
  annual_revenue?: number | null
  description?: string | null
  logo_url?: string | null
  custom_properties: Record<string, unknown>
  updated_at: string
}

export interface VisitorCompanyOption {
  id: string
  display_id: string
  name: string
  domain?: string | null
  logo_url?: string | null
}


export interface VisitorOtherConversation {
  id: string
  display_id: number
  subject: string
  status: ConversationStatus
  created_at: string
}

export interface VisitorContextResponse {
  device: VisitorDeviceInfo | null
  company?: VisitorCompanyData | null
  company_options?: VisitorCompanyOption[]
  company_context_status?: 'ok' | 'unlinked' | 'error'
  location: VisitorLocation | null
  contact: VisitorContactData | null
  other_conversations: VisitorOtherConversation[]
  total_conversations: number
  last_active_at?: string | null
  last_active_source?: 'anonymous_id' | 'crm_contact' | string | null
  session_created_at: string | null
}
