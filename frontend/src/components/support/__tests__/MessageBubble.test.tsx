// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from '@/stores/authStore'
import type { SupportMessage } from '@/lib/pmTypes'
import { TooltipProvider } from '@/components/ui/tooltip'
import { MessageBubble } from '../MessageBubble'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const LONG_PADDLE_URL = 'https://customer-portal.paddle.com/cpl_01jmc9m8bb4r0aqj6pwsfm3e28?action=update_subscription_payment_method&subscription_id=sub_01kjeeddmwt0qjg6snxcavej39&token=pga_eyJhbGciOiJFZERTQSIsImtpZCI6Imp3a18wMWhkazBtZDNzcHRtY3ZoYzR0dG0zZ2JoOSIsInR5cCI6IkpXVCJ9.eyJpZCI6InBnYV8wMWtwMnJtY2dnOHFtbmUyeHg4MWM5c3RldiIsInNlbGxlci1pZCI6IjIxNzkxNyIsInR5cGUiOiJzdGFuZGFyZCIsInZlcnNpb24iOiIxIiwidXNhZ2UiOiJjdXN0b21lci1wb3J0YWwtdXJsIiwic2NvcGUiOiJjdXN0b21lci5hZGp1c3RtZW50LnJlYWQgY3VzdG9tZXIuY2hlY2tvdXQuY3JlYXRlIGN1c3RvbWVyLmNoZWNrb3V0LnJlYWQgY3VzdG9tZXIuY3VzdG9tZXIucmVhZCBjdXN0b21lci5jdXN0b21lci51cGRhdGUgY3VzdG9tZXIuY3VzdG9tZXItYWRkcmVzcy5yZWFkIGN1c3RvbWVyLmN1c3RvbWVyLWFkZHJlc3MudXBkYXRlIGN1c3RvbWVyLmN1c3RvbWVyLWJ1c2luZXNzLnJlYWQgY3VzdG9tZXIuY3VzdG9tZXItYnVzaW5lc3MuY3JlYXRlIGN1c3RvbWVyLmN1c3RvbWVyLWJ1c2luZXNzLnVwZGF0ZSBjdXN0b21lci5jdXN0b21lci1wYXltZW50LW1ldGhvZC5yZWFkIGN1c3RvbWVyLmN1c3RvbWVyLXBheW1lbnQtbWV0aG9kLmRlbGV0ZSBjdXN0b21lci5pbnZvaWNlLnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLWNhbmNlbC5jcmVhdGUgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLWNvbnNlbnQtcmVxdWlyZW1lbnQtZ3JhbnQuY3JlYXRlIGN1c3RvbWVyLnN1YnNjcmlwdGlvbi1jb25zZW50LXJlcXVpcmVtZW50LnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLnVwZGF0ZSBjdXN0b21lci50cmFuc2FjdGlvbi5jcmVhdGUgY3VzdG9tZXIudHJhbnNhY3Rpb24ucmVhZCBjdXN0b21lci50cmFuc2FjdGlvbi51cGRhdGUgY3VzdG9tZXIudHJhbnNhY3Rpb24ub3JpZ2luLnJlYWQiLCJpc3MiOiJndWVzdGFjY2Vzcy1zZXJ2aWNlIiwic3ViIjoiY3RtXzAxa2plZTRxdGpkeTBuMTZ4cmZ4cXllY2NtIiwiZXhwIjoxNzc2MTQ4MzE5LCJpYXQiOjE3NzYwNjE5MTl9.u1Lm-pF347MaDE92MtneXBR6a4KxXiobrEBXjXjVm5Jk49qqGHKSYpUswzVyZy9BU0kl26tvkonj3gjMVquzAA'

describe('MessageBubble', () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: {
        id: 'viewer-1',
        email: 'viewer@example.com',
        full_name: 'Viewer',
        organization_id: 'org-1',
        created_at: '2026-04-01T00:00:00Z',
        updated_at: '2026-04-01T00:00:00Z',
      },
      loading: false,
      serverUnreachable: false,
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false })
  })

  it('renders a long billing link message with wrapping-safe anchors and link previews', () => {
    const message: SupportMessage = {
      id: 'msg-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Daniyal Ali Sajid',
      content: `Hi Patrick,\n\nUse the secure link below:\n\n${LONG_PADDLE_URL}\n\nLet us know if you run into any issues!`,
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      metadata: JSON.stringify({
        link_previews: [
          {
            url: LONG_PADDLE_URL,
            host: 'customer-portal.paddle.com',
            title: 'Customer Portal',
          },
        ],
      }),
      created_at: '2026-04-13T06:40:07.886147Z',
      updated_at: '2026-04-13T06:42:09.73606Z',
    }

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <TooltipProvider>
            <MessageBubble message={message} />
          </TooltipProvider>
        </QueryClientProvider>,
      )
    })

    expect(container.textContent).toContain('Customer Portal')

    const bubble = container.querySelector('[data-slot="support-message-bubble"]')
    expect(bubble?.className).toContain('min-w-0')

    const rawLink = Array.from(container.querySelectorAll('a')).find((anchor) => anchor.getAttribute('href') === LONG_PADDLE_URL)
    expect(rawLink).toBeTruthy()
    expect(rawLink?.className).toContain('[overflow-wrap:anywhere]')

    act(() => {
      root.unmount()
    })
    container.remove()
    queryClient.clear()
  })

  it('constrains email iframe content to the message bubble width', () => {
    const message: SupportMessage = {
      id: 'msg-email-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Customer',
      content: 'Email body',
      html_body: '<div style="white-space: nowrap">Hello from a long email body that should not push into the details sidebar.</div>',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()

    act(() => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <TooltipProvider>
            <MessageBubble message={message} />
          </TooltipProvider>
        </QueryClientProvider>,
      )
    })

    const bubble = container.querySelector('[data-slot="support-message-bubble"]')
    expect(bubble?.className).toContain('max-w-[min(85%,46rem)]')

    const iframe = container.querySelector('iframe[title="Email body"]') as HTMLIFrameElement | null
    expect(iframe).toBeTruthy()
    expect(iframe?.style.maxWidth).toBe('100%')
    expect(iframe?.style.minWidth).toBe('0px')
    expect(iframe?.style.minWidth).not.toBe('602px')
    expect(iframe?.getAttribute('srcdoc')).toContain('overflow-wrap: anywhere !important')

    act(() => {
      root.unmount()
    })
    container.remove()
    queryClient.clear()
  })
})
