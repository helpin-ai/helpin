import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { CustomerPortalAttachmentPolicy, CustomerPortalUploadOptions, CustomerPortalUploadedFile } from '@/lib/services/customerPortalService'
import { fileRejection, isPreviewableImageType, withInferredFileType } from '@/lib/supportAttachmentFiles'

export type PortalUploadStatus = 'queued' | 'uploading' | 'uploaded' | 'error'

export interface PortalPendingFile {
  id: string
  name: string
  size: number
  type: string
  previewUrl?: string
  status: PortalUploadStatus
  progress: number
  error?: string
  attachmentId?: string
}

type Upload = (file: File, options: CustomerPortalUploadOptions) => Promise<CustomerPortalUploadedFile>

/** Uploads run two at a time so a large video does not hold up small files. */
const CONCURRENT_UPLOADS = 2
const DEFAULT_MAX_FILES = 10

interface Entry {
  file: File
  previewUrl?: string
  controller?: AbortController
}

/**
 * usePortalAttachmentUploads manages the files attached to one portal message:
 * validation against the server's rules, queued uploads with progress, retry,
 * removal, and cleanup. It mirrors the chat widget's attachment behaviour.
 */
export function usePortalAttachmentUploads({ upload, policy }: { upload: Upload; policy?: CustomerPortalAttachmentPolicy }) {
  const [files, setFiles] = useState<PortalPendingFile[]>([])
  const [notice, setNotice] = useState<string | null>(null)
  const entries = useRef(new Map<string, Entry>())
  const queue = useRef<string[]>([])
  const active = useRef(0)
  const uploadRef = useRef(upload)
  useEffect(() => {
    uploadRef.current = upload
  }, [upload])

  const update = useCallback((id: string, patch: Partial<PortalPendingFile>) => {
    setFiles((rows) => rows.map((row) => (row.id === id ? { ...row, ...patch } : row)))
  }, [])

  // Finished uploads start the next queued file through this ref.
  const pumpRef = useRef<() => void>(() => {})
  const pump = useCallback(() => {
    while (active.current < CONCURRENT_UPLOADS && queue.current.length > 0) {
      const id = queue.current.shift()!
      const entry = entries.current.get(id)
      if (!entry) continue
      const controller = new AbortController()
      entry.controller = controller
      active.current += 1
      update(id, { status: 'uploading', progress: 0, error: undefined })
      uploadRef.current(entry.file, {
        signal: controller.signal,
        onProgress: (percent) => {
          if (!controller.signal.aborted) update(id, { progress: Math.min(99, Math.max(0, Math.round(percent))) })
        },
      })
        .then((result) => {
          if (!controller.signal.aborted) update(id, { status: 'uploaded', progress: 100, attachmentId: result.id })
        })
        .catch((reason: unknown) => {
          if (controller.signal.aborted) return
          update(id, { status: 'error', error: reason instanceof Error ? reason.message : 'Upload failed. Please try again.' })
        })
        .finally(() => {
          if (entry.controller === controller) entry.controller = undefined
          active.current -= 1
          pumpRef.current()
        })
    }
  }, [update])
  useEffect(() => {
    pumpRef.current = pump
  }, [pump])

  const addFiles = useCallback((selected: File[]) => {
    if (selected.length === 0) return
    const maxFiles = policy?.max_files ?? DEFAULT_MAX_FILES
    const messages: string[] = []
    const rows: PortalPendingFile[] = []
    let slots = maxFiles - entries.current.size
    for (const original of selected) {
      const file = withInferredFileType(original)
      const rejection = policy ? fileRejection(file, policy) : file.size <= 0 ? `${file.name} is empty. Choose a different file.` : null
      if (rejection) {
        messages.push(rejection)
        continue
      }
      if (slots <= 0) {
        messages.push(`You can attach up to ${maxFiles} files per message.`)
        break
      }
      slots -= 1
      const id = `portal-file-${Date.now()}-${Math.random().toString(36).slice(2)}`
      const previewUrl = isPreviewableImageType(file.type) ? URL.createObjectURL(file) : undefined
      entries.current.set(id, { file, previewUrl })
      queue.current.push(id)
      rows.push({ id, name: file.name, size: file.size, type: file.type, previewUrl, status: 'queued', progress: 0 })
    }
    setNotice(messages.length > 0 ? messages.join(' ') : null)
    if (rows.length > 0) {
      setFiles((current) => [...current, ...rows])
      pump()
    }
  }, [policy, pump])

  const retry = useCallback((id: string) => {
    if (!entries.current.has(id) || queue.current.includes(id)) return
    queue.current.push(id)
    update(id, { status: 'queued', progress: 0, error: undefined })
    pump()
  }, [pump, update])

  const remove = useCallback((id: string) => {
    const entry = entries.current.get(id)
    if (!entry) return
    entry.controller?.abort()
    if (entry.previewUrl) URL.revokeObjectURL(entry.previewUrl)
    entries.current.delete(id)
    queue.current = queue.current.filter((queued) => queued !== id)
    setFiles((rows) => rows.filter((row) => row.id !== id))
    setNotice(null)
  }, [])

  const clear = useCallback(() => {
    for (const entry of entries.current.values()) {
      entry.controller?.abort()
      if (entry.previewUrl) URL.revokeObjectURL(entry.previewUrl)
    }
    entries.current.clear()
    queue.current = []
    setFiles([])
    setNotice(null)
  }, [])

  // Cancel in-flight uploads and release previews when the composer unmounts.
  useEffect(() => {
    const current = entries.current
    return () => {
      for (const entry of current.values()) {
        entry.controller?.abort()
        if (entry.previewUrl) URL.revokeObjectURL(entry.previewUrl)
      }
    }
  }, [])

  const attachmentIds = useMemo(
    () => files.filter((file) => file.status === 'uploaded' && file.attachmentId).map((file) => file.attachmentId!),
    [files],
  )
  const uploading = files.some((file) => file.status === 'queued' || file.status === 'uploading')
  const failed = files.some((file) => file.status === 'error')

  return {
    files,
    notice,
    attachmentIds,
    uploading,
    /** Sending waits for uploads and for failed files to be retried or removed. */
    blocked: uploading || failed,
    addFiles,
    retry,
    remove,
    clear,
  }
}

export type PortalAttachmentUploads = ReturnType<typeof usePortalAttachmentUploads>
