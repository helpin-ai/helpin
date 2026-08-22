import { useResolvedTransitionStore } from '../resolved-transition-store'

beforeEach(() => {
  useResolvedTransitionStore.setState({ pendingResolvedId: null })
})

test('a resolved transition can be consumed exactly once', () => {
  useResolvedTransitionStore.getState().markResolved('conv-1')

  expect(useResolvedTransitionStore.getState().consumeResolved()).toBe('conv-1')
  expect(useResolvedTransitionStore.getState().consumeResolved()).toBeNull()
  expect(useResolvedTransitionStore.getState().pendingResolvedId).toBeNull()
})
