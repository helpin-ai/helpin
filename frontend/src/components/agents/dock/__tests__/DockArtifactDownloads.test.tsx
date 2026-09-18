// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DockArtifactDownloads } from '../DockArtifactDownloads';
import type { AgentRunArtifact } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  getArtifactContentURL: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock('@/lib/services/automationService', () => ({
  automationService: { getArtifactContentURL: mocks.getArtifactContentURL },
}));

vi.mock('sonner', () => ({ toast: { error: mocks.toastError } }));

function artifact(id: string, fileName: string, storageMode = 'object'): AgentRunArtifact {
  return {
    id,
    workspace_id: 'workspace-1',
    run_id: 'run-1',
    artifact_type: 'analysis_output',
    format: fileName.split('.').pop() || 'txt',
    storage_mode: storageMode,
    metadata: { file_name: fileName, content_type: 'text/csv', size_bytes: 2048 },
    sequence_no: 1,
    created_at: '2026-09-18T07:00:00Z',
  };
}

describe('DockArtifactDownloads', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    mocks.getArtifactContentURL.mockReset();
    mocks.toastError.mockReset();
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.restoreAllMocks();
  });

  it('shows durable private files and opens an authenticated download', async () => {
    const replace = vi.fn();
    vi.spyOn(window, 'open').mockReturnValue({ opener: null, location: { replace }, close: vi.fn() } as unknown as Window);
    mocks.getArtifactContentURL.mockResolvedValue({ data: { url: 'https://private.example/file.csv', expires_at: 'later' }, error: null });

    await act(async () => {
      root.render(
        <DockArtifactDownloads
          workspaceId="workspace-1"
          artifacts={[artifact('file-1', 'results.csv'), artifact('file-2', 'notes.txt'), artifact('inline', 'hidden.csv', 'inline')]}
        />,
      );
    });

    expect(container.textContent).toContain('2 files');
    expect(container.textContent).toContain('Private');
    expect(container.textContent).toContain('results.csv');
    expect(container.textContent).toContain('notes.txt');
    expect(container.textContent).not.toContain('hidden.csv');

    const button = container.querySelector('button[aria-label="Download results.csv"]');
    expect(button).not.toBeNull();
    await act(async () => { button?.dispatchEvent(new MouseEvent('click', { bubbles: true })); });

    expect(mocks.getArtifactContentURL).toHaveBeenCalledWith('workspace-1', 'file-1');
    expect(replace).toHaveBeenCalledWith('https://private.example/file.csv');
  });
});
