// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AgentRunArtifactView } from '../AgentRunArtifactView';
import type { AgentRunArtifact } from '@/lib/pmTypes';
import { automationService } from '@/lib/services/automationService';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.restoreAllMocks();
});

function browserRecordingArtifact(): AgentRunArtifact {
  return {
    id: 'recording-1',
    workspace_id: 'ws-1',
    run_id: 'run-1',
    artifact_type: 'browser_recording',
    format: 'mp4',
    storage_mode: 'object',
    metadata: { file_name: 'login-flow.mp4' },
    sequence_no: 1,
    created_at: '2026-08-10T12:00:00Z',
  };
}

describe('AgentRunArtifactView browser recordings', () => {
  it('loads the private content URL and renders native video controls', async () => {
    const contentURL = vi.spyOn(automationService, 'getArtifactContentURL').mockResolvedValue({
      data: { url: 'https://signed.example/login-flow.mp4', expires_at: '2026-08-10T13:00:00Z' },
      error: null,
    });

    await act(async () => {
      root.render(<AgentRunArtifactView artifact={browserRecordingArtifact()} />);
    });

    const video = container.querySelector<HTMLVideoElement>('video');
    expect(contentURL).toHaveBeenCalledWith('ws-1', 'recording-1');
    expect(video?.getAttribute('src')).toBe('https://signed.example/login-flow.mp4');
    expect(video?.controls).toBe(true);
    expect(video?.preload).toBe('metadata');
  });
});

describe('private analysis outputs', () => {
  it.each(['png', 'csv', 'json', 'txt'])('retrieves %s through the authenticated artifact endpoint', async (format) => {
    const contentURL = vi.spyOn(automationService, 'getArtifactContentURL').mockResolvedValue({
      data: { url: `https://signed.example/result.${format}`, expires_at: '2026-09-17T13:00:00Z' }, error: null,
    });
    await act(async () => root.render(<AgentRunArtifactView artifact={{ ...browserRecordingArtifact(), artifact_type: 'analysis_output', format }} />));
    expect(contentURL).toHaveBeenCalledWith('ws-1', 'recording-1');
    expect(container.querySelector('a[download]')?.getAttribute('href')).toBe(`https://signed.example/result.${format}`);
    expect(Boolean(container.querySelector('img'))).toBe(format === 'png');
    expect(container.querySelector('video')).toBeNull();
  });
});
