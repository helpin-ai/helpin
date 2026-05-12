import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('TipTap placeholder CSS', () => {
  it('supports root and direct-child placeholder classes', () => {
    const css = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

    expect(css).toContain('.tiptap.is-editor-empty > p.is-empty:first-child::before')
    expect(css).toContain('.ProseMirror.is-editor-empty > p.is-empty:first-child::before')
    expect(css).toContain('.tiptap > p.is-editor-empty:first-child::before')
    expect(css).toContain('.ProseMirror > p.is-editor-empty:first-child::before')
  })

  it('does not render the empty editor placeholder on every empty paragraph', () => {
    const css = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

    expect(css).not.toMatch(/\.tiptap\.is-editor-empty\s+p\.is-empty::before/)
    expect(css).not.toMatch(/\.ProseMirror\.is-editor-empty\s+p\.is-empty::before/)
    expect(css).not.toMatch(/\.tiptap\s+p\.is-editor-empty:first-child::before/)
    expect(css).not.toMatch(/\.tiptap\s*>\s*\.is-editor-empty::before/)
  })
})
