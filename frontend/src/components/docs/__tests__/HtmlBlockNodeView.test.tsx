// @vitest-environment jsdom
import { act } from 'react'
import type { ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@tiptap/react', () => ({
  NodeViewWrapper: ({ children }: { children: ReactNode }) => <div data-node-view-wrapper="">{children}</div>,
}))

vi.mock('@/lib/icons', () => ({
  SourceCodeIcon: ({ className }: { className?: string }) => <svg className={className} />,
  ViewIcon: ({ className }: { className?: string }) => <svg className={className} />,
  PencilEdit01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  Delete01Icon: ({ className }: { className?: string }) => <svg className={className} />,
  Alert01Icon: ({ className }: { className?: string }) => <svg className={className} />,
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

describe('HtmlBlockNodeView', () => {
  it('renders trusted raw HTML blocks inline without an iframe', () => {
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

    const iframes = container.querySelectorAll('iframe')
    expect(iframes).toHaveLength(1)
    expect(iframes[0]?.getAttribute('src')).toBe('https://www.youtube.com/embed/abc123')
    expect(iframes[0]?.className).not.toContain('docs-html-block-frame')
    expect(container.querySelector('#models-body')).toBeTruthy()
    expect(container.innerHTML).toContain('<script>window.__helpinTest = true</script>')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
