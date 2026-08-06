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

vi.mock('@/lib/services/pmEpicService', () => ({
  pmEpicService: { linkTasks: serviceMocks.linkTasks },
}))

import { LinkTasksToEpicDialog } from '../LinkTasksToEpicDialog'

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
  { ...baseTask, id: 'task-new', display_id: 41, task_key: 'HLP-41', name: 'Unassigned task' } as Task,
  { ...baseTask, id: 'task-move', display_id: 42, task_key: 'HLP-42', name: 'Move this task', epic_id: 'epic-old', epic_name: 'Old epic' } as Task,
]

function findButton(label: string) {
  return Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent?.trim() === label)
}

async function renderDialog(overrides: Partial<React.ComponentProps<typeof LinkTasksToEpicDialog>> = {}) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const onLinked = vi.fn()
  const onOpenChange = vi.fn()

  act(() => {
    root.render(
      <LinkTasksToEpicDialog
        open
        onOpenChange={onOpenChange}
        workspaceId="ws-1"
        epicId="epic-target"
        epicName="Target epic"
        teamId="team-a"
        teamName="Alpha"
        onLinked={onLinked}
        {...overrides}
      />,
    )
  })
  await act(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 0))
    await Promise.resolve()
  })

  return { root, onLinked, onOpenChange }
}

describe('LinkTasksToEpicDialog', () => {
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

  it('shows same-team candidates with team and current-epic context', async () => {
    const rendered = await renderDialog()

    expect(document.body.textContent).toContain('No epic')
    expect(document.body.textContent).toContain('In another epic')
    expect(document.body.textContent).toContain('Alpha')
    expect(document.body.textContent).toContain('Old epic')
    expect(serviceMocks.list).toHaveBeenCalledWith('ws-1', expect.objectContaining({ team_id: 'team-a', archived: false }))

    act(() => rendered.root.unmount())
  })

  it('requires review before moving a task from another epic', async () => {
    const rendered = await renderDialog()

    act(() => {
      document.body.querySelector<HTMLButtonElement>('[data-task-id="task-move"]')?.click()
    })
    expect(findButton('Review changes')).toBeTruthy()

    act(() => findButton('Review changes')?.click())
    expect(document.body.textContent).toContain('Move from Old epic')
    expect(findButton('Move and link 1 task')).toBeTruthy()

    await act(async () => {
      findButton('Move and link 1 task')?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(serviceMocks.linkTasks).toHaveBeenCalledWith('ws-1', 'epic-target', { task_ids: ['task-move'] })
    expect(rendered.onLinked).toHaveBeenCalledWith({ linked_count: 1, moved_count: 0 })
    expect(rendered.onOpenChange).toHaveBeenCalledWith(false)

    act(() => rendered.root.unmount())
  })

  it('links an unassigned task without a review step', async () => {
    const rendered = await renderDialog()

    act(() => {
      document.body.querySelector<HTMLButtonElement>('[data-task-id="task-new"]')?.click()
    })
    expect(findButton('Link 1 task')).toBeTruthy()

    await act(async () => {
      findButton('Link 1 task')?.click()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(serviceMocks.linkTasks).toHaveBeenCalledWith('ws-1', 'epic-target', { task_ids: ['task-new'] })

    act(() => rendered.root.unmount())
  })
})
