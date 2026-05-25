// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/gitService', () => ({
  gitService: {
    getTaskGitLinks: vi.fn(),
    getTaskDeliveryTarget: vi.fn(),
  },
}))

import { gitService } from '@/lib/services/gitService'
import { queryKeys } from '@/lib/queryKeys'
import { TaskGitPanel } from '../TaskGitPanel'
import type { TaskDeliveryTarget, TaskGitLink } from '@/lib/pmTypes'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root | null = null
let container: HTMLDivElement | null = null

function makeLink(prStatus: string): TaskGitLink {
  return {
    id: 'link-1',
    workspace_id: 'ws-1',
    task_id: 'task-1',
    integration_id: 'integration-1',
    provider: 'github',
    repo: 'helpin-ai/helpin',
    branch: 'feature/task-1',
    pr_number: 31,
    pr_title: 'Fix task status sync',
    pr_url: 'https://github.com/helpin-ai/helpin/pull/31',
    pr_status: prStatus,
    created_at: '2026-04-23T08:00:00Z',
    updated_at: '2026-04-23T08:00:00Z',
  }
}

function makeDeliveryTarget(overrides: Partial<TaskDeliveryTarget> = {}): TaskDeliveryTarget {
  return {
    id: 'delivery-1',
    workspace_id: 'ws-1',
    task_id: 'task-1',
    repo_full_name: 'helpin-ai/helpin',
    base_branch: 'main',
    working_branch: 'feature/task-1',
    delivery_state: 'active',
    active_pr_number: 31,
    active_pr_title: 'Fix task status sync',
    active_pr_url: 'https://github.com/helpin-ai/helpin/pull/31',
    active_pr_status: 'open',
    last_commit_sha: 'abcdef1234567890',
    last_synced_at: '2026-04-23T08:00:00Z',
    created_at: '2026-04-23T08:00:00Z',
    updated_at: '2026-04-23T08:00:00Z',
    ...overrides,
  }
}

async function renderPanel(links: TaskGitLink[], deliveryTarget: TaskDeliveryTarget | null = makeDeliveryTarget()) {
  vi.mocked(gitService.getTaskGitLinks)
    .mockResolvedValueOnce({ data: links, error: null, status: 200 })
  vi.mocked(gitService.getTaskDeliveryTarget)
    .mockResolvedValueOnce({ data: deliveryTarget, error: null, status: deliveryTarget ? 200 : 404 })

  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)

  await act(async () => {
    root?.render(
      <QueryClientProvider client={client}>
        <TaskGitPanel workspaceId="ws-1" taskId="task-1" />
      </QueryClientProvider>,
    )
  })
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })

  return client
}

describe('TaskGitPanel', () => {
  afterEach(() => {
    if (root) {
      act(() => root?.unmount())
    }
    container?.remove()
    root = null
    container = null
    vi.clearAllMocks()
  })

  it('reloads development history when the task git links query is invalidated', async () => {
    vi.mocked(gitService.getTaskGitLinks)
      .mockResolvedValueOnce({ data: [makeLink('open')], error: null, status: 200 })
      .mockResolvedValueOnce({ data: [makeLink('merged')], error: null, status: 200 })
    vi.mocked(gitService.getTaskDeliveryTarget)
      .mockResolvedValue({ data: makeDeliveryTarget(), error: null, status: 200 })

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)

    await act(async () => {
      root?.render(
        <QueryClientProvider client={client}>
          <TaskGitPanel workspaceId="ws-1" taskId="task-1" />
        </QueryClientProvider>,
      )
    })
    // Wait for the React Query subscription + commit. The mock's promise
    // resolves on the microtask queue, but useQuery's internal observer
    // schedules its commit on a macrotask — yielding once to setTimeout
    // is enough to flush both.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(container.textContent).toContain('open')

    await act(async () => {
      await client.invalidateQueries({ queryKey: queryKeys.git.taskLinks('ws-1', 'task-1') })
      await new Promise((resolve) => setTimeout(resolve, 0))
    })

    expect(gitService.getTaskGitLinks).toHaveBeenCalledTimes(2)
    expect(container.textContent).toContain('merged')
    expect(container.textContent).not.toContain('open')
  })

  it('renders linked git work as compact inferred timeline rows', async () => {
    await renderPanel([
      {
        ...makeLink('open'),
        id: 'link-pr',
        commit_sha: 'abcdef1234567890',
      },
      {
        ...makeLink('open'),
        id: 'link-commit',
        pr_number: undefined,
        pr_title: undefined,
        pr_url: undefined,
        pr_status: undefined,
        commit_sha: '1234567890abcdef',
      },
      {
        ...makeLink('open'),
        id: 'link-branch',
        pr_number: undefined,
        pr_title: undefined,
        pr_url: undefined,
        pr_status: undefined,
        commit_sha: undefined,
      },
    ])

    expect(container.textContent).toContain('Pull request')
    expect(container.textContent).toContain('Commit')
    expect(container.textContent).toContain('Branch')
    expect(container.textContent).toContain('helpin-ai/helpin')
    expect(container.textContent).toContain('feature/task-1')
    expect(container.textContent).toContain('#31')
    expect(container.textContent).toContain('abcdef1')
    expect(container.textContent).toContain('1234567')

    const branchLink = container.querySelector('a[href="https://github.com/helpin-ai/helpin/tree/feature/task-1"]')
    expect(branchLink).toBeTruthy()
    const prLink = container.querySelector('a[href="https://github.com/helpin-ai/helpin/pull/31"]')
    expect(prLink).toBeTruthy()
  })

  it('shows the working branch flowing into the base branch when delivery target data exists', async () => {
    await renderPanel([
      {
        ...makeLink('open'),
        branch: 'chore/hel-46-frequent-logouts-session-not-shared-across-tabs',
        commit_sha: 'd9ff653abcdef123',
      },
    ], makeDeliveryTarget({
      base_branch: 'main',
      working_branch: 'chore/hel-46-frequent-logouts-session-not-shared-across-tabs',
    }))

    expect(gitService.getTaskDeliveryTarget).toHaveBeenCalledWith('ws-1', 'task-1')
    expect(container.textContent).toContain('chore/hel-46-frequent-logouts-session-not-shared-across-tabs')
    expect(container.textContent).toContain('main')
    expect(container.textContent).toContain('→')
    expect(container.textContent).toContain('d9ff653')
  })
})
