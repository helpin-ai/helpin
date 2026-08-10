// @vitest-environment jsdom
import { act } from 'react';
import type { ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@tiptap/react', () => ({
  NodeViewWrapper: ({ children }: { children: ReactNode }) => <div data-node-view-wrapper="">{children}</div>,
}));

vi.mock('@/lib/icons', () => ({
  Delete01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  Download04Icon: ({ className }: { className?: string }) => <svg className={className} />,
  PlayCircleIcon: ({ className }: { className?: string }) => <svg className={className} />,
}));

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

import { automationService } from '@/lib/services/automationService';
import { ArtifactVideoNodeView } from '../ArtifactVideoNodeView';

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

describe('ArtifactVideoNodeView', () => {
  it('resolves a private artifact and renders native video controls', async () => {
    const contentURL = vi.spyOn(automationService, 'getArtifactContentURL').mockResolvedValue({
      data: { url: 'https://signed.example/recording.mp4', expires_at: '2026-08-10T13:00:00Z' },
      error: null,
    });

    await act(async () => {
      root.render(
        <ArtifactVideoNodeView
          node={{
            attrs: {
              src: 'helpin://artifacts/recording-1',
              artifactId: 'recording-1',
              fileName: 'login-flow.mp4',
              contentType: 'video/mp4',
              description: 'GitHub connection login flow',
              caption: 'Authentication walkthrough',
            },
          } as never}
          editor={{ isEditable: false } as never}
          extension={{ options: { workspaceId: 'ws-1' } } as never}
          deleteNode={vi.fn()}
        />,
      );
    });

    const video = container.querySelector<HTMLVideoElement>('video');
    expect(contentURL).toHaveBeenCalledWith('ws-1', 'recording-1');
    expect(video?.getAttribute('src')).toBe('https://signed.example/recording.mp4');
    expect(video?.controls).toBe(true);
    expect(video?.preload).toBe('metadata');
    expect(video?.getAttribute('aria-label')).toBe('GitHub connection login flow');
    expect(container.textContent).toContain('Authentication walkthrough');
  });
});
