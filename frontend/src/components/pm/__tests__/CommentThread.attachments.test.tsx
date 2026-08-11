// @vitest-environment jsdom

import React, { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { AttachmentResponse, CommentWithAuthor, CreateCommentRequest } from '@/lib/pmTypes'

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

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

vi.mock('@/components/pm/ImageLightbox', () => ({
  ImageLightbox: ({
    src,
    alt,
    hasPrevious,
    hasNext,
    onPrevious,
    onNext,
    positionLabel,
  }: {
    src: string
    alt?: string
    hasPrevious?: boolean
    hasNext?: boolean
    onPrevious?: () => void
    onNext?: () => void
    positionLabel?: string
  }) => (
    <div data-testid="image-lightbox" data-src={src} data-alt={alt} data-position={positionLabel}>
      {hasPrevious ? (
        <button type="button" data-testid="lightbox-previous" onClick={onPrevious}>
          Previous
        </button>
      ) : null}
      {hasNext ? (
        <button type="button" data-testid="lightbox-next" onClick={onNext}>
          Next
        </button>
      ) : null}
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

vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuItem: ({
    children,
    onSelect,
  }: {
    children: React.ReactNode
    onSelect?: () => void
  }) => (
    <button type="button" onClick={() => onSelect?.()}>
      {children}
    </button>
  ),
  DropdownMenuSeparator: () => null,
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

function renderThread({
  comments = [],
  commentService = createCommentService(),
  hideEmptyState = false,
}: {
  comments?: CommentWithAuthor[]
  commentService?: ReturnType<typeof createCommentService>
  hideEmptyState?: boolean
} = {}) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)

  act(() => {
    root.render(
      <CommentThread
        workspaceId={workspaceId}
        entityType="task"
        entityId="task-1"
        comments={comments}
        currentUserId="user-1"
        commentService={commentService}
        hideEmptyState={hideEmptyState}
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

async function pasteImageInto(scope: HTMLElement) {
  const button = scope.querySelector<HTMLButtonElement>('[data-testid="paste-image"]')
  if (!button) throw new Error('paste button not found in scope')
  await act(async () => {
    button.click()
  })
}

async function clickEdit(container: HTMLElement) {
  const editButton = Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find(
    (b) => b.textContent?.trim() === 'Edit',
  )
  if (!editButton) throw new Error('Edit menu item not found')
  await act(async () => {
    editButton.click()
  })
}

function existingCommentByCurrentUser(id: string): CommentWithAuthor {
  return {
    comment: {
      id,
      entity_type: 'task',
      entity_id: 'task-1',
      author_id: 'user-1',
      body: '<p>existing</p>',
      parent_id: null,
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

function existingCommentFromUser(id: string, userId: string, body: string): CommentWithAuthor {
  return {
    comment: {
      id,
      entity_type: 'task',
      entity_id: 'task-1',
      author_id: userId,
      body,
      parent_id: null,
      created_at: '2026-05-03T00:00:00Z',
      updated_at: '2026-05-03T00:00:00Z',
    },
    author: {
      id: userId,
      email: `${userId}@example.com`,
      full_name: userId === 'user-1' ? 'Test User' : 'Reply User',
      created_at: '2026-05-03T00:00:00Z',
      updated_at: '2026-05-03T00:00:00Z',
    },
    reply_count: 0,
    attachments: [],
    reactions: [],
  }
}

function createAttachment(id: string, fileName: string, contentType = 'image/png'): AttachmentResponse {
  return {
    attachment: {
      id,
      workspace_id: workspaceId,
      entity_type: 'comment',
      entity_id: 'comment-1',
      file_name: fileName,
      file_size: 1024,
      content_type: contentType,
      storage_key: `attachments/${id}`,
      is_uploaded: true,
      uploaded_by_id: 'user-1',
      created_at: '2026-05-03T00:00:00Z',
    },
    url: `https://cdn.example.com/${fileName}`,
    public_url: `https://cdn.example.com/${fileName}`,
  }
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

    const composer = container.querySelector<HTMLElement>('[data-testid="comment-editor"]')!
    expect(composer.querySelector('[data-testid="uploaded-files"]')?.textContent).toBe('att-1,att-2')
    expect(pmAttachmentService.remove).not.toHaveBeenCalled()

    await submitComment(container)

    expect(commentService.create).toHaveBeenCalledWith(
      workspaceId,
      expect.objectContaining({ attachment_ids: ['att-1', 'att-2'] }),
    )
    expect(pmAttachmentService.remove).not.toHaveBeenCalled()
  })

  it('keeps compact top spacing above the empty top-level composer when the empty state is hidden', () => {
    const { container } = renderThread({ comments: [], hideEmptyState: true })

    const composer = container.querySelector<HTMLElement>('[data-testid="comment-editor"]')
    const composerFrame = composer?.parentElement

    expect(composerFrame?.className).toContain('mt-1')
  })

  it('extends the collapse stem up to the parent comment avatar', () => {
    const parent = existingCommentByCurrentUser('parent-1')
    parent.comment.body = '<p>line one</p><p>line two</p><p>line three</p><p>line four</p>'
    const reply = existingCommentFromUser('reply-1', 'user-2', '<p>reply</p>')
    reply.comment.parent_id = parent.comment.id
    parent.replies = [reply]
    parent.reply_count = 1

    const { container } = renderThread({ comments: [parent] })

    const topStem = container.querySelector<HTMLElement>('[data-comment-collapse-stem="top"]')

    expect(topStem?.className).toContain('top-6')
    expect(topStem?.className).toContain('bottom-[-10px]')
    expect(topStem?.className).toContain('bg-border')
    expect(topStem?.className).not.toContain('h-[')
    expect(topStem?.className).not.toContain('bg-border/60')
  })

  it('makes reply connector rails height-independent without drawing below the last reply avatar', () => {
    const parent = existingCommentByCurrentUser('parent-1')
    const firstReply = existingCommentFromUser('reply-1', 'user-2', '<p>first reply line one</p><p>first reply line two</p><p>first reply line three</p>')
    const secondReply = existingCommentFromUser('reply-2', 'user-2', '<p>second reply</p>')
    firstReply.comment.parent_id = parent.comment.id
    secondReply.comment.parent_id = parent.comment.id
    parent.replies = [firstReply, secondReply]
    parent.reply_count = 2

    const { container } = renderThread({ comments: [parent] })

    const topStem = container.querySelector<HTMLElement>('[data-comment-collapse-stem="top"]')
    const elbow = container.querySelector<HTMLElement>('[data-comment-collapse-stem="elbow"]')
    const repliesRail = Array.from(container.querySelectorAll<HTMLElement>('[data-comment-replies-rail="true"]'))

    expect(topStem?.className).toContain('bg-border')
    expect(elbow?.className).toContain('bg-border')
    expect(repliesRail).toHaveLength(2)
    expect(repliesRail[0].className).toContain('bg-border')
    expect(repliesRail[0].className).toContain('top-3.5')
    expect(repliesRail[0].className).toContain('bottom-[-12px]')
    expect(repliesRail[1].className).toContain('bg-border')
    expect(repliesRail[1].className).toContain('top-0')
    expect(repliesRail[1].className).toContain('h-3.5')
    expect(repliesRail[1].className).not.toContain('bottom-')
  })

  it('navigates between multiple image attachments in a comment preview', async () => {
    const comment = existingCommentByCurrentUser('comment-1')
    comment.attachments = [
      createAttachment('att-1', 'image-1.png'),
      createAttachment('att-2', 'image-2.png'),
      createAttachment('att-3', 'image-3.png'),
      createAttachment('att-4', 'image-4.png'),
    ]
    const { container } = renderThread({ comments: [comment] })

    const secondImage = container.querySelector<HTMLImageElement>('img[alt="image-2.png"]')
    const trigger = secondImage?.closest('button')
    expect(trigger).toBeTruthy()

    await act(async () => {
      trigger?.click()
    })

    let lightbox = container.querySelector<HTMLElement>('[data-testid="image-lightbox"]')
    expect(lightbox?.dataset.src).toBe('https://cdn.example.com/image-2.png')
    expect(lightbox?.dataset.position).toBe('2 / 4')
    expect(container.querySelector('[data-testid="lightbox-previous"]')).toBeTruthy()
    expect(container.querySelector('[data-testid="lightbox-next"]')).toBeTruthy()

    await act(async () => {
      container.querySelector<HTMLButtonElement>('[data-testid="lightbox-next"]')?.click()
    })

    lightbox = container.querySelector<HTMLElement>('[data-testid="image-lightbox"]')
    expect(lightbox?.dataset.src).toBe('https://cdn.example.com/image-3.png')
    expect(lightbox?.dataset.position).toBe('3 / 4')
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

  it('does not pending-delete submitted attachments when unmounted before comment create resolves', async () => {
    let uploadIndex = 0
    vi.mocked(pmAttachmentService.initiateUpload).mockImplementation((_ws, payload) => {
      uploadIndex += 1
      return Promise.resolve({
        data: {
          attachment: {
            id: `att-submit-${uploadIndex}`,
            workspace_id: workspaceId,
            entity_type: payload.entity_type,
            entity_id: payload.entity_id,
            file_name: payload.file_name,
            file_size: payload.file_size,
            content_type: payload.content_type,
            storage_key: `attachments/att-submit-${uploadIndex}`,
            is_uploaded: false,
            uploaded_by_id: 'user-1',
            created_at: '2026-05-03T00:00:00Z',
          },
          url: `https://upload.example.com/att-submit-${uploadIndex}`,
          public_url: `https://cdn.example.com/att-submit-${uploadIndex}.png`,
        },
        error: null,
        status: 200,
      })
    })
    vi.mocked(uploadToS3).mockResolvedValue({ ok: true, error: null })
    vi.mocked(pmAttachmentService.confirmUpload).mockResolvedValue({ data: null, error: null, status: 200 })

    let resolveCreate: (value: Awaited<ReturnType<ReturnType<typeof createCommentService>['create']>>) => void
    const commentService = createCommentService()
    commentService.create.mockImplementation((_workspaceId: string, payload: CreateCommentRequest) =>
      new Promise((resolve) => {
        resolveCreate = resolve
      }).then(() => ({
        data: createComment('comment-submit', payload),
        error: null,
        status: 200,
      })),
    )

    const { container, root } = renderThread({ commentService })

    await pasteImage(container)
    await pasteImage(container)
    await submitComment(container)

    act(() => {
      root.unmount()
    })

    expect(commentService.create).toHaveBeenCalledWith(
      workspaceId,
      expect.objectContaining({ attachment_ids: ['att-submit-1', 'att-submit-2'] }),
    )
    expect(pmAttachmentService.remove).not.toHaveBeenCalled()

    await act(async () => {
      resolveCreate!({
        data: createComment('comment-submit', {
          entity_type: 'task',
          entity_id: 'task-1',
          body: '<p>comment</p>',
          attachment_ids: ['att-submit-1', 'att-submit-2'],
        }),
        error: null,
        status: 200,
      })
    })
  })

  it('cleans up edit-mode pending attachments on unmount', async () => {
    vi.mocked(pmAttachmentService.initiateUpload).mockResolvedValue({
      data: {
        attachment: {
          id: 'att-edit-pending',
          workspace_id: workspaceId,
          entity_type: 'editor_upload',
          entity_id: workspaceId,
          file_name: 'clipboard-1.png',
          file_size: 11,
          content_type: 'image/png',
          storage_key: 'attachments/att-edit-pending',
          is_uploaded: false,
          uploaded_by_id: 'user-1',
          created_at: '2026-05-03T00:00:00Z',
        },
        url: 'https://upload.example.com/att-edit-pending',
        public_url: 'https://cdn.example.com/att-edit-pending.png',
      },
      error: null,
      status: 200,
    })
    vi.mocked(uploadToS3).mockResolvedValue({ ok: true, error: null })
    vi.mocked(pmAttachmentService.confirmUpload).mockResolvedValue({ data: null, error: null, status: 200 })

    const { container, root } = renderThread({
      comments: [existingCommentByCurrentUser('comment-1')],
    })

    await clickEdit(container)

    // With existing comments, the top-level composer stays collapsed; the visible
    // editor is the edit form for the selected comment.
    const editors = container.querySelectorAll<HTMLElement>('[data-testid="comment-editor"]')
    expect(editors.length).toBe(1)
    await pasteImageInto(editors[0])

    expect(pmAttachmentService.remove).not.toHaveBeenCalled()

    act(() => {
      root.unmount()
    })

    expect(pmAttachmentService.remove).toHaveBeenCalledWith(workspaceId, 'att-edit-pending', {
      pendingOnly: true,
    })
  })

  it('shows existing replies expanded without opening a reply editor', async () => {
    const parent = existingCommentFromUser('comment-parent', 'user-1', '<p>Parent comment</p>')
    const reply = existingCommentFromUser('comment-reply', 'user-2', '<p>Visible reply</p>')
    reply.comment.parent_id = parent.comment.id
    parent.replies = [reply]
    parent.reply_count = 1

    const { container } = renderThread({ comments: [parent] })

    expect(container.textContent).toContain('Visible reply')
    expect(container.querySelector('[data-testid="comment-editor"]')).toBeNull()
    expect(container.querySelector('[aria-label="Reply in thread"]')).toBeTruthy()

    const collapseButton = container.querySelector<HTMLButtonElement>('[aria-label="Collapse replies"]')
    expect(collapseButton).toBeTruthy()
    expect(container.querySelector('[data-comment-collapse-stem="top"]')).toBeTruthy()
    await act(async () => {
      collapseButton?.click()
    })

    expect(container.textContent).not.toContain('Visible reply')
    expect(container.textContent).toContain('1 reply')
    expect(container.querySelector('[data-comment-collapse-stem="top"]')).toBeNull()

    const replyButton = container.querySelector<HTMLButtonElement>('[aria-label="Reply"]')
    expect(replyButton).toBeTruthy()
    await act(async () => {
      replyButton?.click()
    })

    expect(container.textContent).toContain('Visible reply')
    expect(container.querySelector('[data-testid="comment-editor"]')).toBeTruthy()
  })

  it('opens the parent thread reply editor from a nested reply action', async () => {
    const parent = existingCommentFromUser('comment-parent', 'user-1', '<p>Parent comment</p>')
    const reply = existingCommentFromUser('comment-reply', 'user-2', '<p>Visible reply</p>')
    reply.comment.parent_id = parent.comment.id
    parent.replies = [reply]
    parent.reply_count = 1

    const { container } = renderThread({ comments: [parent] })

    expect(container.querySelector('[data-testid="comment-editor"]')).toBeNull()

    const replyInThreadButton = container.querySelector<HTMLButtonElement>('[aria-label="Reply in thread"]')
    expect(replyInThreadButton).toBeTruthy()
    await act(async () => {
      replyInThreadButton?.click()
    })

    expect(container.querySelector('[data-testid="comment-editor"]')).toBeTruthy()
  })

  it('does not show thread collapse chrome when only a reply editor is open', async () => {
    const parent = existingCommentFromUser('comment-parent', 'user-1', '<p>Parent comment</p>')

    const { container } = renderThread({ comments: [parent] })

    const replyButton = container.querySelector<HTMLButtonElement>('[aria-label="Reply"]')
    expect(replyButton).toBeTruthy()
    await act(async () => {
      replyButton?.click()
    })

    expect(container.querySelector('[data-testid="comment-editor"]')).toBeTruthy()
    expect(container.querySelector('[aria-label="Collapse replies"]')).toBeNull()
  })

  it('removes thread collapse chrome after the last visible reply is deleted', async () => {
    const parent = existingCommentFromUser('comment-parent', 'user-1', '<p>Parent comment</p>')
    const reply = existingCommentFromUser('comment-reply', 'user-1', '<p>Visible reply</p>')
    reply.comment.parent_id = parent.comment.id
    parent.replies = [reply]
    parent.reply_count = 1
    const onCommentsChange = vi.fn()
    const commentService = createCommentService()
    commentService.remove.mockResolvedValue({ data: null, error: null, status: 200 })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(
        <CommentThread
          workspaceId={workspaceId}
          entityType="task"
          entityId="task-1"
          comments={[parent]}
          currentUserId="user-1"
          commentService={commentService}
          onCommentsChange={onCommentsChange}
        />,
      )
    })

    expect(container.querySelector('[aria-label="Collapse replies"]')).toBeTruthy()

    const deleteButton = Array.from(container.querySelectorAll<HTMLButtonElement>('button')).findLast(
      (button) => button.textContent?.trim() === 'Delete',
    )
    expect(deleteButton).toBeTruthy()
    await act(async () => {
      deleteButton?.click()
    })

    const nextComments = onCommentsChange.mock.calls.at(-1)?.[0] as CommentWithAuthor[]
    expect(nextComments[0].reply_count).toBe(0)

    await act(async () => {
      root.render(
        <CommentThread
          workspaceId={workspaceId}
          entityType="task"
          entityId="task-1"
          comments={nextComments}
          currentUserId="user-1"
          commentService={commentService}
          onCommentsChange={onCommentsChange}
        />,
      )
    })

    expect(container.querySelector('[aria-label="Collapse replies"]')).toBeNull()
    act(() => root.unmount())
  })

  it('identifies comments created through a named AI agent', () => {
    const comment = existingCommentByCurrentUser('comment-agent')
    comment.comment.agent_id = 'agent-1'
    comment.comment.agent_name = 'Code Review Agent'
    comment.comment.agent_run_id = 'run-1'

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => {
      root.render(
        <CommentThread
          workspaceId={workspaceId}
          entityType="task"
          entityId="task-1"
          comments={[comment]}
          currentUserId="user-1"
          commentService={createCommentService()}
        />,
      )
    })

    expect(container.textContent).toContain('Test User')
    expect(container.textContent).toContain('(via Code Review Agent)')
    expect(container.querySelector('[aria-label="AI agent comment"]')).toBeTruthy()
    act(() => root.unmount())
  })

  it('uses the generic AI Agent label when the agent name is unavailable', () => {
    const comment = existingCommentByCurrentUser('comment-agent-fallback')
    comment.comment.agent_id = 'deleted-agent'

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    act(() => {
      root.render(
        <CommentThread
          workspaceId={workspaceId}
          entityType="task"
          entityId="task-1"
          comments={[comment]}
          currentUserId="user-1"
          commentService={createCommentService()}
        />,
      )
    })

    expect(container.textContent).toContain('(via AI Agent)')
    act(() => root.unmount())
  })
})
