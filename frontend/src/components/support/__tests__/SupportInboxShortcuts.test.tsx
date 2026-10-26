// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { SupportConversation } from '@/lib/pmTypes'
import { dispatchSupportShortcut, onSupportShortcut } from '@/lib/supportShortcuts'
import { useSupportInboxShortcuts } from '@/hooks/useSupportInboxShortcuts'
import { useSupportInboxStore } from '@/stores/supportInboxStore'
import { ConversationSnoozeControl } from '../ConversationSnoozeControl'

const snoozeMutate = vi.fn()
vi.mock('@/hooks/queries/useSupportOperations', () => ({
  useSnoozeConversation: () => ({ mutate: snoozeMutate, isPending: false }),
  useUnsnoozeConversation: () => ({ mutate: vi.fn(), isPending: false }),
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function mount(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  act(() => root.render(node))
  return () => act(() => { root.unmount(); container.remove() })
}

afterEach(() => {
  document.body.innerHTML = ''
  snoozeMutate.mockReset()
})

describe('snooze shortcut', () => {
  const conversation = { id: 'c1', status: 'open', snoozed_until: null, merged_into_id: null } as unknown as SupportConversation

  it('opens the snooze menu for the open conversation only', () => {
    const unmount = mount(<ConversationSnoozeControl workspaceId="ws" conversation={conversation} />)
    expect(dispatchSupportShortcut('snooze', 'other')).toBe(false)
    expect(document.body.textContent).not.toContain('Snooze until')
    let handled = false
    act(() => { handled = dispatchSupportShortcut('snooze', 'c1') })
    expect(handled).toBe(true)
    expect(document.body.textContent).toContain('Snooze until')
    expect(document.body.textContent).toContain('Tomorrow')
    unmount()
  })

  it('does nothing for a conversation that cannot be snoozed', () => {
    const unmount = mount(<ConversationSnoozeControl workspaceId="ws" conversation={{ ...conversation, status: 'resolved' }} />)
    expect(dispatchSupportShortcut('snooze', 'c1')).toBe(false)
    unmount()
  })
})

function ShortcutHost({ onCheatsheet }: { onCheatsheet: () => void }) {
  useSupportInboxShortcuts({ enabled: true, onCheatsheet })
  return <input aria-label="Search" />
}

describe('useSupportInboxShortcuts', () => {
  it('sends key presses to the selected conversation and ignores typing', () => {
    useSupportInboxStore.getState().selectConversation('c9')
    const received: string[] = []
    const stop = onSupportShortcut((detail) => {
      received.push(`${detail.action}:${detail.conversationId}`)
      detail.handled = true
    })
    const onCheatsheet = vi.fn()
    const unmount = mount(<ShortcutHost onCheatsheet={onCheatsheet} />)

    const resolve = new KeyboardEvent('keydown', { key: 'e', bubbles: true, cancelable: true })
    document.body.dispatchEvent(resolve)
    expect(received).toEqual(['resolve:c9'])
    expect(resolve.defaultPrevented).toBe(true)

    document.querySelector('input')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'e', bubbles: true }))
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'e', metaKey: true, bubbles: true }))
    expect(received).toEqual(['resolve:c9'])

    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '?', bubbles: true }))
    expect(onCheatsheet).toHaveBeenCalledTimes(1)

    unmount()
    stop()
    useSupportInboxStore.getState().selectConversation(null)
  })
})
