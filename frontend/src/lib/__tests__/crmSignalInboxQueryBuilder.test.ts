import { describe, expect, it } from 'vitest';
import { inboxQueryFilter, inboxStatus, parseSignalsSearch, reviewSignalsSearch } from '../crmSignalInboxQueryBuilder';
import type { CRMSignalInboxItem } from '../crmSignalInboxTypes';

describe('Signals navigation', () => {
  it('preserves an evidence-group deep link and distinguishes review from execution', () => {
    expect(parseSignalsSearch({ group: 'abc123', scope: 'mine' })).toEqual({ group: 'abc123', scope: 'mine' });
    expect(inboxStatus({ kind: 'evidence', evidence_review: 'needs_review' } as CRMSignalInboxItem)).toBe('Evidence to review');
    expect(inboxStatus({ kind: 'evidence', evidence_review: 'reviewed' } as CRMSignalInboxItem)).toBe('Evidence reviewed');
  });
  it('preserves shareable filters and either kind of drawer', () => {
    const search = { scope: 'all', state: 'needs_approval', category: 'retention', priority: 'high', evidence_review: 'needs_review', action_type: 'risk_alert', sort: 'recommended', q: 'Northstar', page: 3, recommendation: 'proposal-id' };
    expect(parseSignalsSearch(search)).toEqual(search);
  });
  it('drops invalid enum values and unsafe pages while bounding text', () => {
    expect(parseSignalsSearch({ scope: 'foreign', state: 'bad', priority: 'critical', page: Infinity, signal: 'x'.repeat(101), q: 'x'.repeat(501) })).toEqual({ q: 'x'.repeat(500) });
    expect(parseSignalsSearch({ page: '2' }).page).toBe(2);
  });
  it('old Review opens all assignments awaiting approval and preserves deep links', () => {
    expect(reviewSignalsSearch({ recommendation: 'proposal-id', category: 'expansion' })).toEqual({ recommendation: 'proposal-id', category: 'expansion', view: undefined, scope: 'all', state: 'needs_approval', sort: 'recommended' });
  });
  it('builds only metadata-backed structured filters, not separate page operators', () => {
    expect(inboxQueryFilter({ priority: 'unscored', evidence_review: 'none', action_type: 'enrichment' })).toEqual({ logic: 'and', rules: [{ field: 'priority', operator: 'is', value: 'unscored' }, { field: 'evidence_review', operator: 'is', value: 'none' }, { field: 'has_enrichment', operator: 'is', value: 'yes' }] });
    expect(inboxQueryFilter({ priority: 'fabricated', action_type: 'unsafe' })).toBeUndefined();
  });
  it('does not conceal pending approvals behind lifecycle or execution problems', () => {
    expect(inboxStatus({ lifecycle: 'open', attention: 'automation_failed', pending_action_count: 1 } as CRMSignalInboxItem)).toBe('Action needs attention · Approval pending');
    expect(inboxStatus({ lifecycle: 'paused', attention: 'needs_approval', pending_action_count: 1 } as CRMSignalInboxItem)).toBe('Paused · Approval pending');
  });
});
