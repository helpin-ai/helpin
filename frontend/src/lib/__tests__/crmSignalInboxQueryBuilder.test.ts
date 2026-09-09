import { describe, expect, it } from 'vitest';
import { inboxNavigation, inboxStatus, parseSignalsSearch, reviewSignalsSearch } from '../crmSignalInboxQueryBuilder';
import type { CRMSignalInboxItem } from '../crmSignalInboxTypes';

describe('Signals navigation', () => {
  it('preserves an evidence-group deep link and distinguishes review from execution', () => {
    expect(parseSignalsSearch({ group: 'abc123', scope: 'mine' })).toEqual({ group: 'abc123', scope: 'mine' });
    expect(inboxStatus({ kind: 'evidence', evidence_review: 'needs_review' } as CRMSignalInboxItem)).toBe('Evidence to review');
    expect(inboxStatus({ kind: 'evidence', evidence_review: 'reviewed' } as CRMSignalInboxItem)).toBe('Evidence reviewed');
  });
  it('preserves shareable filters and either kind of drawer', () => {
    const search = { scope: 'all', state: 'needs_approval', category: 'retention', sort: 'newest', q: 'Northstar', page: 3, recommendation: 'proposal-id' };
    expect(parseSignalsSearch(search)).toEqual(search);
  });
  it('drops invalid enum values and unsafe pages while bounding text', () => {
    expect(parseSignalsSearch({ scope: 'foreign', state: 'bad', page: Infinity, signal: 'x'.repeat(101), q: 'x'.repeat(501) })).toEqual({ q: 'x'.repeat(500) });
    expect(parseSignalsSearch({ page: '2' }).page).toBe(2);
  });
  it('old Review opens all assignments awaiting approval and preserves deep links', () => {
    expect(reviewSignalsSearch({ recommendation: 'proposal-id', category: 'expansion' })).toEqual({ recommendation: 'proposal-id', category: 'expansion', view: undefined, scope: 'all', state: 'needs_approval', sort: 'priority' });
  });
  it('drops retired URL filters and restarts pagination without losing the drawer or visible filters', () => {
    for (const key of ['priority', 'evidence_review', 'attention', 'action_type', 'filter']) {
      expect(parseSignalsSearch({ [key]: 'legacy', scope: 'mine', category: 'expansion', sort: 'oldest', group: 'group-id', page: 4 })).toEqual({ scope: 'mine', category: 'expansion', sort: 'oldest', group: 'group-id', page: 1 });
    }
    expect(parseSignalsSearch({ sort: 'recommended', state: 'needs_approval', page: 4 })).toEqual({ state: 'needs_approval', page: 1 });
  });
  it('offers only priority and chronological sorting without changing the priority algorithm', () => {
    expect(inboxNavigation.sort).toEqual([{ value: 'priority', label: 'High priority' }, { value: 'newest', label: 'Newest first' }, { value: 'oldest', label: 'Oldest first' }]);
  });
  it('does not conceal pending approvals behind lifecycle or execution problems', () => {
    expect(inboxStatus({ lifecycle: 'open', attention: 'automation_failed', pending_action_count: 1 } as CRMSignalInboxItem)).toBe('Action needs attention · Approval pending');
    expect(inboxStatus({ lifecycle: 'paused', attention: 'needs_approval', pending_action_count: 1 } as CRMSignalInboxItem)).toBe('Paused · Approval pending');
  });
});
