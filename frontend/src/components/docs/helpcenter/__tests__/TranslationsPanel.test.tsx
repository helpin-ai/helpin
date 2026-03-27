// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/components/ui/card', () => ({
  Card: ({ children }: { children: ReactNode }) => <section>{children}</section>,
  CardHeader: ({ children }: { children: ReactNode }) => <header>{children}</header>,
  CardTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
  CardDescription: ({ children }: { children: ReactNode }) => <p>{children}</p>,
  CardContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}))

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props}>{children}</button>,
}))

vi.mock('@/components/ui/badge', () => ({
  Badge: ({ children }: { children: ReactNode }) => <span>{children}</span>,
}))

import { TranslationsPanel } from '../TranslationsPanel'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('TranslationsPanel', () => {
  it('renders locale rows with missing, draft, published, and needs review states and disables publish when parents are not ready', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <TranslationsPanel
          title="Article translations"
          description="Manage published language variants"
          locales={['en', 'fr', 'de', 'es']}
          rows={[
            { locale: 'en', state: 'missing' },
            { locale: 'fr', state: 'draft', updatedAtLabel: 'Updated 2h ago' },
            { locale: 'de', state: 'published', updatedAtLabel: 'Updated yesterday' },
            { locale: 'es', state: 'needs_review', updatedAtLabel: 'Source changed today', publishBlockedReason: 'Publish the parent translation first' },
          ]}
          onAdd={vi.fn()}
          onEdit={vi.fn()}
          onPublish={vi.fn()}
          onUnpublish={vi.fn()}
          onMarkReviewed={vi.fn()}
        />,
      )
    })

    expect(container.textContent).toContain('Add')
    expect(container.textContent).toContain('Draft')
    expect(container.textContent).toContain('Published')
    expect(container.textContent).toContain('Needs review')
    expect(container.textContent).toContain('Publish the parent translation first')

    const buttons = Array.from(container.querySelectorAll('button'))
    const disabledPublish = buttons.find((button) => button.textContent === 'Publish' && button.hasAttribute('disabled'))
    expect(disabledPublish).toBeTruthy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
