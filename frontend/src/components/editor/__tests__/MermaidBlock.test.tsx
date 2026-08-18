// @vitest-environment jsdom

import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MermaidBlock } from '../MermaidBlock'
import { preloadMermaidDiagrams } from '../mermaidPreviewCache'

const testState = vi.hoisted(() => ({
  renderMermaidSvg: vi.fn<(source: string) => Promise<string>>(),
}))

vi.mock('@/lib/mermaidRenderer', () => ({
  renderMermaidSvg: (source: string) => testState.renderMermaidSvg(source),
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | null = null

afterEach(() => {
  if (root) act(() => root?.unmount())
  root = null
  document.body.innerHTML = ''
  testState.renderMermaidSvg.mockReset()
})

function mount(source: string) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root?.render(<MermaidBlock source={source} />))
  return container
}

describe('MermaidBlock', () => {
  it('keeps the loading and rendered diagram footprints stable', async () => {
    let resolveRender: ((svg: string) => void) | undefined
    testState.renderMermaidSvg.mockReturnValue(new Promise((resolve) => {
      resolveRender = resolve
    }))

    const container = mount('graph TD\nA-->B')
    const loading = container.querySelector('[data-mermaid-status="loading"]')
    expect(loading?.className).toContain('min-h-40')

    await act(async () => resolveRender?.('<svg viewBox="0 0 100 100"></svg>'))

    const ready = container.querySelector('[data-mermaid-status="ready"]')
    expect(ready?.className).toContain('min-h-40')
    expect(ready?.className).toContain('[overflow-anchor:none]')
  })

  it('reuses a rendered diagram when it is mounted again', async () => {
    const source = 'graph TD\nCached-->Diagram'
    testState.renderMermaidSvg.mockResolvedValue('<svg data-cached="true"></svg>')
    const firstContainer = mount(source)

    await act(async () => undefined)
    expect(firstContainer.querySelector('[data-mermaid-status="ready"]')).toBeTruthy()
    expect(testState.renderMermaidSvg).toHaveBeenCalledOnce()

    act(() => root?.unmount())
    root = null
    document.body.innerHTML = ''

    const secondContainer = mount(source)
    expect(secondContainer.querySelector('[data-mermaid-status="ready"]')).toBeTruthy()
    expect(secondContainer.querySelector('[data-mermaid-status="loading"]')).toBeNull()
    expect(testState.renderMermaidSvg).toHaveBeenCalledOnce()
  })

  it('preloads each unique diagram once before the editor mounts', async () => {
    const source = 'graph TD\nPreloaded-->Once'
    testState.renderMermaidSvg.mockResolvedValue('<svg data-preloaded="true"></svg>')

    await preloadMermaidDiagrams([source, source, '  '])

    expect(testState.renderMermaidSvg).toHaveBeenCalledOnce()
    expect(testState.renderMermaidSvg).toHaveBeenCalledWith(source)
  })
})
