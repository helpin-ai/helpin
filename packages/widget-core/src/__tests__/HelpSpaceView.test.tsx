import { act, fireEvent, render, waitFor } from '@testing-library/preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HelpSpaceView } from '../components/HelpSpaceView';

describe('HelpSpaceView', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('reserves right-side header space when the selected docs space has a subtitle', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify([]), { status: 200 })),
    );

    const onClose = vi.fn();
    const { container } = render(
      <HelpSpaceView
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        space={{
          id: 'space-1',
          name: 'A very long customer education and implementation docs space',
          slug: 'implementation-docs',
        }}
        showBack
        onBack={() => {}}
        onClose={onClose}
        onSelectCollection={() => {}}
      />,
    );

    const header = container.querySelector('.helpin-help-header');

    expect(header?.querySelector('.helpin-help-back')).toBeTruthy();
    expect(header?.querySelector('.helpin-help-header-copy .helpin-help-title')?.textContent).toBe(
      'A very long customer education and implementation docs space',
    );
    expect(header?.querySelector('.helpin-help-header-copy .helpin-help-subtitle')?.textContent).toBe(
      'Browse collections',
    );
    const closeButton = header?.querySelector('.helpin-window-close-inline');
    expect(closeButton).toBeTruthy();

    fireEvent.click(closeButton as Element);

    expect(onClose).toHaveBeenCalled();
  });

  it('marks the space screen for drill-in animation', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify([]), { status: 200 })),
    );

    const { container } = render(
      <HelpSpaceView
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        space={{ id: 'space-1', name: 'Developer Docs', slug: 'developer-docs' }}
        showBack
        onBack={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    expect(container.querySelector('.helpin-help-view')?.classList.contains('helpin-help-drilldown-view')).toBe(true);
  });

  it('delays skeleton rows so fast collection loads do not flash loading UI', async () => {
    vi.useFakeTimers();
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify([
        {
          id: 'collection-1',
          name: 'Analytics',
          slug: 'analytics',
          public_id: 'abc123',
          parent_collection_id: null,
          depth: 0,
          article_count: 3,
        },
      ]), { status: 200 })),
    );

    const { container, queryByText } = render(
      <HelpSpaceView
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        space={{ id: 'space-1', name: 'Developer Docs', slug: 'developer-docs' }}
        showBack={false}
        showHeader={false}
        onBack={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    expect(container.querySelectorAll('.helpin-help-link-skeleton')).toHaveLength(0);
    expect(container.querySelector('.helpin-help-loading-reserve')).toBeTruthy();
    expect(queryByText('Loading collections...')).toBeNull();

    await waitFor(() => expect(queryByText('Analytics')).toBeTruthy());

    expect(container.querySelectorAll('.helpin-help-link-skeleton')).toHaveLength(0);
    expect(container.querySelector('.helpin-help-loading-reserve')).toBeNull();
  });

  it('shows skeleton rows only after collections have loaded for at least 150ms', () => {
    vi.useFakeTimers();
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    );

    const { container, queryByText } = render(
      <HelpSpaceView
        host="https://client.helpin.ai"
        widgetKey="wk_123"
        space={{ id: 'space-1', name: 'Developer Docs', slug: 'developer-docs' }}
        showBack={false}
        showHeader={false}
        onBack={() => {}}
        onSelectCollection={() => {}}
      />,
    );

    expect(queryByText('Loading collections...')).toBeNull();
    expect(container.querySelector('.helpin-help-loading-reserve')).toBeTruthy();
    expect(container.querySelectorAll('.helpin-help-link-skeleton')).toHaveLength(0);

    act(() => {
      vi.advanceTimersByTime(149);
    });

    expect(container.querySelector('.helpin-help-loading-reserve')).toBeTruthy();
    expect(container.querySelectorAll('.helpin-help-link-skeleton')).toHaveLength(0);

    act(() => {
      vi.advanceTimersByTime(1);
    });

    expect(container.querySelector('.helpin-help-loading-reserve')).toBeNull();
    expect(container.querySelectorAll('.helpin-help-link-skeleton')).toHaveLength(3);
    expect(container.querySelector('.helpin-help-loading-list')?.getAttribute('aria-label')).toBe('Loading collections');
  });
});
