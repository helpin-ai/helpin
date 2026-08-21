import { render, screen, fireEvent } from '@testing-library/react'
import { SegmentedControl } from '../segmented-control'

test('renders each segment label', () => {
  render(
    <SegmentedControl
      segments={[
        { value: 'mine', label: 'Mine' },
        { value: 'all', label: 'All' },
      ]}
      value="mine"
      onChange={vi.fn()}
    />,
  )
  expect(screen.getByText('Mine')).toBeDefined()
  expect(screen.getByText('All')).toBeDefined()
})

test('fires onChange with tapped segment value', () => {
  const onChange = vi.fn()
  render(
    <SegmentedControl
      segments={[
        { value: 'mine', label: 'Mine' },
        { value: 'all', label: 'All' },
      ]}
      value="mine"
      onChange={onChange}
    />,
  )
  fireEvent.click(screen.getByRole('button', { name: /All/ }))
  expect(onChange).toHaveBeenCalledWith('all')
})

test('marks the selected segment via aria-pressed', () => {
  render(
    <SegmentedControl
      segments={[
        { value: 'mine', label: 'Mine' },
        { value: 'all', label: 'All' },
      ]}
      value="mine"
      onChange={vi.fn()}
    />,
  )
  expect(screen.getByRole('button', { name: /Mine/ }).getAttribute('aria-pressed')).toBe('true')
  expect(screen.getByRole('button', { name: /All/ }).getAttribute('aria-pressed')).toBe('false')
})
