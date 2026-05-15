// @vitest-environment jsdom
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { describe, expect, it } from 'vitest'
import { HtmlBlockExtension } from '../HtmlBlockExtension'

describe('HtmlBlockExtension', () => {
  it('serializes htmlBlock fragments as rendered HTML instead of escaped text', () => {
    const editor = new Editor({
      extensions: [StarterKit, HtmlBlockExtension],
      content: {
        type: 'doc',
        content: [
          {
            type: 'htmlBlock',
            attrs: {
              html: '<div><strong>Safe content</strong></div>',
            },
          },
        ],
      },
    })

    try {
      const html = editor.getHTML()
      expect(html).toContain('<div><strong>Safe content</strong></div>')
      expect(html).not.toContain('&lt;strong&gt;')
    } finally {
      editor.destroy()
    }
  })

  it('keeps sandboxed htmlBlock source unescaped for trusted raw embeds', () => {
    const editor = new Editor({
      extensions: [StarterKit, HtmlBlockExtension],
      content: {
        type: 'doc',
        content: [
          {
            type: 'htmlBlock',
            attrs: {
              renderMode: 'sandboxed',
              html: '<iframe src="https://www.youtube.com/embed/abc123"></iframe>',
            },
          },
        ],
      },
    })

    try {
      const html = editor.getHTML()
      expect(html).toContain('class="docs-html-block-frame"')
      expect(html).toContain('srcdoc="<iframe src=&quot;https://www.youtube.com/embed/abc123&quot;></iframe>"')
    } finally {
      editor.destroy()
    }
  })

  it('renders full HTML documents in an iframe even without explicit sandbox mode', () => {
    const editor = new Editor({
      extensions: [StarterKit, HtmlBlockExtension],
      content: {
        type: 'doc',
        content: [
          {
            type: 'htmlBlock',
            attrs: {
              html: '<!doctype html><html><head><style>.box{color:red}</style></head><body><div class="box">Diagram</div><script>window.ok=true</script></body></html>',
            },
          },
        ],
      },
    })

    try {
      const html = editor.getHTML()
      expect(html).toContain('class="docs-html-block-frame"')
      expect(html).toContain('<style>.box{color:red}</style>')
      expect(html).toContain('<script>window.ok=true</script>')
      expect(html).toContain('srcdoc="<!doctype html>')
    } finally {
      editor.destroy()
    }
  })
})
