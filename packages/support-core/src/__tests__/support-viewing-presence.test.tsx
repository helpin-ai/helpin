import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { useSupportPresenceStore } from '../support-presence-store'
import { useConversationViewingPresence } from '../use-support-realtime'

afterEach(() => {
  act(() => useSupportPresenceStore.setState({ wsConnected: false, wsSend: null }))
})

describe('useConversationViewingPresence', () => {
  test('starts, switches, and stops conversation viewing over the active socket', () => {
    const send = vi.fn()
    act(() => useSupportPresenceStore.setState({ wsConnected: true, wsSend: send }))
    const view = renderHook(
      ({ conversationId }) => useConversationViewingPresence('workspace-1', conversationId),
      { initialProps: { conversationId: 'conversation-1' } },
    )

    expect(send).toHaveBeenCalledWith('support:viewing:start', { conversation_id: 'conversation-1' })

    view.rerender({ conversationId: 'conversation-2' })
    expect(send).toHaveBeenCalledWith('support:viewing:stop', { conversation_id: 'conversation-1' })
    expect(send).toHaveBeenCalledWith('support:viewing:start', { conversation_id: 'conversation-2' })

    view.unmount()
    expect(send).toHaveBeenCalledWith('support:viewing:stop', { conversation_id: 'conversation-2' })
  })

  test('stays silent until the websocket is connected', () => {
    const send = vi.fn()
    act(() => useSupportPresenceStore.setState({ wsConnected: false, wsSend: send }))
    renderHook(() => useConversationViewingPresence('workspace-1', 'conversation-1'))
    expect(send).not.toHaveBeenCalled()
  })
})
