import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { DocsRouteViewport } from '../DocsRouteViewport'

describe('DocsRouteViewport', () => {
  it('uses the shared workspace gutters and Automation bottom spacing', () => {
    const markup = renderToStaticMarkup(
      <DocsRouteViewport>
        <div>Docs page</div>
      </DocsRouteViewport>,
    )

    expect(markup).toContain('p-4')
    expect(markup).toContain('pb-20')
    expect(markup).toContain('md:p-6')
    expect(markup).toContain('md:pb-24')
    expect(markup).toContain('[scrollbar-gutter:stable]')
    expect(markup).toContain('mx-auto max-w-7xl')
  })
})
