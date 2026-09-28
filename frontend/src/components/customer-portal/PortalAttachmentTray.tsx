import { useId, useRef } from 'react'
import { QuietIconAction, QuietTextAction } from '@/components/design-system/quiet'
import { AttachmentIcon, Cancel01Icon } from '@/lib/icons'
import { acceptAttributeFor, formatFileSize } from '@/lib/supportAttachmentFiles'
import type { CustomerPortalAttachmentPolicy } from '@/lib/services/customerPortalService'
import { cn } from '@/lib/utils'
import type { PortalAttachmentUploads, PortalPendingFile } from './usePortalAttachmentUploads'

function fileStatus(file: PortalPendingFile) {
  switch (file.status) {
    case 'queued': return 'Waiting to upload'
    case 'uploading': return file.progress >= 99 ? 'Finishing upload…' : `Uploading ${file.progress}%`
    case 'uploaded': return 'Ready to send'
    case 'error': return file.error ?? 'Upload failed'
  }
}

function PendingFileRow({ file, onRetry, onRemove }: { file: PortalPendingFile; onRetry: () => void; onRemove: () => void }) {
  const inProgress = file.status === 'queued' || file.status === 'uploading'
  return (
    <li className="flex min-w-0 items-center gap-2.5 py-1.5">
      {file.previewUrl ? (
        <img src={file.previewUrl} alt="" className="size-9 shrink-0 rounded-md object-cover" />
      ) : (
        <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-quiet-icon-well text-quiet-text-tertiary">
          <AttachmentIcon className="size-[15px]" aria-hidden="true" />
        </span>
      )}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[12.5px] font-medium text-quiet-text-primary">{file.name}</span>
        {/* Errors wrap so the reason is never cut off on narrow screens. */}
        <span className={cn('block text-[11.5px]', file.status === 'error' ? 'break-words text-destructive' : 'truncate text-quiet-muted')}>
          {formatFileSize(file.size)} · {fileStatus(file)}
        </span>
        {inProgress ? (
          <span className="mt-1 block h-0.5 w-full overflow-hidden rounded-full bg-quiet-divider-strong">
            <span className="block h-full bg-quiet-text-primary transition-[width]" style={{ width: `${file.status === 'queued' ? 0 : file.progress}%` }} />
          </span>
        ) : null}
      </span>
      {file.status === 'error' ? <QuietTextAction type="button" onClick={onRetry}>Retry</QuietTextAction> : null}
      <QuietIconAction type="button" aria-label={`Remove ${file.name}`} onClick={onRemove}>
        <Cancel01Icon className="size-3.5" />
      </QuietIconAction>
    </li>
  )
}

/**
 * PortalAttachmentTray lists a message's files with progress and actions, and
 * offers the keyboard path for choosing files. Drop and paste are wired by the
 * surrounding composer with usePortalFileDrop and pastedFiles (portalFileDrop.ts).
 */
export function PortalAttachmentTray({ uploads, policy, disabled }: {
  uploads: PortalAttachmentUploads
  policy?: CustomerPortalAttachmentPolicy
  disabled?: boolean
}) {
  const inputId = useId()
  const input = useRef<HTMLInputElement>(null)
  return (
    <div className="min-w-0 flex-1">
      <input
        ref={input}
        id={inputId}
        type="file"
        multiple
        accept={policy ? acceptAttributeFor(policy.content_types) : undefined}
        aria-label="Attach files"
        className="sr-only"
        disabled={disabled}
        onChange={(event) => {
          uploads.addFiles(Array.from(event.target.files ?? []))
          event.target.value = ''
        }}
      />
      {uploads.files.length > 0 ? (
        <ul aria-label="Attached files" className="mb-1.5">
          {uploads.files.map((file) => (
            <PendingFileRow key={file.id} file={file} onRetry={() => uploads.retry(file.id)} onRemove={() => uploads.remove(file.id)} />
          ))}
        </ul>
      ) : null}
      <QuietTextAction type="button" disabled={disabled} onClick={() => input.current?.click()} className="gap-1.5">
        <AttachmentIcon className="size-[15px]" aria-hidden="true" />
        Attach files
      </QuietTextAction>
      <span role="status" className="sr-only">
        {uploads.uploading ? 'Uploading attachments…' : uploads.files.length > 0 ? `${uploads.attachmentIds.length} of ${uploads.files.length} files ready` : ''}
      </span>
      {uploads.notice ? <p role="alert" className="mt-1.5 text-[12.5px] text-destructive">{uploads.notice}</p> : null}
    </div>
  )
}
