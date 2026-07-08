import { render, screen, fireEvent } from '@testing-library/react'
import { TabBar, formatBadgeCount } from '../tab-bar'

test.each([
  [0, null],
  [5, '5'],
  [120, '99+'],
] as const)('formatBadgeCount(%i) -> %s', (input, expected) => {
  expect(formatBadgeCount(input)).toBe(expected)
})

test('renders both tab items', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} />)
  expect(screen.getByRole('button', { name: /Inbox/ })).toBeDefined()
  expect(screen.getByRole('button', { name: /You/ })).toBeDefined()
})

test('marks the active tab via aria-pressed', () => {
  render(<TabBar activeTab="you" onNavigate={vi.fn()} />)
  expect(screen.getByRole('button', { name: /Inbox/ }).getAttribute('aria-pressed')).toBe('false')
  expect(screen.getByRole('button', { name: /You/ }).getAttribute('aria-pressed')).toBe('true')
})

test('fires onNavigate with the tapped tab key', () => {
  const onNavigate = vi.fn()
  render(<TabBar activeTab="inbox" onNavigate={onNavigate} />)
  fireEvent.click(screen.getByRole('button', { name: /You/ }))
  expect(onNavigate).toHaveBeenCalledWith('you')
})

test('renders the unread badge on the Inbox tab when a count is provided', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} unreadCount={5} />)
  expect(screen.getByTestId('tab-badge').textContent).toBe('5')
})

test('caps the badge at 99+', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} unreadCount={150} />)
  expect(screen.getByTestId('tab-badge').textContent).toBe('99+')
})

test('hides the badge element when the count is zero or absent', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} unreadCount={0} />)
  expect(screen.queryByTestId('tab-badge')).toBeNull()
})
