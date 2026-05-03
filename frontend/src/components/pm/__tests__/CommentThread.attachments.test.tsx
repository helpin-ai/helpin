// @vitest-environment jsdom

import React, { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { CommentWithAuthor, CreateCommentRequest } from '@/lib/pmTypes'

vi.mock('@/components/pm/CommentEditor', () => ({
  CommentEditor: ({
    onImageSelect,
    onSubmit,
    uploadedFiles = [],
  }: {
    onImageSelect?: (files: File[]) => void
    onSubmit: (text: string) => void | Promise<void>
    uploadedFiles?: { id: string; name: string; url?: string }[]
  }) => (
    <div data-testid="comment-editor">
      <button
        type="button"
        data-testid="paste-image"
        onClick={() =>
          onImageSelect?.([
            new File(['image-bytes'], `clipboard-${uploadedFiles.length + 1}.png`, {
              type: 'image/png',
            }),
          ])
        }
      >
        Paste image
      </button>
      <button type="button" data-testid="submit-comment" onClick={() => void onSubmit('<p>comment</p>')}>
        Submit
      </button>
      <div data-testid="uploaded-files">{uploadedFiles.map((file) => file.id).join(',')}</div>
    </div>
  ),
}))

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('@/components/ui/popover', () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('@/lib/api', () => ({
  uploadToS3: vi.fn(),
}))

vi.mock('@/lib/services/pmAttachmentService', () => ({
  pmAttachmentService: {
    initiateUpload: vi.fn(),
    confirmUpload: vi.fn(),
    remove: vi.fn(),
  },
}))

const { CommentThread } = await import('../CommentThread')
const { uploadToS3 } = await import('@/lib/api')
const { pmAttachmentService } = await import('@/lib/services/pmAttachmentService')

const workspaceId = 'ws-1'

function renderThread(commentService = createCommentService()) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <CommentThread
        workspaceId={workspaceId}
        entityType="task"
        entityId="task-1"
        comments={[]}
        currentUserId="user-1"
        commentService={commentService}
        onCommentsChange={vi.fn()}
      />,
    )
  })

  return { container, root, commentService }
}

function createCommentService() {
  return {
    list: vi.fn().mockResolvedValue({ data: [], error: null, status: 200 }),
    create: vi.fn().mockImplementation((_workspaceId: string, payload: CreateCommentRequest) =>
      Promise.resolve({
        data: createComment('comment-1', payload),
        error: null,
        status: 200,
      }),
    ),
    update: vi.fn(),
    remove: vi.fn(),
    resolve: vi.fn(),
    reopen: vi.fn(),
    toggleReaction: vi.fn(),
  }
}

function createComment(id: string, payload: CreateCommentRequest): CommentWithAuthor {
  return {
    comment: {
      id,
      entity_type: payload.entity_type,
      entity_id: payload.entity_id,
      author_id: 'user-1',
      body: payload.body,
      parent_id: payload.parent_id,
      created_at: '2026-05-03T00:00:00Z',
      updated_at: '2026-05-03T00:00:00Z',
    },
    author: {
      id: 'user-1',
      email: 'user@example.com',
      full_name: 'Test User',
      created_at: '2026-05-03T00:00:00Z',
      updated_at: '2026-05-03T00:00:00Z',
    },
    reply_count: 0,
    attachments: [],
    reactions: [],
  }
}

async function pasteImage(container: HTMLElement) {
  const button = container.querySelector<HTMLButtonElement>('[data-testid="paste-image"]')
  if (!button) throw new Error('paste button not found')
  await act(async () => {
    button.click()
  })
}

async function submitComment(container: HTMLElement) {
  const button = container.querySelector<HTMLButtonElement>('[data-testid="submit-comment"]')
  if (!button) throw new Error('submit button not found')
  await act(async () => {
    button.click()
  })
}

describe('CommentThread attachment uploads', () => {
  afterEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('keeps earlier pasted images when a second image is uploaded before submit', async () => {
    let uploadIndex = 0
    vi.mocked(pmAttachmentService.initiateUpload).mockImplementation((_ws, payload) => {
      uploadIndex += 1
      return Promise.resolve({
        data: {
          attachment: {
            id: `att-${uploadIndex}`,
            workspace_id: workspaceId,
            entity_type: payload.entity_type,
            entity_id: payload.entity_id,
            file_name: payload.file_name,
            file_size: payload.file_size,
            content_type: payload.content_type,
            storage_key: `attachments/att-${uploadIndex}`,
            is_uploaded: false,
            uploaded_by_id: 'user-1',
            created_at: '2026-05-03T00:00:00Z',
          },
          url: `https://upload.example.com/att-${uploadIndex}`,
          public_url: `https://cdn.example.com/att-${uploadIndex}.png`,
        },
        error: null,
        status: 200,
      })
    })
    vi.mocked(uploadToS3).mockResolvedValue({ ok: true, error: null })
    vi.mocked(pmAttachmentService.confirmUpload).mockResolvedValue({ data: null, error: null, status: 200 })

    const { container, commentService } = renderThread()

    await pasteImage(container)
    await pasteImage(container)

    expect(container.querySelector('[data-testid="uploaded-files"]')?.textContent).toBe('att-1,att-2')
    expect(pmAttachmentService.remove).not.toHaveBeenCalled()

    await submitComment(container)

    expect(commentService.create).toHaveBeenCalledWith(
      workspaceId,
      expect.objectContaining({ attachment_ids: ['att-1', 'att-2'] }),
    )
    expect(pmAttachmentService.remove).not.toHaveBeenCalled()
  })

  it('cleans up only the still-pending pasted images on unmount', async () => {
    vi.mocked(pmAttachmentService.initiateUpload).mockResolvedValue({
      data: {
        attachment: {
          id: 'att-pending',
          workspace_id: workspaceId,
          entity_type: 'editor_upload',
          entity_id: workspaceId,
          file_name: 'clipboard-1.png',
          file_size: 11,
          content_type: 'image/png',
          storage_key: 'attachments/att-pending',
          is_uploaded: false,
          uploaded_by_id: 'user-1',
          created_at: '2026-05-03T00:00:00Z',
        },
        url: 'https://upload.example.com/att-pending',
        public_url: 'https://cdn.example.com/att-pending.png',
      },
      error: null,
      status: 200,
    })
    vi.mocked(uploadToS3).mockResolvedValue({ ok: true, error: null })
    vi.mocked(pmAttachmentService.confirmUpload).mockResolvedValue({ data: null, error: null, status: 200 })

    const { container, root } = renderThread()

    await pasteImage(container)
    expect(pmAttachmentService.remove).not.toHaveBeenCalled()

    act(() => {
      root.unmount()
    })

    expect(pmAttachmentService.remove).toHaveBeenCalledWith(workspaceId, 'att-pending', { pendingOnly: true })
  })
})
