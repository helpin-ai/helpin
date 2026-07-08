import { render, screen, fireEvent } from '@testing-library/react'
import type { SupportConversation } from '@helpin-ai/support-core'
import { ConversationCell } from '../conversation-cell'

function conversation(overrides: Partial<SupportConversation> = {}): SupportConversation {
  return {
    id: 'c1',
    workspace_id: 'w1',
    display_id: 1,
    subject: 'Hello',
    status: 'open',
    priority: 'medium',
    source: 'email',
    customer_name: 'Ada Lovelace',
    last_message: 'Can you help me with my invoice?',
    unread_count: 0,
    created_at: '2026-07-08T09:00:00.000Z',
    updated_at: '2026-07-08T09:00:00.000Z',
    ...overrides,
  }
}

test('renders customer name, preview, and relative time', () => {
  render(<ConversationCell conversation={conversation()} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-name').textContent).toBe('Ada Lovelace')
  expect(screen.getByTestId('conversation-preview').textContent).toBe('Can you help me with my invoice?')
  expect(screen.getByTestId('conversation-time').textContent).toBeTruthy()
})

test('unread dot is hidden (opacity-0) when the conversation is read', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 0 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('unread-dot').className).toContain('opacity-0')
  expect(screen.getByTestId('unread-dot').className).not.toContain('opacity-100')
})

test('unread dot is visible (opacity-100) when the conversation has unread messages', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 2 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('unread-dot').className).toContain('opacity-100')
})

test('unread conversations render the preview with unread typography', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 2 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-preview').className).toContain('font-medium')
})

test('falls back to "No messages yet" when there is no last message', () => {
  render(<ConversationCell conversation={conversation({ last_message: undefined })} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-preview').textContent).toBe('No messages yet')
})

test('shows a status badge only when the conversation is not open', () => {
  const { rerender } = render(<ConversationCell conversation={conversation({ status: 'open' })} onPress={vi.fn()} />)
  expect(screen.queryByText('Resolved')).toBeNull()

  rerender(<ConversationCell conversation={conversation({ status: 'resolved' })} onPress={vi.fn()} />)
  expect(screen.getByText('Resolved')).toBeDefined()
})

test('fires onPress when the cell is clicked', () => {
  const onPress = vi.fn()
  render(<ConversationCell conversation={conversation()} onPress={onPress} />)
  fireEvent.click(screen.getByTestId('conversation-cell'))
  expect(onPress).toHaveBeenCalledOnce()
})
