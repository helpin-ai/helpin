// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { CrmRailNav } from '../CrmRailNav'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('CrmRailNav', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('keeps the action footer in sidebar flow instead of overlaying the trial banner', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <TooltipProvider>
          <CrmRailNav
            groups={[]}
            isActive={() => false}
            wsSlug="workspace"
            onNavigate={vi.fn()}
            onNavigateTo={vi.fn()}
          />
        </TooltipProvider>,
      )
    })

    const footer = container.querySelector('[data-slot="crm-module-footer"]')
    expect(footer?.classList.contains('sticky')).toBe(true)
    expect(footer?.classList.contains('fixed')).toBe(false)
    expect(footer?.classList.contains('mt-auto')).toBe(true)

    act(() => root.unmount())
    container.remove()
  })
})
