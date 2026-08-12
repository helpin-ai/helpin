// @vitest-environment jsdom

import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { describe, expect, it } from 'vitest'

import { ResizableImageExtension } from '@/components/ui/resizable-image-extension'
import {
  fitScale,
  hasCoverShape,
  parseAnnotationState,
  shapeBounds,
  type AnnotationState,
} from '../core/annotationTypes'
import { sanitizeHtml } from '@/components/docs/htmlSanitizer'
import { stripImageAnnotationState } from '@/lib/docsPublishTransforms'

const state: AnnotationState = {
  version: 1,
  baseWidth: 1920,
  baseHeight: 1080,
  shapes: [
    { id: 'a1', type: 'arrow', points: [10, 20, 300, 400], color: '#ef4444', strokeWidth: 4 },
    { id: 'r1', type: 'rect', x: 5, y: 6, width: 100, height: 50, color: '#3b82f6', strokeWidth: 2, rotation: 0 },
  ],
}

function editorWithImage(html: string) {
  return new Editor({
    extensions: [StarterKit, ResizableImageExtension],
    content: html,
  })
}

describe('parseAnnotationState', () => {
  it('accepts both an object and its JSON string form', () => {
    expect(parseAnnotationState(state)).toEqual(state)
    expect(parseAnnotationState(JSON.stringify(state))).toEqual(state)
  })

  it('returns null for malformed input instead of throwing', () => {
    // This runs during document load — one bad attribute must never take the editor down.
    expect(parseAnnotationState('{not json')).toBeNull()
    expect(parseAnnotationState('')).toBeNull()
    expect(parseAnnotationState(null)).toBeNull()
    expect(parseAnnotationState(undefined)).toBeNull()
    expect(parseAnnotationState(42)).toBeNull()
    expect(parseAnnotationState({ shapes: [] })).toBeNull()
    expect(parseAnnotationState({ baseWidth: 0, baseHeight: 0, shapes: [] })).toBeNull()
    expect(parseAnnotationState({ baseWidth: 10, baseHeight: 10 })).toBeNull()
  })

  it('drops unknown and malformed shapes rather than failing the whole state', () => {
    const parsed = parseAnnotationState({
      version: 99,
      baseWidth: 800,
      baseHeight: 600,
      shapes: [
        { id: 'ok', type: 'rect', x: 1, y: 2, width: 3, height: 4 },
        { id: 'future', type: 'hologram', x: 1, y: 2 },
        { id: 'broken-arrow', type: 'arrow', points: [1, 2] },
        { type: 'rect', x: 1, y: 2, width: 3, height: 4 },
        null,
      ],
    })
    expect(parsed?.shapes.map((shape) => shape.id)).toEqual(['ok'])
    expect(parsed?.version).toBe(99)
  })

  it('fills in defaults for optional style fields', () => {
    const parsed = parseAnnotationState({
      baseWidth: 10,
      baseHeight: 10,
      shapes: [{ id: 's', type: 'rect', x: 0, y: 0, width: 1, height: 1 }],
    })
    const shape = parsed?.shapes[0]
    expect(shape).toMatchObject({ strokeWidth: 4, rotation: 0, color: '#ef4444' })
  })

  it('keeps legacy text and callouts editable by adding resize defaults', () => {
    const parsed = parseAnnotationState({
      baseWidth: 800,
      baseHeight: 600,
      shapes: [
        { id: 'text', type: 'text', x: 10, y: 20, text: 'Legacy text' },
        { id: 'callout', type: 'callout', x: 30, y: 40, width: 200, height: 80, text: 'Legacy note' },
      ],
    })

    expect(parsed?.shapes[0]).toMatchObject({ width: 260, fontSize: 24 })
    expect(parsed?.shapes[1]).toMatchObject({ fontSize: 16 })
  })
})

describe('hasCoverShape', () => {
  it('detects redaction shapes, which drive the permanent-flatten default', () => {
    expect(hasCoverShape(state)).toBe(false)
    expect(hasCoverShape({ ...state, shapes: [{ id: 'c', type: 'cover', x: 0, y: 0, width: 5, height: 5, color: '#111827' }] })).toBe(true)
    expect(hasCoverShape(null)).toBe(false)
  })
})

describe('shapeBounds', () => {
  it('is orientation-independent for arrows drawn right-to-left', () => {
    expect(shapeBounds({ id: 'a', type: 'arrow', points: [300, 400, 10, 20], color: '#000', strokeWidth: 2 }))
      .toEqual({ x: 10, y: 20, width: 290, height: 380 })
  })

  it('expresses an ellipse as its bounding box, not its centre', () => {
    expect(shapeBounds({ id: 'e', type: 'ellipse', x: 100, y: 100, radiusX: 20, radiusY: 10, color: '#000', strokeWidth: 2, rotation: 0 }))
      .toEqual({ x: 80, y: 90, width: 40, height: 20 })
  })
})

describe('fitScale', () => {
  it('scales a large screenshot down to fit the stage', () => {
    // 1920x1080 into an 800px-wide, 520px-tall dialog — width is the binding constraint.
    expect(fitScale({ width: 1920, height: 1080 }, 800, 520)).toBeCloseTo(800 / 1920)
    // A tall image is bound by height instead.
    expect(fitScale({ width: 600, height: 2000 }, 800, 520)).toBeCloseTo(520 / 2000)
  })

  it('never upscales, so authoring never shows a softer image than the export', () => {
    expect(fitScale({ width: 200, height: 100 }, 1200, 520)).toBe(1)
  })

  it('round-trips a pointer position through the scale it produced', () => {
    const scale = fitScale({ width: 1920, height: 1080 }, 800, 520)
    const sourcePoint = { x: 960, y: 540 }
    const displayPoint = { x: sourcePoint.x * scale, y: sourcePoint.y * scale }
    expect(displayPoint.x / scale).toBeCloseTo(sourcePoint.x)
    expect(displayPoint.y / scale).toBeCloseTo(sourcePoint.y)
  })

  it('falls back to 1 before the image or container has been measured', () => {
    expect(fitScale({ width: 0, height: 0 }, 800, 520)).toBe(1)
    expect(fitScale({ width: 1920, height: 1080 }, 0, 520)).toBe(1)
  })
})

describe('resizableImage annotation attributes', () => {
  it('round-trips annotation state through renderHTML and parseHTML', () => {
    const editor = editorWithImage(
      `<img src="https://example.test/a.png" data-attachment-id="att-1" data-source-attachment-id="src-1" data-annotation='${JSON.stringify(state)}'>`,
    )
    const node = editor.state.doc.firstChild
    expect(node?.attrs.annotationState).toEqual(state)
    expect(node?.attrs.sourceAttachmentId).toBe('src-1')

    const html = editor.getHTML()
    expect(html).toContain('data-source-attachment-id="src-1"')

    // Re-parsing the serialised form must produce the same state — this is the path a saved
    // document takes on every reload.
    const reloaded = editorWithImage(html)
    expect(reloaded.state.doc.firstChild?.attrs.annotationState).toEqual(state)
    editor.destroy()
    reloaded.destroy()
  })

  it('loads a document with malformed annotation data instead of throwing', () => {
    const editor = editorWithImage(
      `<img src="https://example.test/a.png" data-attachment-id="att-1" data-annotation="{oops">`,
    )
    expect(editor.state.doc.firstChild?.attrs.annotationState).toBeNull()
    editor.destroy()
  })

  it('omits the annotation attributes entirely when there is no annotation', () => {
    const editor = editorWithImage('<img src="https://example.test/a.png" data-attachment-id="att-1">')
    const html = editor.getHTML()
    expect(html).not.toContain('data-annotation')
    expect(html).not.toContain('data-source-attachment-id')
    editor.destroy()
  })
})

describe('stripImageAnnotationState publish transform', () => {
  const ctx = { uploadConfig: { workspaceId: 'w', entityType: 'editor_upload' as const, entityId: 'd' } }

  it('applies only to annotated images', () => {
    expect(stripImageAnnotationState.appliesTo({ type: 'resizableImage', attrs: { src: 'x' } })).toBe(false)
    expect(stripImageAnnotationState.appliesTo({ type: 'resizableImage', attrs: { annotationState: state } })).toBe(true)
    expect(stripImageAnnotationState.appliesTo({ type: 'resizableImage', attrs: { sourceAttachmentId: 's' } })).toBe(true)
    expect(stripImageAnnotationState.appliesTo({ type: 'paragraph' })).toBe(false)
  })

  it('drops the editable state and the pointer to the pre-redaction original', async () => {
    const node = {
      type: 'resizableImage',
      attrs: {
        src: 'https://example.test/rendered.png',
        attachmentId: 'att-2',
        annotationState: null,
        sourceAttachmentId: 'src-1',
        artifactId: 'artifact-1',
        darkSrc: 'https://example.test/original-dark.png',
        darkAttachmentId: 'dark-1',
        width: '60%',
      },
    }
    const published = await stripImageAnnotationState.transform(node, ctx)
    expect(published.attrs).not.toHaveProperty('annotationState')
    expect(published.attrs).not.toHaveProperty('sourceAttachmentId')
    expect(published.attrs).not.toHaveProperty('artifactId')
    expect(published.attrs).not.toHaveProperty('darkSrc')
    expect(published.attrs).not.toHaveProperty('darkAttachmentId')
    // Everything a reader needs must survive.
    expect(published.attrs).toMatchObject({ src: 'https://example.test/rendered.png', attachmentId: 'att-2', width: '60%' })
  })
})

describe('sanitizeHtml with annotated images', () => {
  it('keeps the rendered image intact', () => {
    const safe = sanitizeHtml(
      `<img src="https://example.test/rendered.png" alt="Annotated" width="600" data-attachment-id="att-2">`,
    )
    expect(safe).toContain('src="https://example.test/rendered.png"')
    expect(safe).toContain('alt="Annotated"')
  })
})
