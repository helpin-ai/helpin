// @vitest-environment jsdom
import { act } from 'react'
import type { ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@tiptap/react', () => ({
  NodeViewWrapper: ({ children }: { children: ReactNode }) => <div data-node-view-wrapper="">{children}</div>,
}))

vi.mock('@/lib/icons', () => ({
  Copy01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  SourceCodeIcon: ({ className }: { className?: string }) => <svg className={className} />,
  ViewIcon: ({ className }: { className?: string }) => <svg className={className} />,
  PencilEdit01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  Delete01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  Alert01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  Tick01Icon: ({ className }: { className?: string }) => <svg className={className} />,
}))

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

import { HtmlBlockNodeView } from '../HtmlBlockNodeView'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function createEditorMock() {
  return {
    isEditable: false,
    state: { selection: { from: 1 } },
    on: vi.fn(),
    off: vi.fn(),
  }
}

function createEditableEditorMock() {
  return {
    ...createEditorMock(),
    isEditable: true,
  }
}

describe('HtmlBlockNodeView', () => {
  it('renders trusted raw HTML blocks in an isolated iframe', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <HtmlBlockNodeView
          node={{
            attrs: {
              html: '<div><table><tbody id="models-body"></tbody></table><iframe src="https://www.youtube.com/embed/abc123"></iframe><script>window.__helpinTest = true</script></div>',
              renderMode: 'sandboxed',
            },
            nodeSize: 1,
          } as never}
          editor={createEditorMock() as never}
          updateAttributes={vi.fn()}
          deleteNode={vi.fn()}
          getPos={() => 1}
        />,
      )
    })

    const iframe = container.querySelector('iframe.docs-html-block-frame')
    expect(iframe).toBeTruthy()
    expect(iframe?.getAttribute('sandbox')).toBe('allow-scripts allow-popups allow-forms allow-presentation')
    expect(iframe?.getAttribute('srcdoc')).toContain('<script>window.__helpinTest = true</script>')
    expect(container.querySelector('#models-body')).toBeFalsy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('renders simple inline HTML without editor chrome when read-only', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <HtmlBlockNodeView
          node={{
            attrs: {
              html: '<div><p>Visible content</p></div>',
              renderMode: 'inline',
            },
            nodeSize: 1,
          } as never}
          editor={createEditorMock() as never}
          updateAttributes={vi.fn()}
          deleteNode={vi.fn()}
          getPos={() => 1}
        />,
      )
    })

    expect(container.textContent).toContain('Visible content')
    expect(container.textContent).not.toContain('Rendered')
    expect(container.textContent).not.toContain('Source')
    expect(container.querySelector('iframe')).toBeFalsy()

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('shows rendered and source modes only when editable', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const updateAttributes = vi.fn()

    act(() => {
      root.render(
        <HtmlBlockNodeView
          node={{
            attrs: {
              html: '<div><strong>Visible content</strong></div>',
              renderMode: 'inline',
            },
            nodeSize: 1,
          } as never}
          editor={createEditableEditorMock() as never}
          updateAttributes={updateAttributes}
          deleteNode={vi.fn()}
          getPos={() => 1}
        />,
      )
    })

    expect(container.textContent).toContain('Rendered')
    expect(container.textContent).toContain('Source')
    expect(container.querySelector('strong')?.textContent).toBe('Visible content')

    const buttons = Array.from(container.querySelectorAll('button'))
    const sourceButton = buttons.find((button) => button.textContent === 'Source')
    expect(sourceButton).toBeTruthy()

    act(() => {
      sourceButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const textarea = container.querySelector('textarea')
    expect(textarea?.value).toBe('<div><strong>Visible content</strong></div>')

    const renderedButton = buttons.find((button) => button.textContent === 'Rendered')
    act(() => {
      renderedButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    expect(updateAttributes).toHaveBeenCalledWith({ html: '<div><strong>Visible content</strong></div>' })

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
