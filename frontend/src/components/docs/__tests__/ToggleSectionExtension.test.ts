// @vitest-environment jsdom
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { describe, expect, it } from 'vitest'
import { ToggleSectionExtension } from '../ToggleSectionExtension'

describe('ToggleSectionExtension', () => {
  it('serializes the shared toggle block contract', () => {
    const editor = new Editor({
      extensions: [StarterKit, ToggleSectionExtension],
      content: {
        type: 'doc',
        content: [
          {
            type: 'toggleSection',
            attrs: {
              title: 'Authentication & Setup',
              open: true,
              icon: '🔑',
              badgeText: '3 topics',
            },
            content: [
              {
                type: 'paragraph',
                content: [{ type: 'text', text: 'How to Get Your ContentStudio API Key' }],
              },
            ],
          },
        ],
      },
    })

    try {
      const html = editor.getHTML()
      expect(html).toContain('class="docs-toggle-section"')
      expect(html).toContain('data-toggle-style="helpScoutCard"')
      expect(html).toContain('<span class="docs-toggle-icon">🔑</span>')
      expect(html).toContain('<span class="docs-toggle-title">Authentication &amp; Setup</span>')
      expect(html).toContain('<span class="docs-toggle-badge">3 topics</span>')
      expect(html).toContain('<span class="docs-toggle-chevron" aria-hidden="true"></span>')
      expect(html).toContain('<div class="docs-toggle-content" data-toggle-content="">')
    } finally {
      editor.destroy()
    }
  })
})
