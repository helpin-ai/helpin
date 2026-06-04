import { afterEach, describe, expect, it, vi } from 'vitest';

import { API_BASE } from '@/lib/api';
import { searchService } from '@/lib/services/searchService';

describe('searchService', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('passes an abort signal to the search request', async () => {
    const controller = new AbortController();
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      removeItem: vi.fn(),
      setItem: vi.fn(),
    });
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({
        tasks: [],
        epics: [],
        sprints: [],
        objectives: [],
        members: [],
        documents: [],
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    );

    await searchService.search('ws-1', 'billing', { signal: controller.signal });

    expect(fetchMock).toHaveBeenCalledWith(
      `${API_BASE}/search/?workspace_id=ws-1&q=billing`,
      expect.objectContaining({ signal: controller.signal }),
    );
  });
});
