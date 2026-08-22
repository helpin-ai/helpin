import { render, screen, fireEvent } from '@testing-library/react'
import { TextField } from '../text-field'

test('renders the label and current value', () => {
  render(<TextField label="Email" value="a@example.com" onChange={vi.fn()} />)
  expect(screen.getByText('Email')).toBeDefined()
  expect(screen.getByDisplayValue('a@example.com')).toBeDefined()
})

test('fires onChange with the new value as the user types', () => {
  const onChange = vi.fn()
  render(<TextField label="Email" value="" onChange={onChange} />)
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: 'x' } })
  expect(onChange).toHaveBeenCalledWith('x')
})

test('fires onBlur when focus leaves the input', () => {
  const onBlur = vi.fn()
  render(<TextField label="Email" value="" onChange={vi.fn()} onBlur={onBlur} />)
  const input = screen.getByLabelText('Email')
  fireEvent.focus(input)
  fireEvent.blur(input)
  expect(onBlur).toHaveBeenCalledTimes(1)
})

test('shows the error message and marks the input invalid', () => {
  render(<TextField label="Email" value="" onChange={vi.fn()} error="Email is required" />)
  expect(screen.getByText('Email is required')).toBeDefined()
  expect(screen.getByLabelText('Email').getAttribute('aria-invalid')).toBe('true')
})

test('has no error message or aria-invalid when error is absent', () => {
  render(<TextField label="Email" value="" onChange={vi.fn()} />)
  expect(screen.getByLabelText('Email').getAttribute('aria-invalid')).toBe('false')
})
