import type { CRMSuggestion } from './crmTypes';
import type { CRMSituationAttention, CRMSituationCategory, CRMSituationLifecycle } from './crmSituationTypes';

export interface CRMSignalInboxItem {
  id: string;
  kind: 'situation' | 'recommendation';
  title: string;
  next_step: string;
  category: CRMSituationCategory | '';
  customer_name: string;
  owner_name: string;
  owner_member_id: string | null;
  owner_available: boolean;
  priority: number | null;
  priority_band: 'high' | 'medium' | 'low' | 'unscored';
  lifecycle: CRMSituationLifecycle;
  attention: CRMSituationAttention;
  pending_action_count: number;
  evidence_review: 'needs_review' | 'reviewed' | 'none';
  created_at: string;
}
export interface CRMSignalInboxList {
  data: CRMSignalInboxItem[];
  total: number;
  page: number;
  page_size: number;
  category_counts: Record<CRMSituationCategory | 'all', number>;
  uncategorized_count: number;
}
export interface CRMInboxRecommendation {
  action: CRMSuggestion;
  linked_situations: string[];
}
