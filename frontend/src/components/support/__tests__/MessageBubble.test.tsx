// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from '@/stores/authStore'
import type { SupportMessage } from '@/lib/pmTypes'
import { TooltipProvider } from '@/components/ui/tooltip'
import { MessageBubble, sanitizeSupportShortcutSeed } from '../MessageBubble'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const LONG_PADDLE_URL = 'https://customer-portal.paddle.com/cpl_01jmc9m8bb4r0aqj6pwsfm3e28?action=update_subscription_payment_method&subscription_id=sub_01kjeeddmwt0qjg6snxcavej39&token=pga_eyJhbGciOiJFZERTQSIsImtpZCI6Imp3a18wMWhkazBtZDNzcHRtY3ZoYzR0dG0zZ2JoOSIsInR5cCI6IkpXVCJ9.eyJpZCI6InBnYV8wMWtwMnJtY2dnOHFtbmUyeHg4MWM5c3RldiIsInNlbGxlci1pZCI6IjIxNzkxNyIsInR5cGUiOiJzdGFuZGFyZCIsInZlcnNpb24iOiIxIiwidXNhZ2UiOiJjdXN0b21lci1wb3J0YWwtdXJsIiwic2NvcGUiOiJjdXN0b21lci5hZGp1c3RtZW50LnJlYWQgY3VzdG9tZXIuY2hlY2tvdXQuY3JlYXRlIGN1c3RvbWVyLmNoZWNrb3V0LnJlYWQgY3VzdG9tZXIuY3VzdG9tZXIucmVhZCBjdXN0b21lci5jdXN0b21lci51cGRhdGUgY3VzdG9tZXIuY3VzdG9tZXItYWRkcmVzcy5yZWFkIGN1c3RvbWVyLmN1c3RvbWVyLWFkZHJlc3MudXBkYXRlIGN1c3RvbWVyLmN1c3RvbWVyLWJ1c2luZXNzLnJlYWQgY3VzdG9tZXIuY3VzdG9tZXItYnVzaW5lc3MuY3JlYXRlIGN1c3RvbWVyLmN1c3RvbWVyLWJ1c2luZXNzLnVwZGF0ZSBjdXN0b21lci5jdXN0b21lci1wYXltZW50LW1ldGhvZC5yZWFkIGN1c3RvbWVyLmN1c3RvbWVyLXBheW1lbnQtbWV0aG9kLmRlbGV0ZSBjdXN0b21lci5pbnZvaWNlLnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLWNhbmNlbC5jcmVhdGUgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLWNvbnNlbnQtcmVxdWlyZW1lbnQtZ3JhbnQuY3JlYXRlIGN1c3RvbWVyLnN1YnNjcmlwdGlvbi1jb25zZW50LXJlcXVpcmVtZW50LnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLnJlYWQgY3VzdG9tZXIuc3Vic2NyaXB0aW9uLnVwZGF0ZSBjdXN0b21lci50cmFuc2FjdGlvbi5jcmVhdGUgY3VzdG9tZXIudHJhbnNhY3Rpb24ucmVhZCBjdXN0b21lci50cmFuc2FjdGlvbi51cGRhdGUgY3VzdG9tZXIudHJhbnNhY3Rpb24ub3JpZ2luLnJlYWQiLCJpc3MiOiJndWVzdGFjY2Vzcy1zZXJ2aWNlIiwic3ViIjoiY3RtXzAxa2plZTRxdGpkeTBuMTZ4cmZ4cXllY2NtIiwiZXhwIjoxNzc2MTQ4MzE5LCJpYXQiOjE3NzYwNjE5MTl9.u1Lm-pF347MaDE92MtneXBR6a4KxXiobrEBXjXjVm5Jk49qqGHKSYpUswzVyZy9BU0kl26tvkonj3gjMVquzAA'

function findButtonByText(container: HTMLElement, text: string) {
  return Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes(text)) ?? null
}

function renderBubble(message: SupportMessage, receiptStatus?: 'delivered' | 'sent_email' | 'delivered_email' | 'read' | 'read_email' | null) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  const queryClient = new QueryClient()

  act(() => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <MessageBubble message={message} receiptStatus={receiptStatus} />
        </TooltipProvider>
      </QueryClientProvider>,
    )
  })

  return {
    container,
    cleanup: () => {
      act(() => {
        root.unmount()
      })
      container.remove()
      queryClient.clear()
    },
  }
}

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

  it('removes markdown hard-break escapes when seeding shortcut content', () => {
    expect(sanitizeSupportShortcutSeed('Hi Caleb,\\\n\\\nThank you for reaching out.')).toBe(
      'Hi Caleb,\n\nThank you for reaching out.',
    )
  })

  afterEach(() => {
    document.body.innerHTML = ''
    useAuthStore.setState({ user: null, loading: false, serverUnreachable: false })
  })

  it('uses source-specific icon avatars for automated routing events', () => {
    const ruleMessage: SupportMessage = {
      id: 'msg-rule-route',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'ai',
      sender_display_name: 'Routing',
      content: "Routing rule moved to inbox 'Billing'.",
      message_type: 'system',
      system_event_type: 'triage_routed',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }
    const aiMessage: SupportMessage = {
      ...ruleMessage,
      id: 'msg-ai-route',
      sender_display_name: 'Helpin AI',
      content: "AI routing moved to inbox 'Billing'.",
    }

    const renderedRule = renderBubble(ruleMessage)
    expect(renderedRule.container.querySelector('[aria-label="Routing rule"]')).toBeTruthy()
    expect(renderedRule.container.textContent).not.toContain('RRouting rule')
    renderedRule.cleanup()

    const renderedAI = renderBubble(aiMessage)
    expect(renderedAI.container.querySelector('[aria-label="AI routing"]')).toBeTruthy()
    expect(renderedAI.container.textContent).not.toContain('HAI routing')
    renderedAI.cleanup()
  })

  it('renders resolved system messages with a green check treatment', () => {
    const message: SupportMessage = {
      id: 'msg-resolved',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Sarah Khan',
      content: 'Sarah resolved this conversation.',
      message_type: 'system',
      system_event_type: 'resolved',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }

    const rendered = renderBubble(message)
    const resolvedIcon = rendered.container.querySelector('[aria-label="Resolved"]')
    expect(resolvedIcon).toBeTruthy()
    expect(resolvedIcon?.className).toContain('text-emerald-600')
    const actorAvatar = rendered.container.querySelector('img[alt="Sarah Khan"], [aria-label="Sarah Khan"]')
    expect(actorAvatar).toBeTruthy()
    expect(resolvedIcon?.compareDocumentPosition(actorAvatar as Node) ?? 0).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
    expect(rendered.container.querySelector('.px-3.py-1.text-xs')).toBeTruthy()
    expect(rendered.container.querySelector('.text-sm')?.textContent).not.toBe('Sarah resolved this conversation.')
    expect(rendered.container.textContent).toContain('Sarah resolved this conversation.')
    rendered.cleanup()
  })

  it('renders reopened system messages like neutral routing timeline events', () => {
    const message: SupportMessage = {
      id: 'msg-reopened',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Sarah Khan',
      content: 'Sarah reopened this conversation.',
      message_type: 'system',
      system_event_type: 'reopened',
      is_internal: true,
      created_at: '2026-06-12T09:00:00.000Z',
      updated_at: '2026-06-12T09:00:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.querySelector('.px-3.py-1.text-xs')).toBeTruthy()
    expect(rendered.container.querySelector('[aria-label="Reopened"]')).toBeNull()
    expect(rendered.container.textContent).toContain('Sarah reopened this conversation.')
    rendered.cleanup()
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
    expect(bubble?.className).toContain('w-[min(92%,64rem)]')
    expect(bubble?.className).toContain('max-w-[calc(100%-2.25rem)]')

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

  it('renders compact forwarded attribution in the email badge', () => {
    const message: SupportMessage = {
      id: 'msg-forwarded-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Jane Customer',
      content: 'I need help with my invoice.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      metadata: JSON.stringify({
        forwarded_by_email: 'founder@company.com',
        forwarded_by_name: 'Founder',
        original_sender_email: 'jane@customer.example',
        original_sender_name: 'Jane Customer',
        sender_attribution_confidence_level: 'high',
      }),
      created_at: '2026-06-02T10:14:00.000Z',
      updated_at: '2026-06-02T10:14:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Forwarded by Founder')
    expect(rendered.container.textContent).not.toContain('View details')
    expect(rendered.container.textContent).not.toContain('Received via email')
    expect(rendered.container.textContent).not.toContain('originally from')
    expect(rendered.container.textContent).not.toContain('jane@customer.example')
    expect(findButtonByText(rendered.container, 'Forwarded by Founder')).toBeTruthy()
    rendered.cleanup()
  })

  it('shows the forwarded customer body when the email has no note above the forwarded header', () => {
    const message: SupportMessage = {
      id: 'msg-forwarded-empty-note-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'customer',
      sender_display_name: 'Jason Smith',
      content: `---------- Forwarded message ---------
From: Jason Smith <jason@the-web-dev.com>
Date: Sun, May 31, 2026 at 2:38 PM
Subject: Data Export
To: Waqar from Usermaven <waqar@usermaven.com>

Can I export my data?`,
      html_body: '<div data-helpin-quote="true">Can I export my data?</div>',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      metadata: JSON.stringify({
        forwarded_by_email: 'waqar@usermaven.com',
        forwarded_by_name: 'Waqar Azeem',
        original_sender_email: 'jason@the-web-dev.com',
        original_sender_name: 'Jason Smith',
        sender_attribution_confidence_level: 'high',
      }),
      created_at: '2026-06-02T14:40:00.000Z',
      updated_at: '2026-06-02T14:40:00.000Z',
    }

    const rendered = renderBubble(message)
    expect(rendered.container.textContent).toContain('Can I export my data?')
    expect(rendered.container.textContent).not.toContain('Forwarded message')
    expect(rendered.container.textContent).not.toContain('From: Jason Smith')
    expect(rendered.container.textContent).toContain('Forwarded by Waqar Azeem')
    rendered.cleanup()
  })

  it('renders outbound fallback email delivery receipt states', () => {
    const message: SupportMessage = {
      id: 'msg-email-status-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Agent',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'email',
      email_notified_at: '2026-04-24T12:20:00.000Z',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const sent = renderBubble(message, 'sent_email')
    expect(sent.container.textContent).toContain('Sent via email')
    expect(sent.container.textContent).not.toContain('View details')
    expect(sent.container.textContent?.match(/Sent via email/g)).toHaveLength(1)
    expect(findButtonByText(sent.container, 'Sent via email')).toBeTruthy()
    sent.cleanup()

    const delivered = renderBubble({ ...message, id: 'msg-email-status-delivered', email_delivery_status: 'delivered' }, 'delivered_email')
    expect(delivered.container.textContent).toContain('Delivered via email')
    delivered.cleanup()

    const read = renderBubble({ ...message, id: 'msg-email-status-2', email_read_at: '2026-04-24T12:22:00.000Z' }, 'read_email')
    expect(read.container.textContent).toContain('Read via email')
    read.cleanup()
  })

  it('does not keep showing delivered email from an expired undo window', () => {
    const message: SupportMessage = {
      id: 'msg-expired-cancellable-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'viewer-1',
      sender_display_name: 'Viewer',
      content: 'Earlier email reply.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      cancellable_until: '2026-04-24T12:20:00.000Z',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const expired = renderBubble(message)
    expect(expired.container.textContent).not.toContain('Delivered to email')
    expect(expired.container.textContent).not.toContain('Undo')
    expired.cleanup()

    const active = renderBubble({ ...message, id: 'msg-active-cancellable-1', cancellable_until: '2099-04-24T12:20:00.000Z' })
    expect(active.container.textContent).toContain('Queued for email')
    expect(active.container.textContent).toContain('Undo')
    expect(active.container.textContent).not.toContain('Delivered to email')
    active.cleanup()
  })

  it('renders internal note image and file attachments with image preview', () => {
    const message: SupportMessage = {
      id: 'msg-note-attachments-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'viewer-1',
      sender_display_name: 'Viewer',
      content: '',
      message_type: 'note',
      is_internal: true,
      via_channel: 'widget',
      attachments: [
        {
          id: 'att-image-1',
          file_key: 'support/att-image-1',
          file_name: 'screenshot.png',
          file_type: 'image/png',
          file_size: 2048,
          url: 'https://cdn.example.com/screenshot.png',
        },
        {
          id: 'att-file-1',
          file_key: 'support/att-file-1',
          file_name: 'diagnostics.pdf',
          file_type: 'application/pdf',
          file_size: 4096,
          url: 'https://cdn.example.com/diagnostics.pdf',
        },
      ],
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const { container, cleanup } = renderBubble(message)

    const noteCard = container.querySelector('.border-r-amber-400')
    expect(noteCard).toBeTruthy()

    const fileLink = container.querySelector('a[href="https://cdn.example.com/diagnostics.pdf"]')
    expect(fileLink?.textContent).toContain('diagnostics.pdf')
    expect(fileLink?.className).toContain('border-amber-200')

    const image = container.querySelector('img[alt="screenshot.png"]') as HTMLImageElement | null
    expect(image).toBeTruthy()
    expect(image?.getAttribute('src')).toBe('https://cdn.example.com/screenshot.png')
    expect(image?.className).toContain('max-h-60')

    act(() => {
      image?.closest('button')?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    const preview = document.body.querySelector('img[alt="Preview"]') as HTMLImageElement | null
    expect(preview).toBeTruthy()
    expect(preview?.getAttribute('src')).toBe('https://cdn.example.com/screenshot.png')

    cleanup()
  })

  it('constrains markdown images in internal notes', () => {
    const message: SupportMessage = {
      id: 'msg-note-markdown-image-1',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'viewer-1',
      sender_display_name: 'Viewer',
      content: 'Here is the screenshot:\n\n![Inline screenshot](https://cdn.example.com/inline.png)',
      message_type: 'note',
      is_internal: true,
      via_channel: 'widget',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:18:09.000Z',
    }

    const { container, cleanup } = renderBubble(message)

    const image = container.querySelector('img[alt="Inline screenshot"]') as HTMLImageElement | null
    expect(image).toBeTruthy()
    expect(image?.getAttribute('loading')).toBe('lazy')
    expect(image?.className).toContain('max-h-60')
    expect(image?.className).toContain('max-w-full')

    cleanup()
  })

  it('renders outbound fallback email failure states ahead of read receipts', () => {
    const message: SupportMessage = {
      id: 'msg-email-status-3',
      workspace_id: 'ws-1',
      conversation_id: 'conv-1',
      sender_type: 'user',
      sender_user_id: 'agent-1',
      sender_display_name: 'Agent',
      content: 'Following up here.',
      message_type: 'reply',
      is_internal: false,
      via_channel: 'widget',
      email_notified_at: '2026-04-24T12:20:00.000Z',
      created_at: '2026-04-24T12:18:09.000Z',
      updated_at: '2026-04-24T12:20:00.000Z',
    }

    const bounced = renderBubble({
      ...message,
      email_delivery_status: 'bounced',
      email_delivery_error: 'Mailbox unavailable',
    }, 'read_email')
    expect(bounced.container.textContent).toContain('Delivery failed · Mailbox unavailable')
    expect(bounced.container.textContent).not.toContain('Read via email')
    bounced.cleanup()

    const spam = renderBubble({
      ...message,
      id: 'msg-email-status-4',
      email_delivery_status: 'spam_complaint',
      email_delivery_error: 'Marked by recipient',
    }, 'delivered_email')
    expect(spam.container.textContent).toContain('Marked as spam · Marked by recipient')
    expect(spam.container.textContent).not.toContain('Delivered via email')
    spam.cleanup()
  })
})
