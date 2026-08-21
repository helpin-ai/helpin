import { act, render, screen, fireEvent } from '@testing-library/react'
import { useSupportPresenceStore, type SupportConversation } from '@helpin-ai/support-core'
import { getAvatarColor } from '@/components/support/helpers'
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

afterEach(() => {
  act(() => useSupportPresenceStore.setState({ viewingAgents: {} }))
})

test('renders customer name, preview, and relative time', () => {
  render(<ConversationCell conversation={conversation()} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-name').textContent).toBe('Ada Lovelace')
  expect(screen.getByTestId('conversation-preview').textContent).toBe('Can you help me with my invoice?')
  expect(screen.getByTestId('conversation-time').textContent).toBeTruthy()
})

test('uses the same deterministic support badge color as the web conversation list', () => {
  render(<ConversationCell conversation={conversation({ customer_email: 'ada@example.com' })} onPress={vi.fn()} />)

  const avatar = screen.getByText('A').parentElement
  const expectedClasses = getAvatarColor('ada@example.com').split(' ')
  expect(avatar?.className).toContain(expectedClasses[0])
  expect(avatar?.className).toContain(expectedClasses[2])
  expect(avatar?.className).toContain(expectedClasses[3])
})

test('no unread count badge when the conversation is read', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 0 })} onPress={vi.fn()} />)
  expect(screen.queryByTestId('unread-count')).toBeNull()
})

test('shows the unread count badge (with the count) when there are unread messages', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 2 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('unread-count').textContent).toBe('2')
})

test('caps the unread count badge at 99+', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 150 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('unread-count').textContent).toBe('99+')
})

test('unread conversations render the preview with unread typography', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 2 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-preview').className).toContain('font-medium')
})

test('open + unread conversations get the needs-action row tint', () => {
  render(<ConversationCell conversation={conversation({ status: 'open', unread_count: 1 })} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-cell').className).toContain('bg-primary')
})

test('open + awaiting-reply conversations get the needs-action row tint even when read', () => {
  render(
    <ConversationCell
      conversation={conversation({ status: 'open', unread_count: 0, awaiting_reply: true })}
      onPress={vi.fn()}
    />,
  )
  expect(screen.getByTestId('conversation-cell').className).toContain('bg-primary')
})

test('falls back to "No messages yet" when there is no last message', () => {
  render(<ConversationCell conversation={conversation({ last_message: undefined })} onPress={vi.fn()} />)
  expect(screen.getByTestId('conversation-preview').textContent).toBe('No messages yet')
})

test('renders an internal note with a "Note:" label and the stripped body', () => {
  render(<ConversationCell conversation={conversation({ last_message: 'Note: called the customer back' })} onPress={vi.fn()} />)
  const preview = screen.getByTestId('conversation-preview')
  expect(preview.textContent).toContain('Note:')
  expect(preview.textContent).toContain('called the customer back')
})

test('shows a resolved indicator only when the conversation is resolved', () => {
  const { rerender } = render(<ConversationCell conversation={conversation({ status: 'open' })} onPress={vi.fn()} />)
  expect(screen.queryByLabelText('Resolved')).toBeNull()

  rerender(<ConversationCell conversation={conversation({ status: 'resolved' })} onPress={vi.fn()} />)
  expect(screen.getByLabelText('Resolved')).toBeDefined()
})

test('shows an AI-resolved badge when resolved by AI', () => {
  render(<ConversationCell conversation={conversation({ status: 'open', ai_state: 'resolved' })} onPress={vi.fn()} />)
  expect(screen.getByLabelText('Resolved by AI')).toBeDefined()
})

test('shows the AI handoff badge when system-tagged', () => {
  render(<ConversationCell conversation={conversation({ system_tags: ['ai_handoff'] })} onPress={vi.fn()} />)
  expect(screen.getByLabelText('AI handed off to team')).toBeDefined()
})

test('renders coloured tags with an overflow chip', () => {
  render(
    <ConversationCell
      conversation={conversation({
        tags: [
          { id: 't1', name: 'VIP', color: '#2563eb' },
          { id: 't2', name: 'Billing', color: '#16a34a' },
          { id: 't3', name: 'Urgent', color: '#dc2626' },
        ],
      })}
      onPress={vi.fn()}
    />,
  )
  expect(screen.getByText('VIP')).toBeDefined()
  expect(screen.getByText('Billing')).toBeDefined()
  expect(screen.queryByText('Urgent')).toBeNull()
  expect(screen.getByText('+1')).toBeDefined()
})

test('renders a waiting-for-human pill for queued conversations', () => {
  render(<ConversationCell conversation={conversation({ flow_state: 'queued_for_human' })} onPress={vi.fn()} />)
  expect(screen.getByText(/Waiting for human/)).toBeDefined()
})

test('shows the team-replied indicator when the last message is from an agent', () => {
  render(<ConversationCell conversation={conversation({ last_message_sender_type: 'agent' })} onPress={vi.fn()} />)
  expect(screen.getByLabelText('Team replied')).toBeDefined()
})

test('shows the teammates currently reviewing a read conversation', () => {
  act(() => useSupportPresenceStore.setState({ viewingAgents: { c1: ['self', 'user-2'] } }))
  render(
    <ConversationCell
      conversation={conversation()}
      onPress={vi.fn()}
      currentUserId="self"
      reviewerMembers={[
        { id: 'member-2', user_id: 'user-2', role: 'member', email: 'grace@example.com', display_name: 'Grace Hopper' },
      ]}
    />,
  )
  expect(screen.getByLabelText('Grace Hopper viewing')).toBeDefined()
  const reviewerAvatar = screen.getByText('G').parentElement
  expect(reviewerAvatar?.className).toContain(getAvatarColor('user-2').split(' ')[0])
})

test('fires onPress when the cell is clicked', () => {
  const onPress = vi.fn()
  render(<ConversationCell conversation={conversation()} onPress={onPress} />)
  fireEvent.click(screen.getByTestId('conversation-cell'))
  expect(onPress).toHaveBeenCalledOnce()
})

test('accessible name announces name, preview, and time, but not "unread" when read', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 0 })} onPress={vi.fn()} />)
  const cell = screen.getByTestId('conversation-cell')
  expect(cell.getAttribute('aria-label')).toBe('Ada Lovelace, Can you help me with my invoice?, ' + screen.getByTestId('conversation-time').textContent)
  expect(cell.getAttribute('aria-label')).not.toMatch(/unread/i)
})

test('accessible name appends ", unread" when the conversation is unread', () => {
  render(<ConversationCell conversation={conversation({ unread_count: 3 })} onPress={vi.fn()} />)
  const cell = screen.getByTestId('conversation-cell')
  expect(cell.getAttribute('aria-label')).toMatch(/, unread$/)
})
