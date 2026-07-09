export function filterDocsArticlesByTitle<Article extends { title: string }>(
  articles: Article[],
  search: string,
): Article[] {
  const query = search.trim().toLowerCase();
  if (!query) return articles;
  return articles.filter((article) => article.title.toLowerCase().includes(query));
}

export function scheduleFocusDocsArticleSearchInput(
  getInput: () => HTMLInputElement | null,
): () => void {
  const timeoutId = window.setTimeout(() => {
    getInput()?.focus({ preventScroll: true });
  }, 0);

  return () => window.clearTimeout(timeoutId);
}
