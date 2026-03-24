import { describe, expect, it } from 'vitest'

import { getSortableStoryCardStyle } from '../StoryCard.sortable'

describe('getSortableStoryCardStyle', () => {
  it('uses the sortable hook transition for non-dragging cards', () => {
    expect(getSortableStoryCardStyle({
      transform: { x: 0, y: 24, scaleX: 1, scaleY: 1 },
      transition: 'transform 0ms linear',
      isDragging: false,
    })).toEqual({
      transform: 'translate3d(0px, 24px, 0) scaleX(1) scaleY(1)',
      transition: 'transform 0ms linear',
    })
  })

  it('suppresses transitions for the actively dragged card', () => {
    expect(getSortableStoryCardStyle({
      transform: { x: 0, y: 24, scaleX: 1, scaleY: 1 },
      transition: 'transform 200ms ease',
      isDragging: true,
    })).toEqual({
      transform: 'translate3d(0px, 24px, 0) scaleX(1) scaleY(1)',
      transition: undefined,
    })
  })
})
