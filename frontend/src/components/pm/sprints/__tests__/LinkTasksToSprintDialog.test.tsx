// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Task } from '@/lib/pmTypes'

const serviceMocks = vi.hoisted(() => ({
  list: vi.fn(),
  linkTasks: vi.fn(),
}))

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: { list: serviceMocks.list },
}))

vi.mock('@/lib/services/pmSprintService', () => ({
  pmSprintService: { linkTasks: serviceMocks.linkTasks },
}))

import { LinkTasksToSprintDialog } from '../LinkTasksToSprintDialog'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const baseTask = {
  workspace_id: 'ws-1',
  description: '',
  task_type: 'feature',
  workflow_id: 'wf-1',
  workflow_state_id: 'state-1',
  team_id: 'team-a',
  priority: 'medium',
  severity: 'none',
  position: 0,
  started: false,
  completed: false,
  blocked: false,
  archived: false,
  created_at: '2026-08-06T00:00:00Z',
  updated_at: '2026-08-06T00:00:00Z',
  state_name: 'Todo',
  team_name: 'Alpha',
} satisfies Partial<Task>

const candidates: Task[] = [
  { ...baseTask, id: 'task-backlog', display_id: 51, task_key: 'HLP-51', name: 'Backlog task' } as Task,
  { ...baseTask, id: 'task-move', display_id: 52, task_key: 'HLP-52', name: 'Move this task', sprint_id: 'sprint-old', sprint_name: 'Old sprint' } as Task,
]

function findButton(label: string) {
  return Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent?.trim() === label)
}

async function renderDialog() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const onLinked = vi.fn()
  const onOpenChange = vi.fn()

  act(() => {
    root.render(
      <LinkTasksToSprintDialog
        open
        onOpenChange={onOpenChange}
        workspaceId="ws-1"
        sprintId="sprint-target"
        sprintName="Target sprint"
        teamId="team-a"
        teamName="Alpha"
        onLinked={onLinked}
      />,
    )
  })
  await act(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 0))
    await Promise.resolve()
  })

  return { root, onLinked, onOpenChange }
}

describe('LinkTasksToSprintDialog', () => {
  beforeEach(() => {
    serviceMocks.list.mockResolvedValue({
      data: { data: candidates, total: 2, page: 1, per_page: 50, total_pages: 1 },
      error: null,
    })
    serviceMocks.linkTasks.mockResolvedValue({ data: { linked_count: 1, moved_count: 0 }, error: null })
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('shows same-team backlog and current-sprint context', async () => {
    const rendered = await renderDialog()

    expect(document.body.textContent).toContain('Backlog')
    expect(document.body.textContent).toContain('In another sprint')
    expect(document.body.textContent).toContain('Alpha')
    expect(document.body.textContent).toContain('Old sprint')
    expect(serviceMocks.list).toHaveBeenCalledWith('ws-1', expect.objectContaining({ team_id: 'team-a', archived: false }))

    act(() => rendered.root.unmount())
  })

  it('requires review before moving a task from another sprint', async () => {
    const rendered = await renderDialog()

    act(() => {
      document.body.querySelector<HTMLButtonElement>('[data-task-id="task-move"]')?.click()
    })
    expect(findButton('Review changes')).toBeTruthy()

    act(() => findButton('Review changes')?.click())
    expect(document.body.textContent).toContain('Move from Old sprint')
    expect(findButton('Move and link 1 task')).toBeTruthy()

    await act(async () => {
      findButton('Move and link 1 task')?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(serviceMocks.linkTasks).toHaveBeenCalledWith('ws-1', 'sprint-target', { task_ids: ['task-move'] })
    expect(rendered.onLinked).toHaveBeenCalledWith({ linked_count: 1, moved_count: 0 })
    expect(rendered.onOpenChange).toHaveBeenCalledWith(false)

    act(() => rendered.root.unmount())
  })

  it('links a backlog task without a review step', async () => {
    const rendered = await renderDialog()

    act(() => {
      document.body.querySelector<HTMLButtonElement>('[data-task-id="task-backlog"]')?.click()
    })
    expect(findButton('Link 1 task')).toBeTruthy()

    await act(async () => {
      findButton('Link 1 task')?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(serviceMocks.linkTasks).toHaveBeenCalledWith('ws-1', 'sprint-target', { task_ids: ['task-backlog'] })

    act(() => rendered.root.unmount())
  })
})
