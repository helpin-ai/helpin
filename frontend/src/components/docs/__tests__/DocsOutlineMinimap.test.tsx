// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { DocsOutlineMinimap } from '../DocsOutlineMinimap'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('DocsOutlineMinimap', () => {
  it('uses visible inactive dash styling', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <DocsOutlineMinimap
          items={[
            { index: 0, level: 2, text: 'Overview' },
            { index: 1, level: 2, text: 'Setup' },
            { index: 2, level: 3, text: 'Details' },
          ]}
          scrollContainer={null}
          onSelect={vi.fn()}
        />,
      )
    })

    const setupDash = container.querySelector('button[aria-label="Setup"] span')
    expect(setupDash?.className).toContain('bg-muted-foreground/55')
    expect(setupDash?.className).toContain('group-hover/dash:bg-muted-foreground')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
