import type { ConversationStatus } from './support';

export interface VisitorDeviceInfo {
  browser: string;
  browser_version: string;
  os: string;
  os_version: string;
  device_type: string;
}

export interface VisitorLocation {
  timezone: string | null;
  locale: string | null;
  last_page_url: string | null;
}

export interface VisitorContactData {
  id: string;
  name: string | null;
  email: string | null;
  phone: string | null;
  job_title: string | null;
  lifecycle_stage: string;
  lead_status: string;
  source: string;
  custom_properties?: Record<string, string>;
}

export interface VisitorOtherConversation {
  id: string;
  display_id: number;
  subject: string;
  status: ConversationStatus;
  created_at: string;
}

export interface VisitorContextResponse {
  device: VisitorDeviceInfo | null;
  location: VisitorLocation | null;
  contact: VisitorContactData | null;
  other_conversations: VisitorOtherConversation[];
  total_conversations: number;
  session_created_at: string | null;
}
