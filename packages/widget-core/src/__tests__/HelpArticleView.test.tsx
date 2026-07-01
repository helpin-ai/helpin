import { fireEvent, render, waitFor } from '@testing-library/preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HelpArticleView } from '../components/HelpArticleView';

describe('HelpArticleView', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('loads article detail using the canonical article key', async () => {
    const fetchMock = vi.fn(async () => new Response(
      JSON.stringify({
        id: 'doc-1',
        title: 'Workspace setup',
        slug: 'workspace-setup',
        public_id: '884d78a2',
        article_key: 'workspace-setup-884d78a2',
        content_html: '<p>Hello</p>',
      }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ));
    vi.stubGlobal('fetch', fetchMock);

    const { getByText } = render(
      <HelpArticleView
        host="docs.helpin.ai"
        widgetKey="wk_123"
        articleKey="workspace-setup-884d78a2"
        onBack={() => {}}
      />,
    );

    await waitFor(() => {
      expect(getByText('Hello')).toBeTruthy();
    });

    expect(fetchMock).toHaveBeenCalledWith(
      'https://docs.helpin.ai/widget/support/help/articles/workspace-setup-884d78a2?widget_key=wk_123',
    );
  });

  it('uses an inline header close button', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(
        JSON.stringify({
          id: 'doc-1',
          title: 'Workspace setup',
          slug: 'workspace-setup',
          public_id: '884d78a2',
          article_key: 'workspace-setup-884d78a2',
          content_html: '<p>Hello</p>',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )),
    );

    const onClose = vi.fn();
    const { container, getByText } = render(
      <HelpArticleView
        host="docs.helpin.ai"
        widgetKey="wk_123"
        articleKey="workspace-setup-884d78a2"
        onBack={() => {}}
        onClose={onClose}
      />,
    );

    await waitFor(() => {
      expect(getByText('Hello')).toBeTruthy();
    });

    const header = container.querySelector('.helpin-article-header');
    const closeButton = header?.querySelector('.helpin-window-close-inline[aria-label="Close"]');

    expect(closeButton).toBeTruthy();
    fireEvent.click(closeButton as Element);
    expect(onClose).toHaveBeenCalled();
  });

  it('marks the article screen for drill-in animation', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(
        JSON.stringify({
          id: 'doc-1',
          title: 'Workspace setup',
          slug: 'workspace-setup',
          public_id: '884d78a2',
          article_key: 'workspace-setup-884d78a2',
          content_html: '<p>Hello</p>',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      )),
    );

    const { container } = render(
      <HelpArticleView
        host="docs.helpin.ai"
        widgetKey="wk_123"
        articleKey="workspace-setup-884d78a2"
        onBack={() => {}}
      />,
    );

    expect(container.querySelector('.helpin-article-view')?.classList.contains('helpin-help-drilldown-view')).toBe(true);
  });
});
