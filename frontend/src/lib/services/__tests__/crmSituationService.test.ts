import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { crmSituationService } from '../crmSituationService';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
beforeEach(() => vi.clearAllMocks());
describe('Signals API contracts', () => {
  it('sends scoped server-side filters and pagination without mutating on reads', async () => {
    const filter = JSON.stringify({ logic: 'and', rules: [{ field: 'attention', operator: 'is', value: 'needs_approval' }] });
    await crmSituationService.list('ws&1', { category: 'expansion', scope: 'my_teams', state: 'open', q: 'a & b', page: 2, filter });
    const url = new URL(vi.mocked(api.get).mock.calls[0][0], 'http://test');
    expect(Object.fromEntries(url.searchParams)).toEqual({ workspace_id: 'ws&1', category: 'expansion', scope: 'my_teams', state: 'open', q: 'a & b', page: '2', page_size: '25', filter });
    expect(api.post).not.toHaveBeenCalled();
  });
  it('binds approval and dismissal to the exact displayed action revision', async () => {
    await crmSituationService.accept('ws', 'signal', 'action', 'revision-shown');
    expect(api.post).toHaveBeenLastCalledWith('/crm/situations/signal/actions/action/accept?workspace_id=ws', { revision: 'revision-shown' });
    await crmSituationService.dismiss('ws', 'signal', 'action', 'revision-shown', 'wrong_entity');
    expect(api.post).toHaveBeenLastCalledWith('/crm/situations/signal/actions/action/dismiss?workspace_id=ws', { revision: 'revision-shown', reason: 'wrong_entity' });
  });
  it('uses the lifecycle history cursor, not Playbooks history pagination', async () => {
    await crmSituationService.history('ws', 'signal', 40);
    expect(api.get).toHaveBeenCalledWith('/crm/situations/signal/history?workspace_id=ws&before_revision=40&limit=50');
  });
});
