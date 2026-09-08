import { describe, expect, it } from 'vitest';
import { actionStatus, approvalLabel, dismissalReasons, signalCategories, signalStatus } from '../crmSituationPresentation';
import type { CRMSuggestion } from '../crmTypes';
import type { CRMSituationItem } from '../crmSituationTypes';

describe('Signal presentation', () => {
  it('keeps four business categories plus All', () => {
    expect(signalCategories.map((category) => category.label)).toEqual(['All', 'Sales', 'Onboarding & adoption', 'Expansion', 'Retention']);
    expect(dismissalReasons).toHaveLength(6);
  });
  it.each([
    ['manual_required', 'Approved — follow-through needed'], ['succeeded', 'Completed'], ['failed', 'Action failed'],
    ['in_progress', 'Running — do not repeat'], [undefined, 'Result unknown — check before repeating'],
  ])('does not confuse approval with execution: %s', (execution_status, label) => {
    expect(actionStatus({ status: 'accepted', execution_status, suggestion_type: 'deal_create' } as CRMSuggestion)).toBe(label);
  });
  it.each([['deal_create', 'Create deal'], ['deal_advance', 'Change stage'], ['follow_up', 'Approve'], ['risk_alert', 'Approve'], ['enrichment', 'Approve']])('uses truthful action labels for %s', (suggestion_type, label) => {
    expect(approvalLabel({ suggestion_type } as CRMSuggestion)).toBe(label);
  });
  it('preserves closed state ahead of action summaries and uncertainty ahead of approvals', () => {
    const item = { situation: { lifecycle: 'closed' }, uncertain_action_count: 1, pending_action_count: 2 } as CRMSituationItem;
    expect(signalStatus(item)).toBe('Closed');
    item.situation.lifecycle = 'open';
    expect(signalStatus(item)).toBe('Result unknown');
  });
  it.each(['follow_up', 'risk_alert', 'enrichment'])('does not call historical manual %s approvals completed', (suggestion_type) => {
    expect(actionStatus({ status: 'accepted', execution_status: 'succeeded', suggestion_type } as CRMSuggestion)).toBe('Approved — follow-through needed');
  });
});
