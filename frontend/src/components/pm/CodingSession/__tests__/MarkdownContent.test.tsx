// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { automationService } from '@/lib/services/automationService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { MarkdownContent } from '../MarkdownContent';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

afterEach(() => {
  vi.restoreAllMocks();
  useWorkspaceStore.setState({ currentWorkspace: null });
  document.body.innerHTML = '';
});

describe('MarkdownContent Helpin references', () => {
  it('opens task and document references in a new tab', () => {
    useWorkspaceStore.setState({
      currentWorkspace: { id: 'ws-1', slug: 'acme', name: 'Acme' } as never,
    });
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <MarkdownContent content="[USE-488](helpin://tasks/task-1) and [Architecture](helpin://documents/doc-1)" />,
      );
    });

    const links = Array.from(container.querySelectorAll<HTMLAnchorElement>('a[data-helpin-reference]'));
    expect(links.map((link) => link.textContent)).toEqual(['USE-488', 'Architecture']);
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/w/acme/pm/tasks/task-1',
      '/w/acme/docs/documents/doc-1',
    ]);
    expect(links.map((link) => link.getAttribute('target'))).toEqual(['_blank', '_blank']);
    expect(links.map((link) => link.getAttribute('rel'))).toEqual([
      'noopener noreferrer',
      'noopener noreferrer',
    ]);

    act(() => root.unmount());
  });

  it('keeps normal web links external', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<MarkdownContent content="[Source](https://example.com/research)" />);
    });

    const link = container.querySelector<HTMLAnchorElement>('a');
    expect(link?.getAttribute('href')).toBe('https://example.com/research');
    expect(link?.getAttribute('target')).toBe('_blank');

    act(() => root.unmount());
  });

  it('resolves artifact references through the private content endpoint', async () => {
    useWorkspaceStore.setState({
      currentWorkspace: { id: 'ws-1', slug: 'acme', name: 'Acme' } as never,
    });
    const contentURL = vi.spyOn(automationService, 'getArtifactContentURL').mockResolvedValue({
      data: { url: 'https://signed.example/asset-1', expires_at: '2026-08-06T12:00:00Z' },
      error: null,
    });
    const replace = vi.fn();
    const preview = { opener: window, location: { replace }, close: vi.fn() } as unknown as Window;
    const open = vi.spyOn(window, 'open').mockImplementation(() => preview);
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<MarkdownContent content="[Screenshot](helpin://artifacts/asset-1)" />);
    });
    await act(async () => {
      container.querySelector<HTMLAnchorElement>('a[data-helpin-reference="artifacts"]')?.click();
    });

    expect(contentURL).toHaveBeenCalledWith('ws-1', 'asset-1');
    expect(open).toHaveBeenCalledWith('about:blank', '_blank');
    expect(preview.opener).toBeNull();
    expect(replace).toHaveBeenCalledWith('https://signed.example/asset-1');

    act(() => root.unmount());
  });

  it('renders generated image artifacts inline through a signed URL', async () => {
    useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1', slug: 'acme', name: 'Acme' } as never });
    const contentURL = vi.spyOn(automationService, 'getArtifactContentURL').mockResolvedValue({
      data: { url: 'https://files.example.com/signed/img-1.png', expires_at: '' }, error: null, status: 200,
    } as never);
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(<MarkdownContent content={'Here it is:\n\n![Event sourcing](helpin://artifacts/img-1)'} />);
    });

    expect(contentURL).toHaveBeenCalledWith('ws-1', 'img-1');
    const image = container.querySelector<HTMLImageElement>('[data-agent-inline-image] img');
    expect(image?.getAttribute('src')).toBe('https://files.example.com/signed/img-1.png');
    expect(image?.getAttribute('alt')).toBe('Event sourcing');
    expect(container.querySelector('[data-agent-inline-image]')?.getAttribute('target')).toBe('_blank');

    act(() => root.unmount());
  });

  it('shows a fallback when an inline image cannot be loaded', async () => {
    useWorkspaceStore.setState({ currentWorkspace: { id: 'ws-1', slug: 'acme', name: 'Acme' } as never });
    vi.spyOn(automationService, 'getArtifactContentURL').mockResolvedValue({ data: null, error: 'not found', status: 404 } as never);
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(<MarkdownContent content="![Diagram](helpin://artifacts/missing)" />);
    });

    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toContain('Diagram is unavailable');

    act(() => root.unmount());
  });

  it('does not treat non-artifact images as private artifacts', async () => {
    const contentURL = vi.spyOn(automationService, 'getArtifactContentURL');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(<MarkdownContent content="![Task](helpin://tasks/task-1)" />);
    });

    expect(contentURL).not.toHaveBeenCalled();
    expect(container.querySelector('[data-agent-inline-image]')).toBeNull();

    act(() => root.unmount());
  });
});
