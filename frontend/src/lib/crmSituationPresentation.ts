import type { CRMSignalDismissalReason, CRMSuggestion } from './crmTypes';
import type { CRMSituationCategory, CRMSituationItem } from './crmSituationTypes';
import { attentionLabels } from './crmPlaybookPresentation';

export const signalCategories: { value: 'all' | CRMSituationCategory; label: string }[] = [
  { value: 'all', label: 'All' }, { value: 'sales', label: 'Sales' },
  { value: 'onboarding_adoption', label: 'Onboarding & adoption' },
  { value: 'expansion', label: 'Expansion' }, { value: 'retention', label: 'Retention' },
];
export function signalStatus(item: CRMSituationItem) {
  if (item.situation.lifecycle === 'closed') return 'Closed';
  if (item.situation.lifecycle === 'paused') return 'Paused';
  if (item.uncertain_action_count) return 'Result unknown';
  if (item.failed_action_count) return 'Action failed';
  if (item.executing_action_count) return 'Action running';
  if (item.pending_action_count) return 'Needs approval';
  if (item.manual_action_count) return 'Follow-through needed';
  return attentionLabels[item.effective_attention];
}
export const dismissalReasons: { value: CRMSignalDismissalReason; label: string }[] = [
  { value: 'incorrect_evidence', label: 'Evidence is incorrect' }, { value: 'wrong_entity', label: 'Wrong person or account' },
  { value: 'duplicate', label: 'Duplicate recommendation' }, { value: 'irrelevant', label: 'Not relevant' },
  { value: 'handled', label: 'Already handled' }, { value: 'bad_timing', label: 'Bad timing' },
];
export function approvalLabel(action: CRMSuggestion) {
  return action.suggestion_type === 'deal_create' ? 'Create deal' : action.suggestion_type === 'deal_advance' ? 'Change stage' : 'Approve';
}
export function actionStatus(action: CRMSuggestion) {
  if (action.status === 'superseded') return 'Replaced — context changed';
  if (action.status === 'expired') return 'Approval expired';
  if (action.status === 'dismissed') return 'Dismissed';
  if (action.status === 'pending') return 'Needs approval';
  if (actionRequiresFollowThrough(action)) return 'Approved — follow-through needed';
  switch (action.execution_status) {
    case 'succeeded': return 'Completed';
    case 'failed': return 'Action failed';
    case 'in_progress': return action.execution_error ? 'Result needs checking' : 'Running — do not repeat';
    case 'manual_required': return 'Approved — follow-through needed';
    default: return 'Result unknown — check before repeating';
  }
}
// Older recommendations marked manual approvals as succeeded. Do not claim
// that a message was sent or a record changed for those historical rows.
export function actionRequiresFollowThrough(action: CRMSuggestion) {
  return action.status === 'accepted' && (action.execution_status === 'manual_required' ||
    action.execution_status === 'succeeded' && !['deal_create', 'deal_advance', 'playbook_action'].includes(action.suggestion_type));
}
export function contextText(value: unknown) {
  return typeof value === 'string' && value.trim() ? value.trim() : typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString() : '';
}
