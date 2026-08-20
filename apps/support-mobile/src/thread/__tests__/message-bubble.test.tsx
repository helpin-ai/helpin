import { render, screen } from '@testing-library/react'
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

test('shows a persistent timestamp inside a customer message card', () => {
  render(<MessageBubble message={message()} align="left" />)

  expect(screen.getByText('Can you help with this charge?')).toBeDefined()
  expect(screen.getByText('Now')).toBeDefined()
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
  expect(screen.getByText('Now')).toBeDefined()
})
