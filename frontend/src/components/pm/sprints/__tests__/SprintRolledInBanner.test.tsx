// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it, vi } from 'vitest'

import { SprintRolledInBanner } from '../SprintRolledInBanner'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('SprintRolledInBanner', () => {
  it('renders rolled-in sources and totals', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <SprintRolledInBanner
          items={[
            { source_sprint_id: 'sprint-12', source_sprint_name: 'Sprint 12', rolled_over_count: 3, rolled_over_points: 11 },
            { source_sprint_id: 'sprint-11', source_sprint_name: 'Sprint 11', rolled_over_count: 1, rolled_over_points: 2 },
          ]}
          onOpenSprint={vi.fn()}
        />,
      )
    })

    expect(container.textContent).toContain('4 tasks')
    expect(container.textContent).toContain('13 pts')
    expect(container.textContent).toContain('Sprint 12')
    expect(container.textContent).toContain('Sprint 11')

    act(() => root.unmount())
    container.remove()
  })
})
