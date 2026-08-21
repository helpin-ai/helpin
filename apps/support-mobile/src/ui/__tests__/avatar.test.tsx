import { render, screen } from '@testing-library/react'
import { Avatar, getInitials } from '../avatar'

test('getInitials takes the first letter of the first two words, uppercased', () => {
  expect(getInitials('Ada Lovelace')).toBe('AL')
  expect(getInitials('ada lovelace')).toBe('AL')
  expect(getInitials('Cher')).toBe('C')
  expect(getInitials('  Grace   Hopper  Jr ')).toBe('GH')
  expect(getInitials('')).toBe('')
  expect(getInitials('Ada Lovelace', 1)).toBe('A')
})

test('renders initials fallback when no src is provided', () => {
  render(<Avatar name="Ada Lovelace" />)
  expect(screen.getByText('AL')).toBeDefined()
})

test('supports the single smaller initial used by web support lists', () => {
  render(<Avatar name="Ada Lovelace" size={36} initialCount={1} />)
  const initial = screen.getByText('A')
  expect(initial).toBeDefined()
  expect(initial.getAttribute('style')).toContain('12.24px')
})
