import { afterEach, describe, expect, it, vi } from 'vitest';
import { pmImportService } from '../pmImportService';

describe('pmImportService Shortcut API import', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('starts Shortcut API preview scan', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          scan_id: 'scan-1',
          status: 'pending',
        }),
        { status: 202, headers: { 'Content-Type': 'application/json' } },
      ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const { data, error } = await pmImportService.previewShortcutAPI(
      'ws-1',
      'sc-token',
      {
        import_archived: true,
        import_completed: false,
      },
      'scan-1',
    );

    expect(error).toBeNull();
    expect(data?.scan_id).toBe('scan-1');
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/workspaces/ws-1/import/shortcut/api/preview'),
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
        }),
      }),
    );
    expect(JSON.parse(fetchMock.mock.calls[0]?.[1]?.body as string)).toEqual({
      api_token: 'sc-token',
      scan_id: 'scan-1',
      options: { import_archived: true, import_completed: false },
    });
  });

  it('loads Shortcut API preview scan result', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          scan_id: 'scan-1',
          status: 'ready',
          progress: {
            current_step: 'ready',
            steps_completed: 1,
            steps_total: 1,
            entities_processed: 1,
            entities_total: 1,
          },
          preview: {
            summary: {
              total_tasks: 1,
              tasks_by_type: { feature: 1 },
              epics_count: 0,
              objectives_count: 0,
              sprints_count: 0,
              labels_count: 0,
              docs_count: 0,
              teams_count: 0,
              workflows_count: 0,
              workflow_states_count: 0,
              checklist_items_count: 0,
              duplicate_tasks: 0,
            },
            users: [],
            teams: [],
            workflows: [],
            warnings: [],
          },
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    );
    vi.stubGlobal('fetch', fetchMock);

    const { data, error } = await pmImportService.getShortcutAPIPreview('ws-1', 'scan-1');

    expect(error).toBeNull();
    expect(data?.preview?.summary.total_tasks).toBe(1);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/workspaces/ws-1/import/shortcut/api/preview/scan-1'),
      expect.objectContaining({
        credentials: 'include',
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
        }),
      }),
    );
  });

  it('posts mappings to API execute endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ import_id: 'job-1', status: 'processing' }), {
        status: 202,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    const mappings = [
      {
        shortcut_workflow_id: '500',
        shortcut_workflow_name: 'Product Development',
        mode: 'create_new' as const,
        states: [{ shortcut_state: 'Done', state_type: 'done', position: 0 }],
      },
    ];
    const { data, error } = await pmImportService.executeShortcutAPI(
      'ws-1',
      'sc-token',
      { 'owner@example.com': 'user-1' },
      { 'owner@example.com': 'member-1' },
      { 'Dev Team': 'existing:team-1' },
      mappings,
      { import_archived: true, import_completed: true },
      'scan-1',
    );

    expect(error).toBeNull();
    expect(data?.import_id).toBe('job-1');
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/workspaces/ws-1/import/shortcut/api/execute'),
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        body: JSON.stringify({
          api_token: 'sc-token',
          preview_scan_id: 'scan-1',
          user_mappings: { 'owner@example.com': 'user-1' },
          member_mappings: { 'owner@example.com': 'member-1' },
          team_mappings: { 'Dev Team': 'existing:team-1' },
          workflow_state_mappings: mappings,
          options: { import_archived: true, import_completed: true },
        }),
      }),
    );
  });
});
