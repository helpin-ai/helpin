// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'

import { getSortableTaskCardStyle, shouldIgnoreTaskCardDrag } from '../TaskCard.sortable'

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
      transition: 'transform 120ms cubic-bezier(0.2, 0, 0, 1)',
    })
  })

  it('suppresses transform and transition for the actively dragged source card', () => {
    expect(getSortableTaskCardStyle({
      transform: { x: 0, y: 24, scaleX: 1, scaleY: 1 },
      transition: 'transform 200ms ease',
      isDragging: true,
    })).toEqual({
      transform: undefined,
      transition: undefined,
    })
  })
})

describe('shouldIgnoreTaskCardDrag', () => {
  it('allows drags from non-interactive card content', () => {
    const card = document.createElement('article')
    const title = document.createElement('h4')
    card.appendChild(title)

    expect(shouldIgnoreTaskCardDrag(title, card)).toBe(false)
  })

  it('blocks drags from native interactive controls', () => {
    const card = document.createElement('article')
    const button = document.createElement('button')
    card.appendChild(button)

    expect(shouldIgnoreTaskCardDrag(button, card)).toBe(true)
  })

  it('blocks drags from elements marked as no-drag', () => {
    const card = document.createElement('article')
    const wrapper = document.createElement('span')
    const icon = document.createElement('span')
    wrapper.dataset.noTaskCardDrag = 'true'
    wrapper.appendChild(icon)
    card.appendChild(wrapper)

    expect(shouldIgnoreTaskCardDrag(icon, card)).toBe(true)
  })
})
