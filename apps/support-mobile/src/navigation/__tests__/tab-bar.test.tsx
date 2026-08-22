import { render, screen, fireEvent } from '@testing-library/react'
import { TabBar, formatBadgeCount } from '../tab-bar'

test.each([
  [0, null],
  [5, '5'],
  [120, '99+'],
] as const)('formatBadgeCount(%i) -> %s', (input, expected) => {
  expect(formatBadgeCount(input)).toBe(expected)
})

test('renders all four primary destinations', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} />)
  expect(screen.getByRole('button', { name: /Inbox/ })).toBeDefined()
  expect(screen.getByRole('button', { name: /Mine/ })).toBeDefined()
  expect(screen.getByRole('button', { name: /Search/ })).toBeDefined()
  expect(screen.getByRole('button', { name: /Settings/ })).toBeDefined()
})

test('marks the active tab via aria-pressed', () => {
  render(<TabBar activeTab="settings" onNavigate={vi.fn()} />)
  expect(screen.getByRole('button', { name: /Inbox/ }).getAttribute('aria-pressed')).toBe('false')
  expect(screen.getByRole('button', { name: /Settings/ }).getAttribute('aria-pressed')).toBe('true')
})

test('fires onNavigate with the tapped tab key', () => {
  const onNavigate = vi.fn()
  render(<TabBar activeTab="inbox" onNavigate={onNavigate} />)
  fireEvent.click(screen.getByRole('button', { name: /Mine/ }))
  expect(onNavigate).toHaveBeenCalledWith('mine')
})

test('renders independent unread badges for Inbox and Mine', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} badges={{ inbox: 5, mine: 2 }} />)
  expect(screen.getByTestId('tab-badge-inbox').textContent).toBe('5')
  expect(screen.getByTestId('tab-badge-mine').textContent).toBe('2')
})

test('caps badges at 99+', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} badges={{ inbox: 150 }} />)
  expect(screen.getByTestId('tab-badge-inbox').textContent).toBe('99+')
})

test('hides badge elements when counts are zero or absent', () => {
  render(<TabBar activeTab="inbox" onNavigate={vi.fn()} badges={{ inbox: 0 }} />)
  expect(screen.queryByTestId('tab-badge-inbox')).toBeNull()
  expect(screen.queryByTestId('tab-badge-mine')).toBeNull()
})
