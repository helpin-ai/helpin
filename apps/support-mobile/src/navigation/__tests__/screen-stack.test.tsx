import { render, screen } from '@testing-library/react'
import { GestureScreen, backFallbackPath, resolveDirection } from '../screen-stack'

test('history index changes map to stack direction', () => {
  expect(resolveDirection(0, 1)).toBe('push')
  expect(resolveDirection(2, 1)).toBe('pop')
  expect(resolveDirection(1, 1)).toBe('replace')
})

test('back with no history falls back to inbox or workspaces', () => {
  expect(backFallbackPath('/w/acme/support/conv-1')).toBe('/w/acme/support')
  expect(backFallbackPath('/login')).toBe('/workspaces')
})

test('GestureScreen owns a per-instance gesture value and renders children', () => {
  // Each mount gets its own motion value (gesture is scoped per screen, not
  // shared across exiting+entering screens), starting untranslated.
  const { container } = render(
    <GestureScreen enabled onBack={vi.fn()}>
      <span>screen content</span>
    </GestureScreen>,
  )
  expect(screen.getByText('screen content')).toBeDefined()
  const host = container.firstElementChild as HTMLElement
  expect(host.style.transform === '' || host.style.transform === 'none').toBe(true)
})
