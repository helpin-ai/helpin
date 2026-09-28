// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PortalAttachmentTray } from '../PortalAttachmentTray'
import { pastedFiles, usePortalFileDrop } from '../portalFileDrop'
import type { PortalAttachmentUploads } from '../usePortalAttachmentUploads'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root
let container: HTMLDivElement

beforeEach(() => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

function uploads(overrides: Partial<PortalAttachmentUploads> = {}): PortalAttachmentUploads {
  return { files: [], notice: null, attachmentIds: [], uploading: false, blocked: false, addFiles: vi.fn(), retry: vi.fn(), remove: vi.fn(), clear: vi.fn(), ...overrides }
}

function dataTransfer(files: File[]) {
  return { types: ['Files'], files, dropEffect: 'none' }
}

describe('PortalAttachmentTray', () => {
  it('offers multi-file selection limited to the server types', async () => {
    await act(async () => root.render(<PortalAttachmentTray uploads={uploads()} policy={{ max_bytes: 1, max_files: 10, content_types: ['image/png', 'video/x-matroska'] }} />))
    const input = container.querySelector<HTMLInputElement>('input[type="file"]')!
    expect(input.multiple).toBe(true)
    expect(input.accept.split(',')).toEqual(expect.arrayContaining(['image/png', '.png', 'video/x-matroska', '.mkv']))
  })

  it('shows progress, errors, retry, and remove for each file', async () => {
    const state = uploads({
      files: [
        { id: '1', name: 'shot.png', size: 2048, type: 'image/png', previewUrl: 'blob:1', status: 'uploading', progress: 40 },
        { id: '2', name: 'clip.mkv', size: 4096, type: 'video/x-matroska', status: 'error', progress: 0, error: 'clip.mkv files are not supported' },
      ],
      uploading: true,
      blocked: true,
    })
    await act(async () => root.render(<PortalAttachmentTray uploads={state} />))
    expect(container.textContent).toContain('Uploading 40%')
    expect(container.textContent).toContain('clip.mkv files are not supported')
    expect(container.querySelector('img')?.getAttribute('src')).toBe('blob:1')
    const retry = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Retry')
    await act(async () => retry?.click())
    expect(state.retry).toHaveBeenCalledWith('2')
    await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="Remove shot.png"]')?.click())
    expect(state.remove).toHaveBeenCalledWith('1')
  })

  it('attaches dropped files and shows a cue while dragging', async () => {
    const addFiles = vi.fn()
    function DropTarget() {
      const { dragging, dropProps } = usePortalFileDrop(addFiles, true)
      return <div data-testid="target" data-dragging={dragging} {...dropProps} />
    }
    await act(async () => root.render(<DropTarget />))
    const target = container.querySelector<HTMLDivElement>('[data-testid="target"]')!
    const file = new File(['x'], 'drop.png', { type: 'image/png' })
    const enter = new Event('dragenter', { bubbles: true, cancelable: true })
    Object.assign(enter, { dataTransfer: dataTransfer([file]) })
    await act(async () => { target.dispatchEvent(enter) })
    expect(target.dataset.dragging).toBe('true')
    const drop = new Event('drop', { bubbles: true, cancelable: true })
    Object.assign(drop, { dataTransfer: dataTransfer([file]) })
    await act(async () => { target.dispatchEvent(drop) })
    expect(addFiles).toHaveBeenCalledWith([file])
    expect(target.dataset.dragging).toBe('false')
  })

  it('takes files from a paste', () => {
    const file = new File(['x'], 'pasted.png', { type: 'image/png' })
    expect(pastedFiles({ clipboardData: { files: [file] } } as unknown as React.ClipboardEvent)).toEqual([file])
    expect(pastedFiles({ clipboardData: { files: [] } } as unknown as React.ClipboardEvent)).toEqual([])
  })
})
