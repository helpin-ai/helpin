// @vitest-environment jsdom
import type { ReactNode } from 'react'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ResizableImageComponent } from '../resizable-image-component'

vi.mock('@tiptap/react', () => ({
  NodeViewWrapper: ({ children, ...props }: { children: ReactNode }) => <div {...props}>{children}</div>,
}))

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: ReactNode }) => children,
}))

vi.mock('@/components/docs/DocsImageEditDialog', () => ({
  DocsImageEditDialog: () => null,
}))

vi.mock('@/hooks/useImageActions', () => ({
  useImageActions: () => ({
    copyImage: vi.fn(),
    downloadImage: vi.fn(),
    openInNewTab: vi.fn(),
  }),
}))

vi.mock('@/lib/services/automationService', () => ({
  automationService: { getArtifactContentURL: vi.fn() },
}))

vi.mock('@/lib/services/pmAttachmentService', () => ({
  pmAttachmentService: { proxiedContentUrl: vi.fn() },
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const mountedRoots: Array<{ container: HTMLDivElement; root: ReturnType<typeof createRoot> }> = []

function renderImage(updateAttributes: ReturnType<typeof vi.fn>, width = '35%') {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  mountedRoots.push({ container, root })

  act(() => {
    root.render(
      <ResizableImageComponent
        node={{
          attrs: {
            src: 'https://cdn.example.com/product.png',
            width,
            height: 'auto',
            aspectRatio: null,
            alignment: 'center',
          },
        } as never}
        updateAttributes={updateAttributes}
        selected={false}
        deleteNode={vi.fn()}
        editor={{ isEditable: true } as never}
        extension={{ options: { enableCaption: false } } as never}
        decorations={[] as never}
        innerDecorations={[] as never}
        getPos={vi.fn()}
        HTMLAttributes={{}}
      />,
    )
  })

  const image = container.querySelector('img')
  if (!image) throw new Error('Expected the image to render')
  Object.defineProperty(image, 'naturalWidth', { configurable: true, value: 800 })
  Object.defineProperty(image, 'naturalHeight', { configurable: true, value: 400 })

  return { container, image }
}

afterEach(() => {
  for (const { container, root } of mountedRoots.splice(0)) {
    act(() => root.unmount())
    container.remove()
  }
})

describe('ResizableImageComponent', () => {
  it.each(['35%', '500px'])('does not mutate image attributes when a %s image loads', (width) => {
    const updateAttributes = vi.fn()
    const { image } = renderImage(updateAttributes, width)

    act(() => image.dispatchEvent(new Event('load')))

    expect(updateAttributes).not.toHaveBeenCalled()
  })

  it('persists dimensions and aspect ratio after an explicit resize', () => {
    const updateAttributes = vi.fn()
    const { container, image } = renderImage(updateAttributes)

    act(() => image.dispatchEvent(new Event('load')))

    const resizeHandle = container.querySelector<HTMLElement>('.cursor-nwse-resize')
    if (!resizeHandle) throw new Error('Expected the resize handle to render')
    const imageContainer = resizeHandle.parentElement
    if (!imageContainer) throw new Error('Expected the image container to render')
    imageContainer.getBoundingClientRect = () => ({ left: 0 } as DOMRect)

    act(() => resizeHandle.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: 300 })))
    act(() => window.dispatchEvent(new MouseEvent('mousemove', { clientX: 400 })))
    act(() => window.dispatchEvent(new MouseEvent('mouseup')))

    expect(updateAttributes).toHaveBeenCalledTimes(1)
    expect(updateAttributes).toHaveBeenCalledWith({
      width: '400px',
      height: '200px',
      aspectRatio: 2,
    })
  })
})
