// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';

import { filterDocsArticlesByTitle, scheduleFocusDocsArticleSearchInput } from '../knowledgeDocsArticleSelector';

describe('filterDocsArticlesByTitle', () => {
  const articles = [
    { id: 'a1', title: 'Billing setup guide' },
    { id: 'a2', title: 'Connect GitHub repositories' },
    { id: 'a3', title: 'Website crawler limits' },
  ];

  it('filters articles by title without case sensitivity', () => {
    expect(filterDocsArticlesByTitle(articles, 'github')).toEqual([
      { id: 'a2', title: 'Connect GitHub repositories' },
    ]);
  });

  it('returns every article when the search is blank', () => {
    expect(filterDocsArticlesByTitle(articles, '   ')).toEqual(articles);
  });

  it('focuses the latest article search input after the selector opens', async () => {
    const input = document.createElement('input');
    document.body.appendChild(input);

    const cancel = scheduleFocusDocsArticleSearchInput(() => input);

    expect(document.activeElement).not.toBe(input);

    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(document.activeElement).toBe(input);

    cancel();
    input.remove();
  });
});
