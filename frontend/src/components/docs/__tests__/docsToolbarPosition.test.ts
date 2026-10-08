import { describe, expect, it } from 'vitest'
import { docsToolbarPosition } from '../docsToolbarPosition'

const container = { left: 300, right: 1200, top: 80, bottom: 800 }
const viewport = { width: 1280, height: 900 }

describe('document toolbar placement', () => {
  it('keeps a left-edge text selection inside the editor, away from the sidebar', () => {
    const result = docsToolbarPosition({ left: 320, right: 340, top: 200, bottom: 220 }, container, { width: 440, height: 42 }, viewport)
    expect(result.left).toBe(8)
    expect(result.top).toBe(70)
  })
  it('keeps a wider URL editor inside the right boundary', () => {
    const result = docsToolbarPosition({ left: 1170, right: 1190, top: 200, bottom: 220 }, container, { width: 480, height: 180 }, viewport, true)
    expect(result.left + 480).toBe(892)
    expect(result.top).toBe(148)
  })
  it('opens below a selection when there is no room above', () => {
    const result = docsToolbarPosition({ left: 600, right: 620, top: 90, bottom: 110 }, container, { width: 440, height: 42 }, viewport)
    expect(result.top).toBe(38)
  })
  it('constrains both controls on narrow and partially scrolled screens', () => {
    const result = docsToolbarPosition({ left: 12, right: 40, top: 25, bottom: 45 }, { left: 0, right: 360, top: -200, bottom: 1000 }, { width: 480, height: 180 }, { width: 360, height: 640 })
    expect(result.left).toBe(8)
    expect(result.maxWidth).toBe(344)
    expect(result.top - 200).toBeGreaterThanOrEqual(8)
  })
})
