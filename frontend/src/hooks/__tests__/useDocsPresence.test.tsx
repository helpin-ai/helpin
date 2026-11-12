// @vitest-environment jsdom
import { act, useLayoutEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useDocsPresence } from '../useDocsPresence'
import { useWSStore } from '../useWebSocket'
import { useDocsPresenceStore } from '@/stores/docsPresenceStore'

vi.mock('@/lib/api', () => ({ API_BASE: 'https://api.example.test/api' }))
vi.mock('@helpin-ai/support-core', () => ({ buildWorkspaceWebSocketUrl: () => '' }))

let root: Root
let visibility: DocumentVisibilityState
let onEditingChange: ReturnType<typeof useDocsPresence>
const send = vi.fn()
function Probe({ documentId = 'doc-1' }: { documentId?: string }) {
  const handleEditingChange = useDocsPresence(documentId)
  useLayoutEffect(() => { onEditingChange = handleEditingChange }, [handleEditingChange])
  return null
}
function changeVisibility(value: DocumentVisibilityState) {
  visibility = value
  act(() => document.dispatchEvent(new Event('visibilitychange')))
}
function seedViewers() {
  useDocsPresenceStore.getState().setViewingUser('doc-1', 'user-2', true, { name: 'Teammate' })
  useDocsPresenceStore.getState().setEditingUser('doc-1', 'user-2', true, { area: 'body' })
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  visibility = 'visible'
  vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
  send.mockClear()
  useWSStore.setState({ send })
  useDocsPresenceStore.setState({ viewingUsers: {}, editingUsers: {} })
  root = createRoot(document.createElement('div'))
})

afterEach(() => {
  act(() => root.unmount())
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

it('stops viewing and editing when hidden and rejoins when visible', () => {
  act(() => root.render(<Probe />))
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:start', data: { document_id: 'doc-1' } })
  send.mockClear()
  changeVisibility('hidden')
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:stop', data: { document_id: 'doc-1' } })
  expect(send).toHaveBeenCalledWith({ type: 'docs:editing:stop', data: { document_id: 'doc-1' } })
  send.mockClear()
  act(() => { vi.advanceTimersByTime(90_000); onEditingChange({ area: 'body' }) })
  expect(send).not.toHaveBeenCalled()
  changeVisibility('visible')
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:start', data: { document_id: 'doc-1' } })
})

it('does not announce a tab that opens in the background', () => {
  visibility = 'hidden'
  act(() => root.render(<Probe />))
  act(() => vi.advanceTimersByTime(90_000))
  expect(send).not.toHaveBeenCalled()
})

it('renews visible presence and requests fresh viewers before the server lease expires', () => {
  act(() => root.render(<Probe />))
  send.mockClear()
  act(() => vi.advanceTimersByTime(30_000))
  // Starting/renewing viewing also returns an authoritative server snapshot.
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:start', data: { document_id: 'doc-1' } })
})

it('clears stale avatars while disconnected and rejoins on reconnect', () => {
  act(() => root.render(<Probe />))
  act(() => { seedViewers(); useWSStore.setState({ send: null }) })
  expect(useDocsPresenceStore.getState().viewingUsers['doc-1']).toEqual({})
  expect(useDocsPresenceStore.getState().editingUsers['doc-1']).toEqual({})
  const reconnectedSend = vi.fn()
  act(() => useWSStore.setState({ send: reconnectedSend }))
  expect(reconnectedSend).toHaveBeenCalledWith({ type: 'docs:viewing:start', data: { document_id: 'doc-1' } })
})

it('leaves the old document on navigation and cleans up timers and listeners', () => {
  act(() => root.render(<Probe />))
  act(() => onEditingChange({ area: 'title', section: 'Title' }))
  expect(send).toHaveBeenCalledWith({ type: 'docs:editing:update', data: { document_id: 'doc-1', area: 'title', section: 'Title' } })
  act(() => { seedViewers(); root.render(<Probe documentId="doc-2" />) })
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:stop', data: { document_id: 'doc-1' } })
  expect(send).toHaveBeenCalledWith({ type: 'docs:editing:stop', data: { document_id: 'doc-1' } })
  expect(useDocsPresenceStore.getState().viewingUsers['doc-1']).toEqual({})
  act(() => root.unmount())
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:stop', data: { document_id: 'doc-2' } })
  send.mockClear()
  act(() => vi.advanceTimersByTime(90_000))
  changeVisibility('hidden')
  changeVisibility('visible')
  expect(send).not.toHaveBeenCalled()
})

it('leaves on pagehide and rejoins a restored page', () => {
  act(() => root.render(<Probe />))
  send.mockClear()
  act(() => window.dispatchEvent(new Event('pagehide')))
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:stop', data: { document_id: 'doc-1' } })
  send.mockClear()
  act(() => vi.advanceTimersByTime(30_000))
  expect(send).not.toHaveBeenCalled()
  act(() => window.dispatchEvent(new Event('pageshow')))
  expect(send).toHaveBeenCalledWith({ type: 'docs:viewing:start', data: { document_id: 'doc-1' } })
})

it('does not announce a document before it is available', () => {
  act(() => root.render(<Probe documentId="" />))
  expect(send).not.toHaveBeenCalled()
})
