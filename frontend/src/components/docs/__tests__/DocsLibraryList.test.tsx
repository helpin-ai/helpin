// @vitest-environment jsdom

import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DocsLibraryList, DocsLibraryRow } from '../DocsLibraryList'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('DocsLibraryList', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders title-first borderless rows and hides the default published status', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <DocsLibraryList>
          <DocsLibraryRow
            title="Published guide"
            status="published"
            updatedAt="2026-08-12T10:00:00Z"
            location="Product docs › Setup"
            onOpen={vi.fn()}
          />
          <DocsLibraryRow
            title="Draft guide"
            status="draft"
            updatedAt="2026-08-12T10:00:00Z"
            onOpen={vi.fn()}
          />
        </DocsLibraryList>,
      )
    })

    expect(container.querySelector('[data-slot="docs-library-list"]')).toBeTruthy()
    expect(container.querySelector('[data-slot="docs-library-row"]')?.className).not.toContain('rounded')
    expect(container.textContent).toContain('Product docs › Setup')
    expect(container.textContent).toContain('Draft')
    expect(Array.from(container.querySelectorAll('span')).some((span) => span.textContent === 'Published')).toBe(false)

    act(() => root.unmount())
  })

  it('opens a document from its primary row button', () => {
    const onOpen = vi.fn()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <DocsLibraryList>
          <DocsLibraryRow
            title="Keyboard guide"
            status="archived"
            updatedAt="2026-08-12T10:00:00Z"
            onOpen={onOpen}
          />
        </DocsLibraryList>,
      )
    })

    act(() => container.querySelector('button')?.click())
    expect(onOpen).toHaveBeenCalledOnce()
    expect(container.textContent).toContain('Archived')

    act(() => root.unmount())
  })
})
