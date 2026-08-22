import { render, screen } from '@testing-library/react'
import { ConversationReviewers, conversationReviewerLabel } from '../conversation-reviewers'

const members = [
  { id: 'member-1', user_id: 'user-1', role: 'member', email: 'ada@example.com', display_name: 'Ada Lovelace', avatar_url: 'https://example.com/ada.png' },
  { id: 'member-2', user_id: 'user-2', role: 'member', email: 'grace@example.com', display_name: 'Grace Hopper' },
]

test('shows other active reviewers and excludes the local user', () => {
  render(<ConversationReviewers viewerIds={['self', 'user-1']} members={members} currentUserId="self" />)
  expect(screen.getByLabelText('Ada Lovelace is also viewing')).toBeDefined()
  expect(screen.queryByText(/self/i)).toBeNull()
})

test('does not render when only the local user is viewing', () => {
  const { container } = render(<ConversationReviewers viewerIds={['self']} members={members} currentUserId="self" />)
  expect(container.innerHTML).toBe('')
})

test('summarizes multiple reviewers', () => {
  expect(conversationReviewerLabel(['Ada', 'Grace'])).toBe('Ada and Grace are also viewing')
  expect(conversationReviewerLabel(['Ada', 'Grace', 'Linus'])).toBe('Ada and 2 others are also viewing')
})
