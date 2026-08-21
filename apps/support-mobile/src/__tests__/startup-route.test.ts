import { resolveStartupRoute } from '@mobile/router'

test('startup route waits while authentication is loading', () => {
  expect(resolveStartupRoute(true, null, false)).toBeNull()
})

test('startup route sends signed-out users to login', () => {
  expect(resolveStartupRoute(false, null, false)).toBe('/login')
})

test('startup route sends signed-in users through workspace resolution', () => {
  expect(resolveStartupRoute(false, { id: 'user-1' }, false)).toBe('/workspaces')
})

test('startup route preserves an existing session when the server is temporarily unreachable', () => {
  expect(resolveStartupRoute(false, null, true)).toBe('/workspaces')
})
