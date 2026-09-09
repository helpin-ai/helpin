import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { crmSignalInboxService as service } from '../crmSignalInboxService';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
describe('CRM signal inbox transport', () => {
  beforeEach(() => vi.clearAllMocks());
  it('shows everyone by default while preserving an explicit personal filter', () => {
    service.list('ws', {});
    expect(new URL(vi.mocked(api.get).mock.calls[0][0], 'https://local.test').searchParams.get('scope')).toBe('all');
    service.list('ws', { scope: 'mine' });
    expect(new URL(vi.mocked(api.get).mock.calls[1][0], 'https://local.test').searchParams.get('scope')).toBe('mine');
  });
  it('reads evidence through its own endpoint without creating tracked work', () => {
    service.evidence('ws', 'group-id');
    expect(api.get).toHaveBeenCalledWith('/crm/signal-inbox/evidence/group-id?workspace_id=ws');
    expect(api.post).not.toHaveBeenCalled();
  });
  it('uses server pagination and visible filters without sending retired filters', () => {
    service.list('ws', { scope: 'all', state: 'needs_approval', sort: 'newest', page: 2 });
    const url = new URL(vi.mocked(api.get).mock.calls[0][0], 'https://local.test');
    expect(url.pathname).toBe('/crm/signal-inbox');
    expect(url.searchParams.get('page')).toBe('2');
    expect(url.searchParams.get('state')).toBe('needs_approval');
    expect(url.searchParams.get('sort')).toBe('newest');
    expect(url.searchParams.has('filter')).toBe(false);
  });
  it('normalizes legacy sort and filter values at the transport boundary', () => {
    const legacy = { priority: 'high', evidence_review: 'needs_review', sort: 'recommended', page: 3 };
    service.list('ws', legacy);
    const params = new URL(vi.mocked(api.get).mock.calls[0][0], 'https://local.test').searchParams;
    expect(params.get('sort')).toBe('priority');
    expect(params.get('page')).toBe('1');
    expect(params.has('filter')).toBe(false);
  });
  it('never sends a proposal revision as legacy context edits', () => {
    service.accept('ws', 'proposal', 'shown-revision');
    expect(api.post).toHaveBeenCalledWith('/crm/signal-inbox/recommendations/proposal/accept?workspace_id=ws', { revision: 'shown-revision' });
    service.dismiss('ws', 'proposal', 'shown-revision', 'wrong_entity');
    expect(api.post).toHaveBeenLastCalledWith('/crm/signal-inbox/recommendations/proposal/dismiss?workspace_id=ws', { revision: 'shown-revision', reason: 'wrong_entity' });
  });
});
