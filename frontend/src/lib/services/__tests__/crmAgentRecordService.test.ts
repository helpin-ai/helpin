import { describe, expect, it, vi, beforeEach } from 'vitest';
import { getCRMAgentRecord, listCRMAgentRecords } from '../crmAgentRecordService';
import { crmCompanyService, crmContactService, crmDealService } from '../crmService';

vi.mock('../crmService', () => ({
  crmContactService: { list: vi.fn(), get: vi.fn() },
  crmCompanyService: { list: vi.fn(), get: vi.fn() },
  crmDealService: { list: vi.fn(), get: vi.fn() },
}));
beforeEach(() => vi.clearAllMocks());

describe('CRM agent record lookup', () => {
  it('searches contacts on the server with workspace and pagination scope', async () => {
    vi.mocked(crmContactService.list).mockResolvedValue({ data: {
      data: [{ id: 'contact-1', first_name: 'Sam', last_name: 'Lee', email: 'sam@example.test' }], total: 40, page: 1,
    }, error: null } as never);
    expect(await listCRMAgentRecords('ws-1', 'crm_contact', 'Sam')).toEqual({
      items: [{ id: 'contact-1', name: 'Sam Lee', detail: 'sam@example.test' }], total: 40,
    });
    expect(crmContactService.list).toHaveBeenCalledWith('ws-1', { search: 'Sam', page: 1, per_page: 25 });
    expect(crmCompanyService.list).not.toHaveBeenCalled();
    expect(crmDealService.list).not.toHaveBeenCalled();
  });

  it('resolves saved company selections directly rather than from the first search page', async () => {
    vi.mocked(crmCompanyService.get).mockResolvedValue({ data: { id: 'company-900', name: 'Acme', domain: 'acme.test' }, error: null } as never);
    expect(await getCRMAgentRecord('ws-1', 'crm_company', 'company-900')).toEqual({ id: 'company-900', name: 'Acme', detail: 'acme.test' });
    expect(crmCompanyService.get).toHaveBeenCalledWith('ws-1', 'company-900');
    expect(crmCompanyService.list).not.toHaveBeenCalled();
  });

  it('keeps deal identity and pipeline context when names match', async () => {
    vi.mocked(crmDealService.list).mockResolvedValue({ data: { data: [
      { id: 'deal-1', name: 'Renewal', display_id: 'DEA-1', pipeline: { name: 'Enterprise' }, stage: { name: 'Proposal' } },
      { id: 'deal-2', name: 'Renewal', display_id: 'DEA-2' },
    ], total: 2, page: 1 }, error: null } as never);
    expect((await listCRMAgentRecords('ws-1', 'crm_deal', '')).items).toEqual([
      { id: 'deal-1', name: 'Renewal', detail: 'DEA-1 · Enterprise · Proposal' },
      { id: 'deal-2', name: 'Renewal', detail: 'DEA-2' },
    ]);
  });

  it('accepts an empty server page and rejects failed requests', async () => {
    vi.mocked(crmCompanyService.list).mockResolvedValue({ data: { data: null, total: 0, page: 1 }, error: null } as never);
    expect(await listCRMAgentRecords('ws-1', 'crm_company', '')).toEqual({ items: [], total: 0 });
    vi.mocked(crmCompanyService.list).mockResolvedValue({ data: null, error: 'Forbidden', status: 403 });
    await expect(listCRMAgentRecords('ws-1', 'crm_company', '')).rejects.toThrow('Forbidden');
    vi.mocked(crmCompanyService.get).mockResolvedValue({ data: null, error: null, status: 404 });
    await expect(getCRMAgentRecord('ws-1', 'crm_company', 'missing')).rejects.toThrow('returned no data');
  });
});
