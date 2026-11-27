import { render } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const recordArticleView = vi.fn()
let articleId: string | undefined = 'doc-1'

vi.mock('@/lib/services', () => ({
  helpCenterService: { recordArticleView: (...args: unknown[]) => recordArticleView(...args) },
}))
vi.mock('@/contexts/DocsContext', () => ({
  useDocsContext: () => ({ subdomain: 'acme', config: { brand_name: 'Acme' } }),
}))
vi.mock('@/hooks/queries', () => ({
  useArticle: () => ({
    data: articleId
      ? { id: articleId, public_id: 'ab12cd34', title: 'Start Here', space_slug: 'help', content_html: '' }
      : undefined,
    isLoading: false,
    error: null,
  }),
  useSpaceNavigation: () => ({ data: [] }),
}))
// The view request is what this test covers; the page body is out of scope.
vi.mock('@/components/article/ArticleShell', () => ({ ArticleShell: () => null }))
vi.mock('@/components/layout/Sidebar', () => ({ Sidebar: () => null, SidebarSkeleton: () => null }))

import { ArticleRouteView } from '@/components/routes/ArticleRouteView'

describe('ArticleRouteView view counting', () => {
  beforeEach(() => {
    recordArticleView.mockReset()
    articleId = 'doc-1'
  })

  it('reports one view per article, not per render', () => {
    const { rerender } = render(
      <ArticleRouteView locale="fr" articleKey="start-here-ab12cd34" multilingualEnabled />,
    )
    rerender(<ArticleRouteView locale="fr" articleKey="start-here-ab12cd34" multilingualEnabled />)

    expect(recordArticleView).toHaveBeenCalledTimes(1)
    expect(recordArticleView).toHaveBeenCalledWith('acme', 'fr', 'start-here-ab12cd34', true)
  })

  it('does not report a view for a missing article', () => {
    articleId = undefined
    render(<ArticleRouteView locale="en" articleKey="missing-ab12cd34" multilingualEnabled={false} />)

    expect(recordArticleView).not.toHaveBeenCalled()
  })
})
