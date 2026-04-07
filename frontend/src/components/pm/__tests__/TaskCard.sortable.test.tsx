import { describe, expect, it } from 'vitest'

import { getSortableTaskCardStyle } from '../TaskCard.sortable'

describe('getSortableTaskCardStyle', () => {
  it('uses the sortable hook transition for non-dragging cards', () => {
    expect(getSortableTaskCardStyle({
      transform: { x: 0, y: 24, scaleX: 1, scaleY: 1 },
      transition: 'transform 0ms linear',
      isDragging: false,
    })).toEqual({
      transform: 'translate3d(0px, 24px, 0) scaleX(1) scaleY(1)',
      transition: 'transform 0ms linear',
    })
  })

  it('falls back to smooth shift transition when hook provides no transition', () => {
    expect(getSortableTaskCardStyle({
      transform: { x: 0, y: 24, scaleX: 1, scaleY: 1 },
      transition: undefined,
      isDragging: false,
    })).toEqual({
      transform: 'translate3d(0px, 24px, 0) scaleX(1) scaleY(1)',
      transition: 'transform 200ms cubic-bezier(0.25, 1, 0.5, 1)',
    })
  })

  it('suppresses transitions for the actively dragged card', () => {
    expect(getSortableTaskCardStyle({
      transform: { x: 0, y: 24, scaleX: 1, scaleY: 1 },
      transition: 'transform 200ms ease',
      isDragging: true,
    })).toEqual({
      transform: 'translate3d(0px, 24px, 0) scaleX(1) scaleY(1)',
      transition: undefined,
    })
  })
})
