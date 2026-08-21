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
