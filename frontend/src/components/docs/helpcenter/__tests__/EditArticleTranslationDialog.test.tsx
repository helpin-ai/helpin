// @vitest-environment jsdom
import { act } from 'react'
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react'
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

vi.mock('@/components/ui/input', () => ({
  Input: (props: InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}))

vi.mock('@/components/ui/label', () => ({
  Label: ({ children, ...props }: React.LabelHTMLAttributes<HTMLLabelElement>) => <label {...props}>{children}</label>,
}))

vi.mock('@/components/ui/textarea', () => ({
  Textarea: (props: TextareaHTMLAttributes<HTMLTextAreaElement>) => <textarea {...props} />,
}))

vi.mock('@/components/docs/DocsEditor', () => ({
  DocsEditor: () => <div data-testid="docs-editor" />,
}))

import { EditArticleTranslationDialog } from '../EditArticleTranslationDialog'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('EditArticleTranslationDialog', () => {
  it('does not require a slug field for saving localized draft metadata', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <EditArticleTranslationDialog
          open
          onOpenChange={vi.fn()}
          locale="fr"
          sourceTitle="Start here"
          sourceExcerpt="Quick start"
          translation={{
            id: 'fr-1',
            document_id: 'doc-1',
            workspace_id: 'ws-1',
            space_id: 'space-1',
            locale: 'fr',
            title: 'Commencer ici',
            excerpt: 'Demarrage rapide',
            content: null,
            content_text: '',
            status: 'draft',
            source_synced: true,
            view_count: 0,
            helpful_count: 0,
            not_helpful_count: 0,
            created_at: '2026-03-27T00:00:00Z',
            updated_at: '2026-03-27T00:00:00Z',
          }}
          isSaving={false}
          mode="details_only"
          onSave={vi.fn()}
        />,
      )
    })

    expect(container.textContent).toContain('The public slug is confirmed on first publish')
    expect(container.textContent).toContain('Meta description')
    expect(container.textContent).not.toContain('SEO description')
    expect(container.querySelector('#article-translation-slug')).toBeNull()
    expect(container.querySelector('#article-translation-title')).toBeNull()
    const saveButton = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Save'),
    )
    expect(saveButton?.hasAttribute('disabled')).toBe(false)

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
