import { fireEvent, render, waitFor } from '@testing-library/preact';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HelpCollectionView } from '../components/HelpCollectionView';

describe('HelpCollectionView', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('passes the canonical article key when an article is selected', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/collections/getting-started/articles')) {
        return new Response(
          JSON.stringify([
            {
              id: 'doc-1',
              title: 'Workspace setup',
              slug: 'workspace-setup',
              public_id: '884d78a2',
              article_key: 'workspace-setup-884d78a2',
            },
          ]),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }
      throw new Error(`unexpected fetch url: ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const onSelectArticle = vi.fn();
    const { getByText } = render(
      <HelpCollectionView
        host="docs.helpin.ai"
        widgetKey="wk_123"
        collectionSlug="getting-started"
        onBack={() => {}}
        onSelectArticle={onSelectArticle}
      />,
    );

    await waitFor(() => {
      expect(getByText('Workspace setup')).toBeTruthy();
    });

    fireEvent.click(getByText('Workspace setup'));

    expect(onSelectArticle).toHaveBeenCalledWith('workspace-setup-884d78a2');
  });
});
