// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { KnowledgeSourceRow } from '@/lib/knowledgeSourcesPresentation';
import { KnowledgeSourceSync } from '../KnowledgeSourceSync';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const website: KnowledgeSourceRow = {
  id: 'website:site', sourceId: 'site', type: 'website', typeLabel: 'Website', name: 'Guide',
  description: 'https://example.test', status: 'ready', progress: 100, countLabel: '12 pages',
  lastSyncAt: '2026-10-01T02:03:00Z', syncStartedAt: '2026-10-01T02:00:00Z',
  syncCompletedAt: '2026-10-01T02:03:00Z', nextSyncAt: '2026-10-02T02:00:00Z', indexedChunks: 48,
};

describe('KnowledgeSourceSync', () => {
  let container: HTMLDivElement;
  let root: Root;
  beforeEach(() => { container = document.createElement('div'); document.body.appendChild(container); root = createRoot(container); });
  afterEach(() => { act(() => root.unmount()); container.remove(); });

  const render = async (source: KnowledgeSourceRow) => {
    await act(async () => { root.render(<TooltipProvider><KnowledgeSourceSync source={source} /></TooltipProvider>); });
  };

  it('shows both timestamps and opens sync details on click', async () => {
    await render(website);
    expect(container.textContent).toContain('Last');
    expect(container.textContent).toContain('Next');
    const trigger = container.querySelector<HTMLButtonElement>('button[aria-label="View sync details for Guide"]');
    expect(trigger).not.toBeNull();
    await act(async () => trigger!.click());
    const drawer = document.body.querySelector('[role="dialog"]');
    expect(drawer?.textContent).toContain('Sync details');
    expect(drawer?.textContent).toContain('Completed');
    expect(drawer?.textContent).toContain('12 pages');
    expect(drawer?.textContent).toContain('48');
    expect(drawer?.textContent).toContain('3m');
  });

  it('shows failures in the drawer without inventing a completion time', async () => {
    await render({ ...website, status: 'failed', syncCompletedAt: null, error: 'Website could not be reached.' });
    await act(async () => container.querySelector('button')!.click());
    const drawer = document.body.querySelector('[role="dialog"]');
    expect(drawer?.textContent).toContain('Failed');
    expect(drawer?.textContent).toContain('Website could not be reached.');
    expect(drawer?.textContent).not.toContain('Duration');
  });

  it('keeps on-change docs and manual files distinct from scheduled websites', async () => {
    await render({ ...website, type: 'helpin_docs', nextSyncAt: null, lastSyncAt: null });
    expect(container.textContent).toContain('On changes');
    expect(container.querySelector('button')).toBeNull();
    await render({ ...website, type: 'file', nextSyncAt: null });
    expect(container.textContent).toContain('Manual');
    expect(container.textContent).not.toContain('Next');
  });
});
