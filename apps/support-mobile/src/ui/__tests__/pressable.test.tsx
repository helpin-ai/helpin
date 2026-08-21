import { render, screen, fireEvent } from '@testing-library/react'
import { Pressable } from '../pressable'

test('fires onPress when clicked', () => {
  const onPress = vi.fn()
  render(<Pressable onPress={onPress}>Tap me</Pressable>)
  fireEvent.click(screen.getByRole('button', { name: /Tap me/ }))
  expect(onPress).toHaveBeenCalledTimes(1)
})

test('does not fire onPress when disabled', () => {
  const onPress = vi.fn()
  render(
    <Pressable onPress={onPress} disabled>
      Tap me
    </Pressable>,
  )
  fireEvent.click(screen.getByRole('button', { name: /Tap me/ }))
  expect(onPress).not.toHaveBeenCalled()
})

test('has a minimum 44px touch target', () => {
  render(<Pressable>Tap me</Pressable>)
  const button = screen.getByRole('button', { name: /Tap me/ })
  expect(button.className).toContain('min-h-[44px]')
})
