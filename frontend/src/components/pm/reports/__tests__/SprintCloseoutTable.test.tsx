// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { describe, expect, it } from 'vitest'

import { SprintCloseoutTable } from '../SprintCloseoutTable'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

describe('SprintCloseoutTable', () => {
  it('renders sprint closeout rows and completion percentages', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <SprintCloseoutTable
          items={[
            {
              closeout_id: 'closeout-1',
              sprint_id: 'sprint-1',
              sprint_name: 'Sprint 22',
              team_id: 'team-1',
              team_name: 'Growth',
              committed_count: 10,
              completed_count: 7,
              unfinished_count: 3,
              rolled_over_count: 2,
              committed_points: 24,
              completed_points: 17,
              unfinished_points: 7,
              rolled_over_points: 5,
              completion_rate: 0.7,
              rolled_to_sprint_name: 'Sprint 23',
              closed_at: '2026-04-13T00:00:00Z',
            },
          ]}
        />,
      )
    })

    expect(container.textContent).toContain('Sprint 22')
    expect(container.textContent).toContain('Growth')
    expect(container.textContent).toContain('70%')
    expect(container.textContent).toContain('Sprint 23')

    act(() => root.unmount())
    container.remove()
  })
})
