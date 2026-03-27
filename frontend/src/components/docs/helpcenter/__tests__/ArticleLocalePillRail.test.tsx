// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props}>{children}</button>,
}))

vi.mock('@/components/ui/badge', () => ({
  Badge: ({ children }: { children: ReactNode }) => <span>{children}</span>,
}))

import { ArticleLocalePillRail } from '../ArticleLocalePillRail'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('ArticleLocalePillRail', () => {
  it('renders source and translated locale pills with compact status badges and click handlers', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onSelectLocale = vi.fn()

    act(() => {
      root.render(
        <ArticleLocalePillRail
          items={[
            { locale: 'en', shortLabel: 'EN', isSource: true, sourceStatus: 'published', isActive: true },
            { locale: 'fr', shortLabel: 'FR', translationState: 'draft', isActive: false },
            { locale: 'de', shortLabel: 'DE', translationState: 'missing', isActive: false },
            { locale: 'es', shortLabel: 'ES', translationState: 'needs_review', isActive: false },
          ]}
          onSelectLocale={onSelectLocale}
        />,
      )
    })

    expect(container.textContent).toContain('EN')
    expect(container.textContent).toContain('Source')
    expect(container.textContent).toContain('Published')
    expect(container.textContent).toContain('FR')
    expect(container.textContent).toContain('Draft')
    expect(container.textContent).toContain('DE')
    expect(container.textContent).toContain('Add')
    expect(container.textContent).toContain('ES')
    expect(container.textContent).toContain('Needs review')

    const frButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('FR'))
    expect(frButton).toBeTruthy()

    act(() => {
      frButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(onSelectLocale).toHaveBeenCalledWith('fr')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
