import { backFallbackPath, resolveDirection } from '../screen-stack'

test('history index changes map to stack direction', () => {
  expect(resolveDirection(0, 1)).toBe('push')
  expect(resolveDirection(2, 1)).toBe('pop')
  expect(resolveDirection(1, 1)).toBe('replace')
})

test('back with no history falls back to inbox or workspaces', () => {
  expect(backFallbackPath('/w/acme/support/conv-1')).toBe('/w/acme/support')
  expect(backFallbackPath('/login')).toBe('/workspaces')
})
