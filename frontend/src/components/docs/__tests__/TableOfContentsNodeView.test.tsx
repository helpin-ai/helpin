// @vitest-environment jsdom
import { act } from 'react'
import type { ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@tiptap/react', () => ({
  NodeViewWrapper: ({ children }: { children: ReactNode }) => <div data-node-view-wrapper="">{children}</div>,
}))

vi.mock('@/lib/icons', () => ({
  LeftToRightListBulletIcon: ({ className }: { className?: string }) => <svg className={className} />,
}))

import { TableOfContentsNodeView, tableOfContentsIndent } from '../TableOfContentsNodeView'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function createEditorMock() {
  const headings = [
    { pos: 1, level: 2, text: 'Overview' },
    { pos: 10, level: 3, text: 'Install' },
    { pos: 20, level: 4, text: 'Configure' },
  ]
  const setTextSelection = vi.fn(() => chain)
  const focus = vi.fn(() => chain)
  const run = vi.fn(() => true)
  const chain = { focus, setTextSelection, run }

  return {
    isEditable: true,
    view: {
      nodeDOM: vi.fn(() => {
        const heading = document.createElement('h2')
        heading.scrollIntoView = vi.fn()
        return heading
      }),
    },
    state: {
      doc: {
        descendants: (visitor: (node: { type: { name: string }; attrs: { level: number }; textContent: string }, pos: number) => void) => {
          headings.forEach((heading) => {
            visitor(
              {
                type: { name: 'heading' },
                attrs: { level: heading.level },
                textContent: heading.text,
              },
              heading.pos,
            )
          })
        },
      },
    },
    on: vi.fn(),
    off: vi.fn(),
    chain: vi.fn(() => chain),
  }
}

describe('TableOfContentsNodeView', () => {
  it('renders Notion-style heading links without numeric ordered-list markers', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const editor = createEditorMock()

    act(() => {
      root.render(<TableOfContentsNodeView editor={editor as never} />)
    })

    expect(container.querySelector('ol')).toBeNull()
    expect(container.querySelector('[role="list"]')).toBeTruthy()
    expect(container.querySelector('nav')?.className).not.toContain('border')
    expect(container.querySelector('nav')?.className).not.toContain('rounded')
    expect(container.querySelectorAll('[role="listitem"]')).toHaveLength(3)
    expect(container.textContent).toContain('Overview')
    expect(container.textContent).toContain('Install')
    expect(container.textContent).toContain('Configure')
    expect(container.textContent).not.toContain('1. Overview')
    expect(container.textContent).not.toContain('1.1 Install')
    expect(container.textContent).not.toContain('1.1.1 Configure')

    const listItems = Array.from(container.querySelectorAll('[role="listitem"]')) as HTMLElement[]
    expect(listItems[0]?.style.paddingLeft).toBe('0px')
    expect(listItems[1]?.style.paddingLeft).toBe('12px')
    expect(listItems[2]?.style.paddingLeft).toBe('24px')

    const firstButton = container.querySelector('button')
    act(() => {
      firstButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    expect(editor.view.nodeDOM).toHaveBeenCalledWith(1)
    expect(editor.chain).toHaveBeenCalled()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('computes indentation from heading depth only', () => {
    expect(tableOfContentsIndent(2)).toBe('0px')
    expect(tableOfContentsIndent(3)).toBe('12px')
    expect(tableOfContentsIndent(4)).toBe('24px')
  })
})
