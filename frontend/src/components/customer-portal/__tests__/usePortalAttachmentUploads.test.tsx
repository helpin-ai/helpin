// @vitest-environment jsdom
import { act, useEffect } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CustomerPortalUploadOptions } from '@/lib/services/customerPortalService'
import { usePortalAttachmentUploads, type PortalAttachmentUploads } from '../usePortalAttachmentUploads'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const policy = { max_bytes: 10_000, content_types: ['image/png', 'video/x-matroska', 'application/pdf'], max_files: 3 }

type Pending = { file: File; options: CustomerPortalUploadOptions; resolve: (id: string) => void; reject: (error: Error) => void }

let root: Root
let container: HTMLDivElement
let pending: Pending[]
let current: PortalAttachmentUploads
const upload = vi.fn((file: File, options: CustomerPortalUploadOptions) => new Promise<{ id: string; name: string }>((resolve, reject) => {
  pending.push({ file, options, resolve: (id) => resolve({ id, name: file.name }), reject })
}))

function Harness({ onValue }: { onValue: (value: PortalAttachmentUploads) => void }) {
  const uploads = usePortalAttachmentUploads({ upload, policy })
  useEffect(() => onValue(uploads))
  return null
}

const capture = (value: PortalAttachmentUploads) => { current = value }

async function flush() {
  await act(async () => { await Promise.resolve(); await Promise.resolve() })
}

beforeEach(async () => {
  pending = []
  upload.mockClear()
  URL.createObjectURL = vi.fn(() => 'blob:preview')
  URL.revokeObjectURL = vi.fn()
  container = document.createElement('div')
  root = createRoot(container)
  await act(async () => root.render(<Harness onValue={capture} />))
})

afterEach(() => {
  act(() => root.unmount())
})

const png = (name = 'a.png', size = 10) => new File(['x'.repeat(size)], name, { type: 'image/png' })

describe('usePortalAttachmentUploads', () => {
  it('uploads two files at a time, reports progress, and collects ids', async () => {
    await act(async () => current.addFiles([png('a.png'), png('b.png'), png('c.png')]))
    expect(upload).toHaveBeenCalledTimes(2)
    expect(current.files.map((file) => file.status)).toEqual(['uploading', 'uploading', 'queued'])
    expect(current.blocked).toBe(true)
    await act(async () => pending[0].options.onProgress?.(42))
    expect(current.files[0].progress).toBe(42)
    await act(async () => pending[0].resolve('att-a'))
    await flush()
    expect(upload).toHaveBeenCalledTimes(3)
    await act(async () => { pending[1].resolve('att-b'); pending[2].resolve('att-c') })
    await flush()
    expect(current.attachmentIds).toEqual(['att-a', 'att-b', 'att-c'])
    expect(current.blocked).toBe(false)
  })

  it('rejects files the server would refuse and caps files per message', async () => {
    await act(async () => current.addFiles([
      new File([], 'empty.png', { type: 'image/png' }),
      png('huge.png', 20_000),
      new File(['x'], 'tool.exe', { type: 'application/x-msdownload' }),
      png('1.png'), png('2.png'), png('3.png'), png('4.png'),
    ]))
    expect(current.files.map((file) => file.name)).toEqual(['1.png', '2.png', '3.png'])
    expect(current.notice).toContain('empty.png is empty')
    expect(current.notice).toContain('huge.png is larger than')
    expect(current.notice).toContain('tool.exe isn’t a supported file type')
    expect(current.notice).toContain('up to 3 files')
  })

  it('infers video types from the extension before uploading', async () => {
    await act(async () => current.addFiles([new File(['x'], 'screen.mkv')]))
    expect(upload.mock.calls[0][0].type).toBe('video/x-matroska')
  })

  it('keeps sending blocked until a failed upload is retried or removed', async () => {
    await act(async () => current.addFiles([png('a.png')]))
    await act(async () => pending[0].reject(new Error('Network error')))
    await flush()
    expect(current.files[0]).toMatchObject({ status: 'error', error: 'Network error' })
    expect(current.blocked).toBe(true)
    await act(async () => current.retry(current.files[0].id))
    expect(upload).toHaveBeenCalledTimes(2)
    await act(async () => pending[1].resolve('att-a'))
    await flush()
    expect(current.blocked).toBe(false)
    expect(current.attachmentIds).toEqual(['att-a'])
  })

  it('cancels a removed upload and releases its preview', async () => {
    await act(async () => current.addFiles([png('a.png')]))
    const signal = pending[0].options.signal!
    await act(async () => current.remove(current.files[0].id))
    expect(signal.aborted).toBe(true)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview')
    expect(current.files).toEqual([])
  })

  it('cancels uploads when the composer unmounts', async () => {
    await act(async () => current.addFiles([png('a.png')]))
    const signal = pending[0].options.signal!
    act(() => root.unmount())
    expect(signal.aborted).toBe(true)
    root = createRoot(container)
  })
})
