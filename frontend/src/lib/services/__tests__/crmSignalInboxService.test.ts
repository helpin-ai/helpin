import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { crmSignalInboxService as service } from '../crmSignalInboxService';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
describe('CRM signal inbox transport', () => {
  beforeEach(() => vi.clearAllMocks());
  it('uses a server-paginated union with structured filters', () => {
    service.list('ws', { scope: 'all', state: 'needs_approval', priority: 'unscored', action_type: 'enrichment', sort: 'recommended', page: 2 });
    const url = new URL(vi.mocked(api.get).mock.calls[0][0], 'https://local.test');
    expect(url.pathname).toBe('/crm/signal-inbox');
    expect(url.searchParams.get('page')).toBe('2');
    expect(url.searchParams.get('state')).toBe('needs_approval');
    expect(JSON.parse(url.searchParams.get('filter')!)).toEqual({ logic: 'and', rules: [{ field: 'priority', operator: 'is', value: 'unscored' }, { field: 'has_enrichment', operator: 'is', value: 'yes' }] });
  });
  it('never sends a proposal revision as legacy context edits', () => {
    service.accept('ws', 'proposal', 'shown-revision');
    expect(api.post).toHaveBeenCalledWith('/crm/signal-inbox/recommendations/proposal/accept?workspace_id=ws', { revision: 'shown-revision' });
    service.dismiss('ws', 'proposal', 'shown-revision', 'wrong_entity');
    expect(api.post).toHaveBeenLastCalledWith('/crm/signal-inbox/recommendations/proposal/dismiss?workspace_id=ws', { revision: 'shown-revision', reason: 'wrong_entity' });
  });
});
