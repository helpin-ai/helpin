// @vitest-environment jsdom
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { notifyManager, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterAll, afterEach, beforeAll, expect, it, vi } from 'vitest'
import { useCoverageInsightPages } from '../useCoverageInsightPages'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
let root: Root | null = null
let container: HTMLDivElement
let client: QueryClient
let latest: ReturnType<typeof useCoverageInsightPages<{ id: string }>>
beforeAll(() => notifyManager.setNotifyFunction(callback => act(callback)))
afterAll(() => notifyManager.setNotifyFunction(callback => callback()))
afterEach(() => {
  act(() => root?.unmount())
  root = null
  container?.remove()
  client?.clear()
})

const records = Array.from({ length: 31 }, (_, index) => ({ id: String(index) }))

async function renderList(loader: ReturnType<typeof vi.fn>, workspaceId = 'ws-1') {
  if (!root) {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  }
  function List() {
    latest = useCoverageInsightPages(workspaceId, 'topics', true, loader)
    return <div>{latest.items.map(item => <span key={item.id}>{item.id}</span>)}</div>
  }
  await act(async () => {
    root!.render(<QueryClientProvider client={client}><List /></QueryClientProvider>)
    await new Promise(resolve => setTimeout(resolve, 10))
  })
  await vi.waitFor(() => expect(latest.loading).toBe(false))
}

it('loads results beyond the first page and exposes the complete count', async () => {
  const loader = vi.fn(async (_workspace: string, limit: number, offset: number) => ({ data: { items: records.slice(offset, offset + limit), total: records.length }, error: null }))
  await renderList(loader)
  expect(latest.items).toHaveLength(25)
  expect(latest.total).toBe(31)
  expect(latest.hasMore).toBe(true)
  await act(async () => { await latest.loadMore(); await new Promise(resolve => setTimeout(resolve, 10)) })
  await vi.waitFor(() => expect(latest.items).toHaveLength(31))
  expect(latest.hasMore).toBe(false)
})

it('keeps loaded results when another page fails and allows retry', async () => {
  let fail = true
  const loader = vi.fn(async (_workspace: string, limit: number, offset: number) => offset && fail
    ? { data: null, error: 'Unavailable' }
    : { data: { items: records.slice(offset, offset + limit), total: records.length }, error: null })
  await renderList(loader)
  await act(async () => { await latest.loadMore(); await new Promise(resolve => setTimeout(resolve, 10)) })
  await vi.waitFor(() => expect(latest.loadMoreError).toBeTruthy())
  expect(latest.items).toHaveLength(25)
  fail = false
  await act(async () => { await latest.loadMore(); await new Promise(resolve => setTimeout(resolve, 10)) })
  await vi.waitFor(() => expect(latest.items).toHaveLength(31))
})

it('refreshes all loaded pages after a signal is removed without skipping records', async () => {
  let remaining = [...records]
  const loader = vi.fn(async (_workspace: string, limit: number, offset: number) => ({ data: { items: remaining.slice(offset, offset + limit), total: remaining.length }, error: null }))
  await renderList(loader)
  await act(async () => { await latest.loadMore(); await new Promise(resolve => setTimeout(resolve, 10)) })
  remaining = remaining.slice(1)
  await act(async () => { await latest.refresh(); await new Promise(resolve => setTimeout(resolve, 10)) })
  await vi.waitFor(() => expect(latest.items).toHaveLength(30))
  expect(latest.items.map(item => item.id)).toEqual(remaining.map(item => item.id))
})

it('never shows the previous workspace results while a new workspace loads', async () => {
  const loader = vi.fn(async (workspace: string) => ({ data: { items: [{ id: workspace }], total: 1 }, error: null }))
  await renderList(loader)
  await renderList(loader, 'ws-2')
  await vi.waitFor(() => expect(latest.items.map(item => item.id)).toEqual(['ws-2']))
})
