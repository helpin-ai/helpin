// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => <button {...props}>{children}</button>,
}))

vi.mock('@/components/ui/dialog', () => ({
  Dialog: ({ open, children }: { open: boolean; children: ReactNode }) => (open ? <div>{children}</div> : null),
  DialogContent: ({ children }: { children: ReactNode }) => <section>{children}</section>,
  DialogHeader: ({ children }: { children: ReactNode }) => <header>{children}</header>,
  DialogTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
  DialogDescription: ({ children }: { children: ReactNode }) => <p>{children}</p>,
  DialogFooter: ({ children }: { children: ReactNode }) => <footer>{children}</footer>,
}))

import { MissingArticleTranslationDialog } from '../MissingArticleTranslationDialog'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('MissingArticleTranslationDialog', () => {
  it('offers manual creation and AI generation for missing locales', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onCreateManually = vi.fn()
    const onGenerateWithAI = vi.fn()

    act(() => {
      root.render(
        <MissingArticleTranslationDialog
          open
          locale="fr"
          onOpenChange={vi.fn()}
          onCreateManually={onCreateManually}
          onGenerateWithAI={onGenerateWithAI}
          isGenerating={false}
        />,
      )
    })

    expect(container.textContent).toContain('French')
    expect(container.textContent).toContain('Create manually')
    expect(container.textContent).toContain('Generate with AI')

    const buttons = Array.from(container.querySelectorAll('button'))
    const createButton = buttons.find((button) => button.textContent?.includes('Create manually'))
    const generateButton = buttons.find((button) => button.textContent?.includes('Generate with AI'))

    act(() => {
      createButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      generateButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(onCreateManually).toHaveBeenCalledTimes(1)
    expect(onGenerateWithAI).toHaveBeenCalledTimes(1)

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
