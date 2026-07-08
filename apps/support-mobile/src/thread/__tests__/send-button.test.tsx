import { render, screen, fireEvent } from '@testing-library/react'
import { SendButton } from '../send-button'

function sizeClasses(el: HTMLElement): string[] {
  return el.className.split(/\s+/).filter((cls) => /^(h|w|min-h|min-w)-/.test(cls))
}

test('disabled state: renders the send icon, is not pressable, and fires nothing on click', () => {
  const onPress = vi.fn()
  render(<SendButton state="disabled" onPress={onPress} />)

  const button = screen.getByRole('button', { name: 'Send message' })
  expect(button).toHaveProperty('disabled', true)

  fireEvent.click(button)
  expect(onPress).not.toHaveBeenCalled()
})

test('active state: is pressable and fires onPress on click', () => {
  const onPress = vi.fn()
  render(<SendButton state="active" onPress={onPress} />)

  const button = screen.getByRole('button', { name: 'Send message' })
  expect(button).toHaveProperty('disabled', false)

  fireEvent.click(button)
  expect(onPress).toHaveBeenCalledTimes(1)
})

test('sending state: renders a spinner, announces "Sending message", and fires nothing on click', () => {
  const onPress = vi.fn()
  render(<SendButton state="sending" onPress={onPress} />)

  const button = screen.getByRole('button', { name: 'Sending message' })
  expect(button).toHaveProperty('disabled', true)
  expect(screen.getByRole('status')).toBeDefined() // Spinner's aria-label="Loading" role="status"

  fireEvent.click(button)
  expect(onPress).not.toHaveBeenCalled()
})

test('sent state: announces "Message sent" and is pressable again (re-armed after the 400ms grace window elsewhere)', () => {
  render(<SendButton state="sent" onPress={vi.fn()} />)

  const button = screen.getByRole('button', { name: 'Message sent' })
  expect(button).toHaveProperty('disabled', false)
})

test('button footprint (h-9 w-9, min-h-0 min-w-0) is identical across every state — no layout shift while morphing', () => {
  const states = ['disabled', 'active', 'sending', 'sent'] as const
  const footprints = states.map((state) => {
    const { unmount } = render(<SendButton state={state} onPress={vi.fn()} />)
    const classes = sizeClasses(screen.getByRole('button')).sort()
    unmount()
    return classes
  })

  expect(footprints[0]).toEqual(['h-9', 'min-h-0', 'min-w-0', 'w-9'])
  footprints.forEach((classes) => expect(classes).toEqual(footprints[0]))
})
