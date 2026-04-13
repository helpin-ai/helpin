// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'

import { SprintCloseoutSummary } from '../SprintCloseoutSummary'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('SprintCloseoutSummary', () => {
  it('renders committed, completed, unfinished, and rolled over totals', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <SprintCloseoutSummary
          closeout={{
            id: 'closeout-1',
            sprint_id: 'sprint-1',
            workspace_id: 'ws-1',
            committed_count: 12,
            completed_count: 8,
            unfinished_count: 4,
            rolled_over_count: 3,
            committed_points: 34,
            completed_points: 21,
            unfinished_points: 13,
            rolled_over_points: 11,
            closed_at: '2026-04-13T00:00:00Z',
            created_at: '2026-04-13T00:00:00Z',
            updated_at: '2026-04-13T00:00:00Z',
          }}
        />,
      )
    })

    expect(container.textContent).toContain('Committed')
    expect(container.textContent).toContain('12')
    expect(container.textContent).toContain('34 pts')
    expect(container.textContent).toContain('Completed')
    expect(container.textContent).toContain('8')
    expect(container.textContent).toContain('Rolled over')

    act(() => root.unmount())
    container.remove()
  })
})
