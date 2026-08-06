// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { SprintPlanningColumnActions } from '../SprintPlanningColumnActions'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('SprintPlanningColumnActions', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('places link tasks alongside create task', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const onLinkTasks = vi.fn()
    const onCreateTask = vi.fn()

    act(() => {
      root.render(<SprintPlanningColumnActions onLinkTasks={onLinkTasks} onCreateTask={onCreateTask} />)
    })

    const buttons = Array.from(container.querySelectorAll('button'))
    expect(buttons.map((button) => button.textContent?.trim())).toEqual(['Link tasks', 'Create task'])

    act(() => buttons[0]?.click())
    act(() => buttons[1]?.click())
    expect(onLinkTasks).toHaveBeenCalledOnce()
    expect(onCreateTask).toHaveBeenCalledOnce()

    act(() => root.unmount())
  })

  it('disables linking with team guidance for a legacy sprint', () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <SprintPlanningColumnActions
          linkTasksDisabledReason="Assign this sprint to a team before linking tasks."
          onLinkTasks={() => undefined}
          onCreateTask={() => undefined}
        />,
      )
    })

    const linkButton = container.querySelector<HTMLButtonElement>('button')
    expect(linkButton?.disabled).toBe(true)
    expect(linkButton?.title).toBe('Assign this sprint to a team before linking tasks.')

    act(() => root.unmount())
  })
})
