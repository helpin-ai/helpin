import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import type { SupportMessage } from '@helpin-ai/support-core'
import { MessageBubble } from '../message-bubble'

function message(overrides: Partial<SupportMessage> = {}): SupportMessage {
  const now = new Date().toISOString()
  return {
    id: 'message-1',
    workspace_id: 'workspace-1',
    conversation_id: 'conversation-1',
    sender_type: 'customer',
    sender_display_name: 'Emma Wilson',
    content: 'Can you help with this charge?',
    is_internal: false,
    created_at: now,
    updated_at: now,
    ...overrides,
  }
}

test('shows a persistent clock time below a customer message card', () => {
  render(<MessageBubble message={message({ created_at: '2026-08-20T14:30:00Z' })} align="left" />)

  expect(screen.getByText('Can you help with this charge?')).toBeDefined()
  const bubble = screen.getByTestId('message-bubble')
  const meta = screen.getByTestId('message-meta')
  expect(meta.textContent).toMatch(/\d{1,2}:30/)
  expect(bubble.contains(meta)).toBe(false)
  expect(bubble.className).toContain('bg-muted')
  expect(bubble.className).toContain('border-border/40')
})

test('uses the web blue tint for outbound replies and keeps actions beside the time', () => {
  const onMessageActions = vi.fn()
  render(
    <MessageBubble
      message={message({ sender_type: 'user' })}
      align="right"
      onMessageActions={onMessageActions}
    />,
  )

  const bubble = screen.getByTestId('message-bubble')
  const meta = screen.getByTestId('message-meta')
  const actions = screen.getByRole('button', { name: 'Message actions' })
  expect(bubble.className).toContain('bg-blue-50')
  expect(bubble.className).toContain('dark:bg-blue-950/40')
  expect(meta.contains(actions)).toBe(true)
  fireEvent.click(actions)
  expect(onMessageActions).toHaveBeenCalledOnce()
})

test('gives internal notes a distinct sender label and timestamp', () => {
  render(
    <MessageBubble
      message={message({
        sender_type: 'user',
        sender_display_name: 'Marcus Bell',
        content: 'Confirmed the duplicate charge.',
        is_internal: true,
      })}
      align="left"
    />,
  )

  expect(screen.getByText('Note · Marcus Bell')).toBeDefined()
  expect(screen.getByTestId('message-meta')).toBeDefined()
  expect(screen.getByTestId('message-bubble').className).toContain('bg-amber-50')
  expect(screen.getByTestId('message-bubble').className).toContain('border-r-amber-400')
})

test('shows email and read state below an outbound bubble', () => {
  render(
    <MessageBubble
      message={message({ sender_type: 'user', via_channel: 'email' })}
      align="right"
      receiptStatus="read_email"
    />,
  )

  const bubble = screen.getByTestId('message-bubble')
  const meta = screen.getByTestId('message-meta')
  expect(meta.textContent).toContain('Read via email')
  expect(bubble.contains(meta)).toBe(false)
})

test('previews image attachments in-app instead of opening their raw URL', () => {
  const open = vi.spyOn(window, 'open').mockImplementation(() => null)
  render(
    <MessageBubble
      message={message({
        attachments: [
          {
            id: 'attachment-1',
            file_key: 'support/photo.png',
            file_name: 'photo.png',
            file_type: 'image/png',
            file_size: 1024,
            url: 'https://cdn.example.com/photo.png',
          },
        ],
      })}
      align="left"
    />,
  )

  fireEvent.click(screen.getByRole('button', { name: 'Preview photo.png' }))

  expect(screen.getByTestId('image-viewer-stage')).toBeDefined()
  expect(screen.getByRole('button', { name: 'Close image viewer' })).toBeDefined()
  expect(open).not.toHaveBeenCalled()
  open.mockRestore()
})

test('shows teammate joins with the actor avatar, a surfaced background, and time', () => {
  render(
    <MessageBubble
      message={message({
        sender_type: 'user',
        sender_user_id: 'member-1',
        sender_display_name: 'Emma Wilson',
        message_type: 'system',
        system_event_type: 'teammate_joined',
        content: 'Emma Wilson joined the conversation.',
        created_at: '2026-08-20T14:30:00Z',
      })}
      align="left"
    />,
  )

  const surface = screen.getByTestId('system-event-surface')
  expect(surface.className).toContain('bg-muted/60')
  expect(screen.getByText('EW')).toBeDefined()
  expect(screen.getByText('Emma Wilson').tagName).toBe('STRONG')
  expect(screen.getByTestId('system-event-time').textContent).toMatch(/\d{1,2}:30/)
})

test('uses distinct resolved, closed, and escalation treatments', () => {
  const view = render(
    <MessageBubble
      message={message({
        sender_type: 'user',
        sender_display_name: 'Emma Wilson',
        message_type: 'system',
        system_event_type: 'resolved',
        content: 'Emma Wilson resolved this conversation.',
      })}
      align="left"
    />,
  )

  expect(screen.getByTestId('system-event-surface').className).toContain('bg-emerald-50')
  expect(screen.getByLabelText('Resolved')).toBeDefined()

  view.rerender(
    <MessageBubble
      message={message({
        message_type: 'system',
        system_event_type: 'closed',
        content: 'Emma Wilson closed this conversation.',
      })}
      align="left"
    />,
  )
  expect(screen.getByTestId('system-event-surface').className).toContain('bg-slate-700')
  expect(screen.getByLabelText('Closed')).toBeDefined()

  view.rerender(
    <MessageBubble
      message={message({
        sender_type: 'ai',
        message_type: 'system',
        system_event_type: 'ai_escalated',
        content: 'Let me find a teammate.',
      })}
      align="left"
    />,
  )
  expect(screen.getByTestId('system-event-surface').className).toContain('bg-amber-50')
  expect(screen.getByText('AI escalated to a human')).toBeDefined()
  expect(screen.getByLabelText('AI')).toBeDefined()
})

test('renders every canonical audit event with the polished system surface', () => {
  const eventTypes = [
    'teammate_joined',
    'assigned',
    'unassigned',
    'took',
    'agent_assigned',
    'mailbox_moved',
    'triage_routed',
    'triage_dismissed',
    'ai_escalated',
    'customer_requested_human',
    'resolved',
    'reopened',
    'closed',
    'email_recipients_updated',
    'tag_added',
    'tag_removed',
    'task_created',
  ]

  for (const eventType of eventTypes) {
    const view = render(
      <MessageBubble
        message={message({
          message_type: 'system',
          system_event_type: eventType,
          content: `Emma Wilson recorded ${eventType}.`,
        })}
        align="left"
      />,
    )
    expect(view.container.querySelector(`[data-system-event-type="${eventType}"]`)).not.toBeNull()
    expect(view.getByTestId('system-event-surface')).toBeDefined()
    view.unmount()
  }
})
