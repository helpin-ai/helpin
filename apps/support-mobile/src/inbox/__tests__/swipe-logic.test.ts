import { swipeState } from '../swipeable-row'
import { rubberBand } from '../use-pull-to-refresh'

describe('swipeState', () => {
  test('below the arm threshold is idle', () => {
    expect(swipeState(50, 400)).toBe('idle')
  })

  test('past 96px but below 60% of width is armed', () => {
    expect(swipeState(120, 400)).toBe('armed')
  })

  test('past 60% of width auto-commits', () => {
    expect(swipeState(280, 400)).toBe('auto')
  })

  test('is symmetric for negative (leftward) drags', () => {
    expect(swipeState(-50, 400)).toBe('idle')
    expect(swipeState(-120, 400)).toBe('armed')
    expect(swipeState(-280, 400)).toBe('auto')
  })

  test('exactly at the 96px threshold is armed', () => {
    expect(swipeState(96, 400)).toBe('armed')
  })

  test('exactly at 60% of width auto-commits', () => {
    expect(swipeState(240, 400)).toBe('auto')
  })
})

describe('rubberBand', () => {
  test('halves the raw pull distance', () => {
    expect(rubberBand(140)).toBe(70)
  })

  test('handles zero', () => {
    expect(rubberBand(0)).toBe(0)
  })
})
