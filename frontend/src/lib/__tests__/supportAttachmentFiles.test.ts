import { describe, expect, it } from 'vitest'
import { acceptAttributeFor, fileRejection, isPreviewableImageType, withInferredFileType } from '../supportAttachmentFiles'

const policy = { max_bytes: 1024, content_types: ['image/png', 'image/heic', 'video/x-matroska', 'application/pdf'] }

describe('supportAttachmentFiles', () => {
  it('shows images inline except HEIC and HEIF', () => {
    expect(isPreviewableImageType('image/png')).toBe(true)
    expect(isPreviewableImageType('image/heic')).toBe(false)
    expect(isPreviewableImageType('image/heif')).toBe(false)
    expect(isPreviewableImageType('video/mp4')).toBe(false)
  })

  it('infers a type from the extension only when the browser reports none', () => {
    expect(withInferredFileType(new File(['x'], 'clip.MKV')).type).toBe('video/x-matroska')
    expect(withInferredFileType(new File(['x'], 'photo.heic', { type: 'application/octet-stream' })).type).toBe('image/heic')
    expect(withInferredFileType(new File(['x'], 'clip.avi', { type: 'video/avi' })).type).toBe('video/x-msvideo')
    expect(withInferredFileType(new File(['x'], 'report.pdf', { type: 'application/pdf' })).type).toBe('application/pdf')
    expect(withInferredFileType(new File(['x'], 'unknown.xyz')).type).toBe('')
  })

  it('builds a picker filter from the server types plus their extensions', () => {
    const accept = acceptAttributeFor(policy.content_types).split(',')
    expect(accept).toEqual(expect.arrayContaining(['image/heic', '.heic', '.mkv', '.pdf', 'image/png', '.png']))
    expect(accept).not.toContain('.zip')
  })

  it('explains why a file is refused', () => {
    expect(fileRejection(new File([], 'empty.png', { type: 'image/png' }), policy)).toContain('is empty')
    expect(fileRejection(new File(['x'.repeat(2048)], 'big.png', { type: 'image/png' }), policy)).toContain('larger than 1 KB')
    expect(fileRejection(new File(['x'], 'tool.exe', { type: 'application/x-msdownload' }), policy)).toContain('isn’t a supported file type')
    expect(fileRejection(new File(['x'], 'ok.png', { type: 'image/png' }), policy)).toBeNull()
  })
})
