import { activeGesture, claimGesture, releaseGesture } from '../gesture-claim'

afterEach(() => {
  const owner = activeGesture()
  if (owner) releaseGesture(owner)
})

test('claim succeeds when no gesture is active', () => {
  expect(claimGesture('row-swipe')).toBe(true)
  expect(activeGesture()).toBe('row-swipe')
})

test('a conflicting kind is denied while another is active', () => {
  expect(claimGesture('row-swipe')).toBe(true)
  expect(claimGesture('pull')).toBe(false)
  expect(activeGesture()).toBe('row-swipe')
})

test('re-claiming the same kind while active succeeds', () => {
  expect(claimGesture('pull')).toBe(true)
  expect(claimGesture('pull')).toBe(true)
  expect(activeGesture()).toBe('pull')
})

test('release frees the token for other kinds', () => {
  expect(claimGesture('row-swipe')).toBe(true)
  releaseGesture('row-swipe')
  expect(activeGesture()).toBeNull()
  expect(claimGesture('pull')).toBe(true)
})

test('release by a non-owner is a no-op', () => {
  expect(claimGesture('pull')).toBe(true)
  releaseGesture('row-swipe')
  expect(activeGesture()).toBe('pull')
})
