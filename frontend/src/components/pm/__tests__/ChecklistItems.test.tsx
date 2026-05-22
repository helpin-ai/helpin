// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { pmChecklistService } from '@/lib/services/pmChecklistService'
import type { ChecklistItem } from '@/lib/pmTypes'
import type { AssignableMember } from '@/lib/types'
import { buildChecklistMentionOptions, ChecklistItems } from '../ChecklistItems'

vi.mock('@/lib/services/pmChecklistService', () => ({
  pmChecklistService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

vi.stubGlobal(
  'ResizeObserver',
  class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
)
window.HTMLElement.prototype.scrollIntoView = vi.fn()

let root: Root | null = null
let container: HTMLDivElement | null = null

const checklistMember: AssignableMember = {
  id: 'member-1',
  user_id: 'user-1',
  role: 'member',
  email: 'alice@example.com',
  display_name: 'Alice Smith',
  status: 'active',
}

function makeChecklistItem(overrides: Partial<ChecklistItem> = {}): ChecklistItem {
  return {
    id: 'item-1',
    workspace_id: 'ws-1',
    task_id: 'task-1',
    text: 'Ship the fix',
    completed: false,
    position: 0,
    created_at: '2026-05-13T00:00:00Z',
    updated_at: '2026-05-13T00:00:00Z',
    ...overrides,
  } as ChecklistItem
}

async function renderChecklist(items: ChecklistItem[], members: AssignableMember[] = [checklistMember]) {
  vi.mocked(pmChecklistService.list).mockResolvedValue({ data: items, error: null, status: 200 })

  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)

  await act(async () => {
    root?.render(
      <TooltipProvider>
        <ChecklistItems workspaceId="ws-1" taskId="task-1" members={members} />
      </TooltipProvider>,
    )
  })
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
}

afterEach(() => {
  if (root) {
    act(() => root?.unmount())
  }
  container?.remove()
  root = null
  container = null
  vi.clearAllMocks()
})

describe('buildChecklistMentionOptions', () => {
  it('returns active members and teams with their respective handles', () => {
    const results = buildChecklistMentionOptions(
      '',
      [
        {
          id: 'member-1',
          user_id: 'user-1',
          role: 'member',
          email: 'alice@example.com',
          display_name: 'Alice Smith',
          status: 'active',
        },
        {
          id: 'member-2',
          user_id: 'user-2',
          role: 'member',
          email: 'bob@example.com',
          display_name: 'Bob Jones',
          status: 'inactive',
        },
      ],
      [
        {
          id: 'team-1',
          name: 'Engineering',
          handle: 'engineering',
        },
        {
          id: 'team-2',
          name: 'Platform',
        },
      ],
    )

    expect(results).toEqual([
      {
        avatarUrl: undefined,
        handle: 'alice.smith',
        id: 'user-1',
        label: 'Alice Smith',
        secondaryText: 'alice@example.com',
        type: 'member',
      },
      {
        avatarUrl: undefined,
        handle: 'bob.jones',
        id: 'user-2',
        label: 'Bob Jones',
        secondaryText: 'bob@example.com',
        type: 'member',
      },
      {
        handle: 'engineering',
        id: 'team-1',
        label: 'Engineering',
        secondaryText: '@engineering',
        type: 'team',
      },
    ])
  })

  it('filters results by either member or team text matches', () => {
    const results = buildChecklistMentionOptions(
      'eng',
      [
        {
          id: 'member-1',
          role: 'member',
          email: 'engineer@example.com',
          display_name: 'Jamie',
          status: 'active',
        },
      ],
      [
        {
          id: 'team-1',
          name: 'Engineering',
          handle: 'engineering',
        },
      ],
    )

    expect(results).toEqual([
      {
        handle: 'engineering',
        id: 'team-1',
        label: 'Engineering',
        secondaryText: '@engineering',
        type: 'team',
      },
      {
        avatarUrl: undefined,
        handle: 'jamie',
        id: 'member-1',
        label: 'Jamie',
        secondaryText: 'engineer@example.com',
        type: 'member',
      },
    ])
  })
})

describe('ChecklistItems', () => {
  it('labels the assignee picker trigger for tooltips and assigned state', async () => {
    await renderChecklist([
      makeChecklistItem({ assignee_id: 'user-1' }),
    ])

    expect(container?.querySelector('button[aria-label="Assigned to Alice Smith"]')).toBeTruthy()
  })

  it('unassigns a checklist item through the member picker none option', async () => {
    vi.mocked(pmChecklistService.update).mockResolvedValue({ data: null, error: null, status: 200 })
    await renderChecklist([
      makeChecklistItem({ assignee_id: 'user-1' }),
    ])

    const trigger = container?.querySelector<HTMLButtonElement>('button[aria-label="Assigned to Alice Smith"]')
    expect(trigger).toBeTruthy()

    await act(async () => {
      trigger?.click()
    })

    const unassignedOption = Array.from(document.querySelectorAll('[cmdk-item]')).find((element) =>
      element.textContent?.includes('Unassigned'),
    ) as HTMLElement | undefined
    expect(unassignedOption).toBeTruthy()

    await act(async () => {
      unassignedOption?.click()
    })

    expect(pmChecklistService.update).toHaveBeenCalledWith('ws-1', 'item-1', { assignee_id: '' })
  })

  it('keeps the unassigned picker as a far-right hover action labeled Assign', async () => {
    await renderChecklist([
      makeChecklistItem(),
    ])

    const trigger = container?.querySelector<HTMLButtonElement>('button[aria-label="Assign"]')
    expect(trigger).toBeTruthy()
    expect(trigger?.className).toContain('ml-auto')
    expect(trigger?.className).toContain('opacity-0')
    expect(trigger?.className).toContain('group-hover:opacity-100')
  })

  it('uses green for in-progress checklist progress', async () => {
    await renderChecklist([
      makeChecklistItem({ id: 'item-1', completed: true, position: 0 }),
      makeChecklistItem({ id: 'item-2', completed: false, position: 1 }),
    ])

    const progressFill = container?.querySelector<HTMLElement>('[style*="width: 50%"]')

    expect(progressFill).toBeTruthy()
    expect(progressFill?.className).toContain('bg-green-500')
    expect(progressFill?.className).not.toContain('bg-primary')
  })
})
