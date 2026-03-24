// @vitest-environment jsdom

import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  diffRemovedInlineAttachmentIds,
  extractInlineAttachmentIds,
  mergeAttachmentIds,
  removeInlineImagesByAttachmentIds,
} from '../editorImageAttachments'
import { uploadEditorImage } from '@/hooks/useEditorImageUpload'
import { ResizableImageExtension } from '@/components/ui/resizable-image-extension'

vi.mock('@/lib/services/pmAttachmentService', () => ({
  pmAttachmentService: {
    initiateUpload: vi.fn(),
    confirmUpload: vi.fn(),
  },
}))

vi.mock('@/lib/api', () => ({
  uploadToS3: vi.fn(),
}))

const { pmAttachmentService } = await import('@/lib/services/pmAttachmentService')
const { uploadToS3 } = await import('@/lib/api')

describe('editorImageAttachments', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('extracts unique inline attachment ids from image HTML', () => {
    const html = [
      '<p>Before</p>',
      '<img src="https://cdn.example.com/a.png" data-attachment-id="att-1" />',
      '<p><img src="https://cdn.example.com/b.png" data-attachment-id="att-2" /></p>',
      '<img src="https://cdn.example.com/c.png" data-attachment-id="att-1" />',
    ].join('')

    expect(extractInlineAttachmentIds(html)).toEqual(['att-1', 'att-2'])
  })

  it('removes only the targeted inline images', () => {
    const html = [
      '<p><img src="https://cdn.example.com/a.png" data-attachment-id="att-1" /></p>',
      '<p><img src="https://cdn.example.com/b.png" data-attachment-id="att-2" /></p>',
    ].join('')

    const updated = removeInlineImagesByAttachmentIds(html, ['att-1'])

    expect(updated).not.toContain('data-attachment-id="att-1"')
    expect(updated).toContain('data-attachment-id="att-2"')
  })

  it('diffs removed inline attachment ids between two HTML snapshots', () => {
    const previousHtml = [
      '<p><img src="https://cdn.example.com/a.png" data-attachment-id="att-1" /></p>',
      '<p><img src="https://cdn.example.com/b.png" data-attachment-id="att-2" /></p>',
    ].join('')
    const nextHtml = '<p><img src="https://cdn.example.com/b.png" data-attachment-id="att-2" /></p>'

    expect(diffRemovedInlineAttachmentIds(previousHtml, nextHtml)).toEqual(['att-1'])
  })

  it('merges attachment ids from inline images and manual uploads without duplicates', () => {
    const descriptionHtml =
      '<p><img src="https://cdn.example.com/a.png" data-attachment-id="att-1" /></p>'

    expect(mergeAttachmentIds(['att-1', 'att-2'], extractInlineAttachmentIds(descriptionHtml))).toEqual([
      'att-1',
      'att-2',
    ])
  })

  it('returns attachment metadata after a successful image upload', async () => {
    vi.mocked(pmAttachmentService.initiateUpload).mockResolvedValue({
      data: {
        attachment: {
          id: 'att-123',
          workspace_id: 'ws-1',
          entity_type: 'editor_upload',
          entity_id: 'draft-1',
          file_name: 'clipboard.png',
          file_size: 12,
          content_type: 'image/png',
          storage_key: 'attachments/att-123',
          is_uploaded: false,
          uploaded_by_id: 'user-1',
          created_at: '2026-03-18T00:00:00Z',
        },
        url: 'https://upload.example.com/put',
        public_url: 'https://cdn.example.com/clipboard.png',
      },
      error: null,
      status: 200,
    })
    vi.mocked(uploadToS3).mockResolvedValue({ ok: true, error: null })
    vi.mocked(pmAttachmentService.confirmUpload).mockResolvedValue({
      data: null,
      error: null,
      status: 200,
    })

    const file = new File(['image-bytes'], 'clipboard.png', { type: 'image/png' })
    const result = await uploadEditorImage(file, {
      workspaceId: 'ws-1',
      entityType: 'editor_upload',
      entityId: 'draft-1',
    })

    expect(result).toEqual({
      attachmentId: 'att-123',
      publicUrl: 'https://cdn.example.com/clipboard.png',
    })
    expect(pmAttachmentService.confirmUpload).toHaveBeenCalledWith('ws-1', 'att-123')
  })

  it('preserves attachment ids when rendering and parsing resizable images', () => {
    const editor = new Editor({
      element: document.createElement('div'),
      extensions: [StarterKit, ResizableImageExtension],
      content: '',
    })

    editor.commands.setResizableImage({
      src: 'https://cdn.example.com/a.png',
      alt: 'Screenshot',
      attachmentId: 'att-9',
    })

    expect(editor.getHTML()).toContain('data-attachment-id="att-9"')

    editor.commands.setContent(
      '<img src="https://cdn.example.com/b.png" alt="Another" data-attachment-id="att-10" />',
    )

    const node = editor.getJSON().content?.[0]
    expect(node?.attrs?.attachmentId).toBe('att-10')

    editor.destroy()
  })
})
