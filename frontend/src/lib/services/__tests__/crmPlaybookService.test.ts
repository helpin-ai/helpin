import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { crmPlaybookService, PlaybookRequestError, readPlaybookResponse } from '../crmPlaybookService';
import { blankPlaybook } from '@/lib/crmPlaybookPresentation';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
beforeEach(() => vi.clearAllMocks());

describe('Playbook API contracts', () => {
  it('reviews saved IDs and publishes exactly the approved fingerprint without activation', async () => {
    const selection = { playbook_version_id: 'policy-2', expected_revision: 5, flow_id: 'flow-1', agent_id: 'beacon-1' };
    await crmPlaybookService.reviewConnection('ws&1', 'book-1', selection);
    expect(api.post).toHaveBeenLastCalledWith('/crm/playbooks/book-1/automation/connection/preview?workspace_id=ws%261', selection);
    const approved = { ...selection, command_key: 'save-once', expected_connection_version: 2, review_fingerprint: 'reviewed-digest' };
    await crmPlaybookService.publishConnection('ws&1', 'book-1', approved);
    expect(api.post).toHaveBeenLastCalledWith('/crm/playbooks/book-1/automation/connections?workspace_id=ws%261', approved);
    expect(api.post).toHaveBeenCalledTimes(2);
    expect(approved).not.toHaveProperty('execution_enabled');
    expect(approved).not.toHaveProperty('snapshot');
  });
  it('reads exact receipts and paginates connection history', async () => {
    await crmPlaybookService.connection('ws-1', 'book-1', 'connection-2');
    expect(api.get).toHaveBeenLastCalledWith('/crm/playbooks/book-1/automation/connections/connection-2?workspace_id=ws-1');
    await crmPlaybookService.connections('ws-1', 'book-1', 3);
    expect(api.get).toHaveBeenLastCalledWith('/crm/playbooks/book-1/automation/connections?workspace_id=ws-1&before=3');
    expect(api.post).not.toHaveBeenCalled();
  });
  it('scopes list filters and pagination to the workspace', async () => {
    await crmPlaybookService.list('ws&1', { state: 'draft', q: 'a & b', page: 2 });
    const url = new URL(vi.mocked(api.get).mock.calls[0][0], 'http://test');
    expect(url.pathname).toBe('/crm/playbooks');
    expect(Object.fromEntries(url.searchParams)).toEqual({ workspace_id: 'ws&1', state: 'draft', q: 'a & b', page: '2', page_size: '25' });
  });
  it('accepts the bare Playbook creation response, not a list/detail wrapper', async () => {
    const response = { id: 'book-1', revision: 1, draft: blankPlaybook(), accepting_customers: false, published_version_id: null };
    vi.mocked(api.post).mockResolvedValue({ data: response, error: null, status: 201 });
    const result = readPlaybookResponse(await crmPlaybookService.create('ws-1', { creation_key: 'create-once', definition: blankPlaybook() }));
    expect(result.id).toBe('book-1');
    expect(result).not.toHaveProperty('playbook');
  });
  it('defaults participants to all owners and states, not the Signals attention queue', async () => {
    await crmPlaybookService.participants('ws-1', 'book-1', {});
    expect(vi.mocked(api.get).mock.calls[0][0]).toContain('scope=all&state=all');
  });
  it('previews the precise saved revision and selects published policy explicitly', async () => {
    await crmPlaybookService.preview('ws-1', 'book-1', 8, 'version-2', 3);
    expect(vi.mocked(api.get).mock.calls[0][0]).toContain('revision=8&version_id=version-2&page=3');
    expect(api.post).not.toHaveBeenCalled();
  });
  it('keeps version conflicts distinguishable and rejects missing responses', () => {
    try { readPlaybookResponse({ data: null, error: 'Reload this playbook.', status: 409 }); } catch (error) {
      expect(error).toBeInstanceOf(PlaybookRequestError);
      expect((error as PlaybookRequestError).status).toBe(409);
    }
    expect(() => readPlaybookResponse({ data: null, error: null })).toThrow('No response received');
  });
  it('reads exact published skill selection without creating or enabling automation', async () => {
    await crmPlaybookService.automationPreview('ws&1', 'book-1', 8, 'version-2');
    const url = new URL(vi.mocked(api.get).mock.calls[0][0], 'http://test');
    expect(url.pathname).toBe('/crm/playbooks/book-1/automation/preview');
    expect(Object.fromEntries(url.searchParams)).toEqual({ workspace_id: 'ws&1', revision: '8', version_id: 'version-2' });
    expect(api.post).not.toHaveBeenCalled();
  });
  it('does not imply a published skill connection when previewing a draft', async () => {
    await crmPlaybookService.automationPreview('ws-1', 'book-1', 4);
    const url = new URL(vi.mocked(api.get).mock.calls[0][0], 'http://test');
    expect(url.searchParams.has('version_id')).toBe(false);
    expect(url.searchParams.has('execution_enabled')).toBe(false);
    expect(api.post).not.toHaveBeenCalled();
  });
});
