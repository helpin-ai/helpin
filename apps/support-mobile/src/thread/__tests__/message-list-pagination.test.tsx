import { fireEvent, render } from '@testing-library/react'
import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest'

import type { SupportMessage } from '@helpin-ai/support-core'
import { getAvatarColor } from '@/components/support/helpers'
import { MessageList } from '../message-list'
import { groupMessages } from '../thread-helpers'

const originalElementScrollTo = HTMLElement.prototype.scrollTo

beforeAll(() => {
  HTMLElement.prototype.scrollTo = function scrollTo(options?: ScrollToOptions | number, y?: number) {
    if (typeof options === 'number') {
      this.scrollTop = y ?? 0
      return
    }
    this.scrollTop = options?.top ?? this.scrollTop
  }
})

afterAll(() => {
  HTMLElement.prototype.scrollTo = originalElementScrollTo
})

function message(id: string, minute: string): SupportMessage {
  return {
    id,
    workspace_id: 'ws-1',
    conversation_id: 'conv-1',
    sender_type: 'customer',
    content: id,
    is_internal: false,
    created_at: `2026-08-18T10:${minute}:00Z`,
    updated_at: `2026-08-18T10:${minute}:00Z`,
  }
}

describe('MessageList history pagination', () => {
  it('renders conversation context inside the message scroller before history', () => {
    const view = render(
      <MessageList
        header={<button type="button">Refund request details</button>}
        items={groupMessages([message('msg-03', '03')])}
      />,
    )

    const scroller = view.container.querySelector('.overflow-y-auto') as HTMLDivElement
    const header = view.getByRole('button', { name: 'Refund request details' })
    const messageText = view.getByText('msg-03')

    expect(scroller.contains(header)).toBe(true)
    expect(header.compareDocumentPosition(messageText) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('loads near the top and preserves the visible anchor when history joins the same cluster', () => {
    const loadEarlier = vi.fn()
    const onPillChange = vi.fn()
    const newestMessages = [message('msg-03', '03'), message('msg-04', '04')]

    const view = render(
      <MessageList
        items={groupMessages(newestMessages)}
        hasEarlier={false}
        onLoadEarlier={loadEarlier}
        oldestMessageId="msg-03"
        newestMessageId="msg-04"
        onShowNewMessagePillChange={onPillChange}
      />,
    )

    const scroller = view.container.querySelector('.overflow-y-auto') as HTMLDivElement
    let scrollHeight = 1_000
    Object.defineProperty(scroller, 'scrollHeight', {
      configurable: true,
      get: () => scrollHeight,
    })
    Object.defineProperty(scroller, 'clientHeight', {
      configurable: true,
      get: () => 500,
    })
    scroller.scrollTop = 50

    view.rerender(
      <MessageList
        items={groupMessages(newestMessages)}
        hasEarlier
        onLoadEarlier={loadEarlier}
        oldestMessageId="msg-03"
        newestMessageId="msg-04"
        onShowNewMessagePillChange={onPillChange}
      />,
    )
    fireEvent.scroll(scroller)
    expect(loadEarlier).toHaveBeenCalledTimes(1)

    view.rerender(
      <MessageList
        items={groupMessages(newestMessages)}
        hasEarlier
        loadingEarlier
        onLoadEarlier={loadEarlier}
        oldestMessageId="msg-03"
        newestMessageId="msg-04"
        onShowNewMessagePillChange={onPillChange}
      />,
    )

    scrollHeight = 1_400
    const allMessages = [
      message('msg-01', '01'),
      message('msg-02', '02'),
      ...newestMessages,
    ]
    // groupMessages still produces one day item + one cluster, proving that
    // anchor detection cannot rely on the rendered item count increasing.
    expect(groupMessages(allMessages)).toHaveLength(groupMessages(newestMessages).length)

    view.rerender(
      <MessageList
        items={groupMessages(allMessages)}
        hasEarlier={false}
        loadingEarlier={false}
        onLoadEarlier={loadEarlier}
        oldestMessageId="msg-01"
        newestMessageId="msg-04"
        onShowNewMessagePillChange={onPillChange}
      />,
    )

    expect(scroller.scrollTop).toBe(450)
    expect(onPillChange).not.toHaveBeenCalledWith(true)
  })

  it('detects a new reply even when it joins the existing visual cluster', () => {
    const onPillChange = vi.fn()
    const initialMessages = [message('msg-03', '03'), message('msg-04', '04')]
    const view = render(
      <MessageList
        items={groupMessages(initialMessages)}
        messageCount={initialMessages.length}
        oldestMessageId="msg-03"
        newestMessageId="msg-04"
        onShowNewMessagePillChange={onPillChange}
      />,
    )

    const scroller = view.container.querySelector('.overflow-y-auto') as HTMLDivElement
    Object.defineProperty(scroller, 'scrollHeight', { configurable: true, value: 1_000 })
    Object.defineProperty(scroller, 'clientHeight', { configurable: true, value: 500 })
    scroller.scrollTop = 0
    fireEvent.scroll(scroller)

    const appended = [...initialMessages, message('msg-05', '05')]
    expect(groupMessages(appended)).toHaveLength(groupMessages(initialMessages).length)
    view.rerender(
      <MessageList
        items={groupMessages(appended)}
        messageCount={appended.length}
        oldestMessageId="msg-03"
        newestMessageId="msg-05"
        onShowNewMessagePillChange={onPillChange}
      />,
    )

    expect(onPillChange).toHaveBeenCalledWith(true)
  })

  it('shows a retry control after an earlier-page request fails', () => {
    const loadEarlier = vi.fn()
    const view = render(
      <MessageList
        items={groupMessages([message('msg-03', '03')])}
        hasEarlier
        loadEarlierError
        onLoadEarlier={loadEarlier}
        oldestMessageId="msg-03"
        newestMessageId="msg-03"
      />,
    )

    fireEvent.click(view.getByRole('button', { name: 'Retry earlier messages' }))
    expect(loadEarlier).toHaveBeenCalledTimes(1)
  })

  it('opens actions for a non-system message from its compact trigger', () => {
    const onMessageActions = vi.fn()
    const item = message('msg-03', '03')
    const view = render(
      <MessageList
        items={groupMessages([item])}
        onMessageActions={onMessageActions}
      />,
    )

    fireEvent.click(view.getByRole('button', { name: 'Message actions' }))
    expect(onMessageActions).toHaveBeenCalledWith(item)
  })

  it('uses the deterministic web avatar color for message clusters', () => {
    const item: SupportMessage = {
      ...message('msg-03', '03'),
      sender_type: 'user',
      sender_user_id: 'member-42',
      sender_display_name: 'Azhar',
    }
    const view = render(<MessageList items={groupMessages([item])} />)

    const avatar = view.getByText('A').parentElement
    expect(avatar?.className).toContain(getAvatarColor('member-42').split(' ')[0])
  })

  it('adds breathing room below a system event only when a message follows', () => {
    const joined: SupportMessage = {
      ...message('system-joined', '01'),
      sender_type: 'user',
      message_type: 'system',
      system_event_type: 'teammate_joined',
      content: 'Azhar joined the conversation.',
    }
    const reply = message('msg-02', '02')
    const view = render(<MessageList items={groupMessages([joined, reply])} />)

    const eventText = view.getByText('joined the conversation.')
    expect(eventText.closest('.mb-3')).not.toBeNull()

    view.rerender(<MessageList items={groupMessages([joined])} />)
    expect(view.getByText('joined the conversation.').closest('.mb-3')).toBeNull()
  })
})
