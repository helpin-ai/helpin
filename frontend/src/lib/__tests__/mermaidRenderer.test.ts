// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest'

const testState = vi.hoisted(() => ({
  initialize: vi.fn(),
  render: vi.fn(),
}))

vi.mock('mermaid', () => ({
  default: {
    initialize: testState.initialize,
    render: testState.render,
  },
}))

import { renderMermaidSvg } from '../mermaidRenderer'

afterEach(() => {
  testState.initialize.mockReset()
  testState.render.mockReset()
  document.body.innerHTML = ''
})

describe('renderMermaidSvg', () => {
  it('isolates temporary Mermaid output from the page layout', async () => {
    testState.render.mockImplementation(async (_id: string, _source: string, container?: Element) => {
      const host = container as HTMLElement
      expect(host.isConnected).toBe(true)
      expect(host.dataset.mermaidRenderHost).toBe('')
      expect(host.getAttribute('aria-hidden')).toBe('true')
      expect(host.style.position).toBe('fixed')
      expect(host.style.overflow).toBe('hidden')
      expect(host.style.visibility).toBe('hidden')
      expect(host.style.contain).toBe('strict')
      return { svg: '<svg style="background-color: red;"></svg>' }
    })

    const svg = await renderMermaidSvg('graph TD\nA-->B')

    expect(svg).toContain('background-color: transparent;')
    expect(testState.render).toHaveBeenCalledOnce()
    expect(document.querySelector('[data-mermaid-render-host]')).toBeNull()
  })

  it('removes the isolated render host when Mermaid fails', async () => {
    testState.render.mockRejectedValue(new Error('Invalid diagram'))

    await expect(renderMermaidSvg('not a diagram')).rejects.toThrow('Invalid diagram')
    expect(document.querySelector('[data-mermaid-render-host]')).toBeNull()
  })
})
