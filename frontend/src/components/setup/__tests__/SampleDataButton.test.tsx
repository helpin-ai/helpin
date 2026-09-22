// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { queryKeys } from '@/lib/queryKeys';
import { sampleDataQueryKey } from '@/hooks/queries/useSampleData';
import type { SampleDataStatus } from '@/lib/sampleDataTypes';
import { SampleDataCard } from '../SampleDataButton';
import { summarizeSampleCounts } from '../sampleDataPresentation';
import { button, click, renderWithQuery, type Rendered } from './setupTestUtils';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn(), del: vi.fn() } }));

const empty: SampleDataStatus = { loaded: false, counts: {}, modules: ['pm', 'docs', 'crm', 'support'] };
const loaded: SampleDataStatus = {
  loaded: true,
  loaded_at: '2026-09-22T10:00:00Z',
  counts: { support_conversation: 4, docs_document: 3, docs_space: 1, pm_epic: 1, pm_task: 6, crm_company: 3, crm_contact: 5, crm_deal: 2 },
  modules: ['pm', 'docs', 'crm', 'support'],
};

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.get).mockReset();
  vi.mocked(api.post).mockReset();
  vi.mocked(api.del).mockReset();
  vi.restoreAllMocks();
});

function mockStatus(status: SampleDataStatus, permissions: string[] = []) {
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path.endsWith('/me')) {
      return { data: { workspace_id: 'ws-1', permissions, modules: [], team_memberships: [] }, error: null, status: 200 } as never;
    }
    return { data: status, error: null, status: 200 } as never;
  });
}

function status(container: HTMLElement) {
  return container.querySelector('[role="status"]')?.textContent ?? '';
}

describe('SampleDataCard', () => {
  it('loads sample data, reports what was added and refreshes the Setup guide', async () => {
    mockStatus(empty);
    vi.mocked(api.post).mockResolvedValue({ data: loaded, error: null, status: 201 } as never);
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" canManage />);
    const { container, client } = rendered;
    const invalidate = vi.spyOn(client, 'invalidateQueries');

    expect(container.textContent).toContain('Northwind Outfitters');
    expect(container.textContent).toContain('support conversations, help articles, a project with tasks and CRM companies, contacts and deals');
    mockStatus(loaded);
    await click(button(container, 'Load sample data'));

    expect(api.post).toHaveBeenCalledWith('/workspaces/ws-1/sample-data');
    expect(status(container)).toBe('Sample data loaded: 4 conversations, 3 help articles, 1 project, 6 tasks, 3 companies, 5 contacts, 2 deals.');
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.workspaces.setup('ws-1') });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['pm', 'ws-1'] });
    expect(client.getQueryData(sampleDataQueryKey('ws-1'))).toEqual(loaded);
    expect(button(container, 'Remove sample data')).toBeDefined();
  });

  it('asks for confirmation before removing sample data', async () => {
    mockStatus(loaded);
    vi.mocked(api.del).mockResolvedValue({ data: { ...empty }, error: null, status: 200 } as never);
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true);
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" canManage />);
    const { container } = rendered;
    expect(container.textContent).toContain('4 conversations, 3 help articles');

    await click(button(container, 'Remove sample data'));
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(api.del).not.toHaveBeenCalled();

    mockStatus(empty);
    await click(button(container, 'Remove sample data'));
    expect(api.del).toHaveBeenCalledWith('/workspaces/ws-1/sample-data');
    expect(status(container)).toBe('Sample data removed.');
    expect(button(container, 'Load sample data')).toBeDefined();
  });

  it('explains which sample containers were kept', async () => {
    mockStatus(loaded);
    vi.mocked(api.del).mockResolvedValue({ data: { ...empty, retained: { docs_space: 1 } }, error: null, status: 200 } as never);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" canManage />);

    await click(button(rendered.container, 'Remove sample data'));

    expect(status(rendered.container)).toBe('Sample data removed. Kept the sample help center space because it now holds your own records.');
  });

  it('shows the server reason when loading fails', async () => {
    mockStatus(empty);
    vi.mocked(api.post).mockResolvedValue({ data: null, error: 'sample data is already loaded', status: 409 } as never);
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" canManage />);

    await click(button(rendered.container, 'Load sample data'));

    expect(status(rendered.container)).toBe('Couldn’t load sample data: sample data is already loaded.');
  });

  it('hides actions from members without workspace update permission', async () => {
    mockStatus(empty, ['pm.edit']);
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" />);

    expect(button(rendered.container, 'Load sample data')).toBeUndefined();
    expect(rendered.container.textContent).toContain('A workspace admin can load sample data.');
  });

  it('reads workspace permissions when canManage is not passed', async () => {
    mockStatus(empty, ['workspace.update']);
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" />);

    expect(api.get).toHaveBeenCalledWith('/workspaces/ws-1/me');
    expect(button(rendered.container, 'Load sample data')).toBeDefined();
  });

  it('offers nothing when no sample-capable module is available', async () => {
    mockStatus({ loaded: false, counts: {}, modules: [] });
    rendered = await renderWithQuery(<SampleDataCard workspaceId="ws-1" canManage />);

    expect(button(rendered.container, 'Load sample data')).toBeUndefined();
    expect(rendered.container.textContent).toContain('Sample data needs Projects, Docs, CRM or Support');
  });
});

describe('summarizeSampleCounts', () => {
  it('uses singular labels and skips empty entities', () => {
    expect(summarizeSampleCounts({ counts: { pm_epic: 1, pm_task: 1, crm_deal: 0 } })).toBe('1 project, 1 task');
  });
});
