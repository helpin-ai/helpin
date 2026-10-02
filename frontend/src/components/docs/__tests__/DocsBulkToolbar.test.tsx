// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DocsDocument } from '@/lib/docsTypes'
import { DocsBulkToolbar, DocsSelectionCheckbox } from '../DocsBulkToolbar'
import { DocsLibraryList, DocsLibraryRow } from '../DocsLibraryList'
import { useDocsSelection, type DocsSelection } from '../useDocsSelection'

const mocks = vi.hoisted(() => ({
  archive: vi.fn(), restore: vi.fn(), publish: vi.fn(), delete: vi.fn(), move: vi.fn(), open: vi.fn(), success: vi.fn(), error: vi.fn(),
}))
vi.mock('@/lib/services/docsService', () => ({ docsService: {
  archiveDocument: mocks.archive, unarchiveDocument: mocks.restore, publishDocument: mocks.publish,
  deleteDocument: mocks.delete, moveDocument: mocks.move,
} }))
vi.mock('sonner', () => ({ toast: { success: mocks.success, error: mocks.error } }))
vi.mock('@/hooks/queries', () => ({
  useDocsSpaces: () => ({ data: [{ id: 'space', name: 'Guides' }, { id: 'other', name: 'Team handbook' }] }),
  useDocsCollections: () => ({ data: [] }),
  useMoveDocsDocument: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
const doc = (id: string, status: DocsDocument['status'] = 'draft', is_locked = false) => ({
  id, title: id, status, is_locked, space_id: 'space', updated_at: '2026-09-01T00:00:00Z',
}) as DocsDocument
let latest: DocsSelection
function Harness({ scope = 'all', docs }: { scope?: string; docs: DocsDocument[] }) {
  const selection = useDocsSelection(scope, docs)
  useEffect(() => { latest = selection }, [selection])
  return <>
    <DocsBulkToolbar key={scope} wsId="workspace" selection={selection} />
    <DocsLibraryList>{docs.map(d => <DocsLibraryRow key={d.id} title={d.title} status={d.status} updatedAt={d.updated_at}
      selection={<DocsSelectionCheckbox doc={d} selection={selection} />} onOpen={mocks.open} />)}</DocsLibraryList>
  </>
}
let root: Root
let container: HTMLDivElement
let client: QueryClient
const button = (label: string) => Array.from(document.querySelectorAll('button')).find(b => b.textContent === label)!
const checkbox = (label: string) => document.querySelector(`[aria-label="${label}"]`) as HTMLButtonElement
async function click(element: HTMLElement) { expect(element).toBeTruthy(); await act(async () => element.click()) }
async function choose(label: string) {
  await act(async () => button('Actions').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })))
  const item = Array.from(document.querySelectorAll('[role="menuitem"]')).find(e => e.textContent === label) as HTMLElement
  await click(item)
}
function render(docs: DocsDocument[], scope = 'all') {
  act(() => root.render(<QueryClientProvider client={client}><Harness docs={docs} scope={scope} /></QueryClientProvider>))
}
beforeEach(() => {
  vi.clearAllMocks()
  HTMLElement.prototype.scrollIntoView = vi.fn()
  for (const fn of [mocks.archive, mocks.restore, mocks.publish, mocks.delete, mocks.move]) fn.mockResolvedValue({ data: {}, error: null })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})
afterEach(() => { act(() => root.unmount()); client.clear(); document.body.innerHTML = '' })

describe('Docs bulk selection', () => {
  it('selects visible unlocked rows without opening documents and exposes mixed selection', async () => {
    render([doc('one'), doc('two'), doc('locked', 'draft', true)])
    await click(checkbox('Select one'))
    expect(mocks.open).not.toHaveBeenCalled()
    expect(checkbox('Select all visible documents').getAttribute('aria-checked')).toBe('mixed')
    await click(checkbox('Select all visible documents'))
    expect(latest.ids).toEqual(['one', 'two'])
    expect(checkbox('Select locked').disabled).toBe(true)
    await click(button('Clear'))
    expect(latest.ids).toEqual([])
  })
  it('clears on scope changes and forgets rows removed from the visible list', async () => {
    render([doc('one'), doc('two')])
    await click(checkbox('Select all visible documents'))
    render([doc('two')])
    expect(latest.ids).toEqual(['two'])
    render([doc('one'), doc('two')])
    expect(latest.ids).toEqual(['two'])
    render([doc('one'), doc('two')], 'different-collection')
    expect(latest.ids).toEqual([])
  })
  it('keeps failures selected, reports API errors, and refreshes once', async () => {
    mocks.archive.mockImplementation(async (_ws: string, id: string) => ({ data: null, error: id === 'two' ? 'No access' : null }))
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    render([doc('one'), doc('two')])
    await click(checkbox('Select all visible documents'))
    await choose('Archive')
    expect(mocks.archive).toHaveBeenCalledTimes(2)
    expect(latest.ids).toEqual(['two'])
    expect(mocks.error).toHaveBeenCalledWith('1 archived, 1 failed', { description: 'two: No access' })
    expect(invalidate).toHaveBeenCalledOnce()
  })
  it('restores only archived documents in a mixed selection', async () => {
    render([doc('one'), doc('old', 'archived')])
    await click(checkbox('Select all visible documents'))
    await choose('Restore (1)')
    expect(mocks.restore).toHaveBeenCalledOnce()
    expect(mocks.restore).toHaveBeenCalledWith('workspace', 'old')
    expect(latest.ids).toEqual(['one'])
  })
  it('requires explicit confirmation before publishing or deleting', async () => {
    render([doc('one')])
    await click(checkbox('Select one'))
    await choose('Publish')
    expect(mocks.publish).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('published publicly')
    await click(button('Cancel'))
    expect(mocks.publish).not.toHaveBeenCalled()
    await choose('Publish')
    await click(button('Publish'))
    expect(mocks.publish).toHaveBeenCalledOnce()
    expect(mocks.publish).toHaveBeenCalledWith('workspace', 'one')
    await click(checkbox('Select one'))
    await choose('Delete')
    expect(mocks.delete).not.toHaveBeenCalled()
    await click(button('Delete'))
    expect(mocks.delete).toHaveBeenCalledOnce()
    expect(mocks.delete).toHaveBeenCalledWith('workspace', 'one')
  })
  it('moves selected documents to the chosen space and uncategorized destination', async () => {
    render([doc('one'), doc('two')])
    await click(checkbox('Select all visible documents'))
    await choose('Move')
    expect(mocks.move).not.toHaveBeenCalled()
    const spacePicker = checkbox('Destination space')
    await act(async () => spacePicker.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })))
    await click(Array.from(document.querySelectorAll('[role=option]')).find(e => e.textContent?.includes('Team handbook')) as HTMLElement)
    await click(button('Move'))
    expect(mocks.move).toHaveBeenCalledTimes(2)
    expect(mocks.move).toHaveBeenCalledWith('workspace', 'one', { space_id: 'other', collection_id: undefined })
    expect(latest.ids).toEqual([])
  })
  it('disables selection and duplicate actions while a batch is running', async () => {
    let finish!: (value: unknown) => void
    mocks.archive.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    render([doc('one')])
    await click(checkbox('Select one'))
    await choose('Archive')
    expect(checkbox('Select one').disabled).toBe(true)
    expect(button('Actions').disabled).toBe(true)
    await act(async () => finish({ data: {}, error: null }))
    expect(latest.busy).toBe(false)
    expect(mocks.archive).toHaveBeenCalledOnce()
  })
})
