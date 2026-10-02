// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useNotificationView } from '../useNotificationView'
import { isViewingNotificationTarget } from '@/lib/notificationView'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: ReturnType<typeof createRoot>
function View({ workspaceId = 'ws-1', id = 'chat-1', active = true }: { workspaceId?: string; id?: string; active?: boolean }) {
  useNotificationView(workspaceId, 'chat', id, active)
  return null
}
beforeEach(() => {
  root = createRoot(document.createElement('div'))
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
})
afterEach(() => { act(() => root.unmount()); vi.restoreAllMocks() })

it('stops suppressing when a view closes, switches item, or changes workspace', () => {
  act(() => root.render(<View />))
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(true)
  act(() => root.render(<View active={false} />))
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(false)
  act(() => root.render(<View id="chat-2" workspaceId="ws-2" />))
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(false)
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-2')).toBe(false)
  expect(isViewingNotificationTarget('ws-2', 'chat', 'chat-2')).toBe(true)
  act(() => root.render(null))
  expect(isViewingNotificationTarget('ws-2', 'chat', 'chat-2')).toBe(false)
})

it('keeps tracking a view when another instance of the same item unmounts', () => {
  act(() => root.render(<><View key="one" /><View key="two" /></>))
  act(() => root.render(<><View key="one" /></>))
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(true)
  act(() => root.render(null))
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(false)
})

it('requires the page to be visible and focused', () => {
  act(() => root.render(<View />))
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(false)
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
  expect(isViewingNotificationTarget('ws-1', 'chat', 'chat-1')).toBe(false)
})
