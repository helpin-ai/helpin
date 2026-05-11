// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/gitService', () => ({
  gitService: {
    getTaskGitLinks: vi.fn(),
  },
}))

import { gitService } from '@/lib/services/gitService'
import { queryKeys } from '@/lib/queryKeys'
import { TaskGitPanel } from '../TaskGitPanel'
import type { TaskGitLink } from '@/lib/pmTypes'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

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

describe('TaskGitPanel', () => {
  let root: Root | null = null
  let container: HTMLDivElement | null = null

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
})
