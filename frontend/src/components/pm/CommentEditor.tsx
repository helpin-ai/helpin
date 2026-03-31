import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { MentionHighlight } from '@/components/pm/mention-highlight'
import { diffRemovedInlineAttachmentIds, extractInlineAttachmentIds } from '@/components/pm/editorImageAttachments'
import { ResizableImageExtension } from '@/components/ui/resizable-image-extension'
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList'
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload'
import { pmAttachmentService } from '@/lib/services/pmAttachmentService'
import { useRef, useState, useCallback, useEffect, useMemo } from 'react'
import { Loader2, Send, ImageIcon, Paperclip, X } from 'lucide-react'
import { toast } from 'sonner'
import type { WorkspaceTeam, AssignableMember } from '@/lib/types'
import {
  getMentionSuggestions,
  type MentionSuggestionItem,
} from '@/components/pm/mentionSuggestions'

interface CommentEditorProps {
  onSubmit: (text: string) => void | Promise<void>
  loading?: boolean
  placeholder?: string
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  members?: AssignableMember[]
  uploadConfig?: EditorUploadConfig
  onUploadStateChange?: (pendingUploads: number) => void
  onFileSelect?: () => void
  uploadedFiles?: { id: string; name: string }[]
  onRemoveUploadedFile?: (id: string) => void
}

function getFileExtension(filename: string): string {
  const parts = filename.split('.');
  return parts.length > 1 ? parts.pop()!.toLowerCase() : '';
}

function detectMentions(
  editorInstance: ReturnType<typeof useEditor>,
  teams: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[],
  members: AssignableMember[],
): { from: number; to: number; items: MentionSuggestionItem[]; selectedIndex: number } | null {
  if (!editorInstance) return null
  if (teams.length === 0 && members.length === 0) return null

  const { selection } = editorInstance.state
  if (!selection.empty) return null

  const textBefore = selection.$from.parent.textBetween(
    0,
    selection.$from.parentOffset,
    undefined,
    '\ufffc',
  )
  const match = textBefore.match(/(?:^|\s)@([a-z0-9._-]*)$/i)
  if (!match) return null

  const query = match[1].toLowerCase()
  const items = getMentionSuggestions(query, members, teams, 8)

  if (items.length === 0) return null

  return {
    from: selection.from - (query.length + 1),
    to: selection.from,
    items: items.slice(0, 8),
    selectedIndex: 0,
  }
}

export function CommentEditor({
  onSubmit,
  loading = false,
  placeholder = 'Leave a comment...',
  teams = [],
  members = [],
  uploadConfig,
  onUploadStateChange,
  onFileSelect,
  uploadedFiles = [],
  onRemoveUploadedFile,
}: CommentEditorProps) {
  const [mentionState, setMentionState] = useState<{
    from: number
    to: number
    items: MentionSuggestionItem[]
    selectedIndex: number
  } | null>(null)
  const mentionStateRef = useRef(mentionState)
  mentionStateRef.current = mentionState
  const teamsRef = useRef(teams)
  teamsRef.current = teams
  const membersRef = useRef(members)
  membersRef.current = members
  const uploadConfigRef = useRef(uploadConfig)
  uploadConfigRef.current = uploadConfig
  const onUploadStateChangeRef = useRef(onUploadStateChange)
  onUploadStateChangeRef.current = onUploadStateChange
  const pendingUploadsRef = useRef(0)
  const [pendingUploads, setPendingUploads] = useState(0)
  const [hasContent, setHasContent] = useState(false)
  const currentHtmlRef = useRef('')
  const skipNextCleanupRef = useRef(false)

  const getContent = useCallback((editor: ReturnType<typeof useEditor>) => {
    if (!editor) return ''
    // If the editor has images, return HTML; otherwise return plain text for backward compat
    const html = editor.getHTML()
    const hasImages = html.includes('<img ')
    if (hasImages) return html
    return editor.getText()
  }, [])

  const handleImageUpload = useCallback(
    async (file: File, editorInstance: ReturnType<typeof useEditor>) => {
      if (!editorInstance || !uploadConfigRef.current) return
      if (!file.type.startsWith('image/')) return

      const uploadId = `img-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

      // Read file as data URI for instant preview
      const dataUri = await new Promise<string>((resolve) => {
        const reader = new FileReader()
        reader.onload = () => resolve(reader.result as string)
        reader.readAsDataURL(file)
      })

      editorInstance.chain().focus().setResizableImage({ src: dataUri, alt: file.name, title: uploadId }).run()

      pendingUploadsRef.current += 1
      setPendingUploads(pendingUploadsRef.current)
      onUploadStateChangeRef.current?.(pendingUploadsRef.current)
      try {
        const upload = await uploadEditorImage(file, uploadConfigRef.current!)

        const { doc } = editorInstance.state
        let targetPos: number | null = null
        doc.descendants((node, pos) => {
          if (node.type.name === 'resizableImage' && node.attrs.title === uploadId) {
            targetPos = pos
            return false
          }
        })

        if (targetPos !== null) {
          const node = doc.nodeAt(targetPos)
          if (node) {
            editorInstance.view.dispatch(
              editorInstance.state.tr.setNodeMarkup(targetPos, undefined, {
                ...node.attrs,
                src: upload.publicUrl,
                title: null,
                attachmentId: upload.attachmentId,
              }),
            )
          }
        }
      } catch {
        // On failure, remove the placeholder image
        const { doc } = editorInstance.state
        let targetPos: number | null = null
        doc.descendants((node, pos) => {
          if (node.type.name === 'resizableImage' && node.attrs.title === uploadId) {
            targetPos = pos
            return false
          }
        })

        if (targetPos !== null) {
          const node = doc.nodeAt(targetPos)
          if (node) {
            editorInstance.view.dispatch(
              editorInstance.state.tr.delete(targetPos, targetPos + node.nodeSize),
            )
          }
        }
        toast.error('Failed to upload image')
      } finally {
        pendingUploadsRef.current = Math.max(0, pendingUploadsRef.current - 1)
        setPendingUploads(pendingUploadsRef.current)
        onUploadStateChangeRef.current?.(pendingUploadsRef.current)
      }
    },
    [],
  )

  const cleanupDraftAttachments = useCallback(async (attachmentIds: string[]) => {
    const currentConfig = uploadConfigRef.current
    if (!currentConfig || currentConfig.entityType !== 'editor_upload' || attachmentIds.length === 0) {
      return
    }
    await Promise.allSettled(
      attachmentIds.map((attachmentId) => pmAttachmentService.remove(currentConfig.workspaceId, attachmentId)),
    )
  }, [])

  const handleSubmit = useCallback(async () => {
    if (!editorRef.current || loading) return
    const text = getContent(editorRef.current)
    if (!text.trim() && uploadedFiles.length === 0) return
    try {
      await onSubmit(text.trim())
    } catch {
      return
    }
    skipNextCleanupRef.current = true
    editorRef.current.commands.clearContent()
  }, [onSubmit, loading, getContent, uploadedFiles.length])

  const handleSubmitRef = useRef(handleSubmit)
  handleSubmitRef.current = handleSubmit

  const hasMentionables = teams.length > 0 || members.length > 0

  const extensions = useMemo(() => {
    const exts = [
      StarterKit.configure({
        heading: false,
        blockquote: false,
        codeBlock: false,
        horizontalRule: false,
        bulletList: false,
        orderedList: false,
        listItem: false,
      }),
      Placeholder.configure({ placeholder }),
      MentionHighlight,
    ]
    if (uploadConfig) {
      exts.push(ResizableImageExtension as typeof exts[number])
    }
    return exts
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [placeholder, !!uploadConfig])

  const editor = useEditor({
    extensions,
    onUpdate: ({ editor: e }) => {
      setHasContent(!e.isEmpty)
    },
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[40px] max-h-[120px] overflow-y-auto px-3 py-2 text-sm',
      },
      handlePaste: (_view, event) => {
        if (!uploadConfigRef.current) return false
        const items = event.clipboardData?.items
        if (!items) return false
        for (const item of items) {
          if (item.type.startsWith('image/')) {
            event.preventDefault()
            const file = item.getAsFile()
            if (file) handleImageUpload(file, editorRef.current)
            return true
          }
        }
        return false
      },
      handleDrop: (_view, event, _slice, moved) => {
        if (!uploadConfigRef.current || moved) return false
        const files = event.dataTransfer?.files
        if (!files?.length) return false
        for (const file of files) {
          if (file.type.startsWith('image/')) {
            event.preventDefault()
            handleImageUpload(file, editorRef.current)
            return true
          }
        }
        return false
      },
      handleKeyDown: (_view, event) => {
        const currentMention = mentionStateRef.current
        if (currentMention && currentMention.items.length > 0) {
          if (event.key === 'ArrowDown') {
            event.preventDefault()
            setMentionState({
              ...currentMention,
              selectedIndex: (currentMention.selectedIndex + 1) % currentMention.items.length,
            })
            return true
          }
          if (event.key === 'ArrowUp') {
            event.preventDefault()
            setMentionState({
              ...currentMention,
              selectedIndex:
                (currentMention.selectedIndex - 1 + currentMention.items.length) %
                currentMention.items.length,
            })
            return true
          }
          if (event.key === 'Enter' || event.key === 'Tab') {
            const selected = currentMention.items[currentMention.selectedIndex]
            if (!selected || !editorRef.current) return false
            event.preventDefault()
            editorRef.current
              .chain()
              .focus()
              .insertContentAt(
                { from: currentMention.from, to: currentMention.to },
                `@${selected.handle} `,
              )
              .run()
            setMentionState(null)
            return true
          }
          if (event.key === 'Escape') {
            event.preventDefault()
            setMentionState(null)
            return true
          }
        }

        // Ctrl/Cmd+Enter to submit
        if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
          event.preventDefault()
          handleSubmitRef.current()
          return true
        }

        return false
      },
    },
    onUpdate: () => {
      const html = editorRef.current?.getHTML() ?? ''
      if (skipNextCleanupRef.current) {
        skipNextCleanupRef.current = false
        currentHtmlRef.current = html
        setMentionState(detectMentions(editorRef.current, teamsRef.current, membersRef.current))
        return
      }
      const removedDraftAttachmentIds = diffRemovedInlineAttachmentIds(currentHtmlRef.current, html)
      currentHtmlRef.current = html
      if (removedDraftAttachmentIds.length > 0) {
        void cleanupDraftAttachments(removedDraftAttachmentIds)
      }
      setMentionState(detectMentions(editorRef.current, teamsRef.current, membersRef.current))
    },
    onBlur: () => setMentionState(null),
  })

  const editorRef = useRef(editor)
  editorRef.current = editor

  // Also register via editor.on() as backup — TipTap v3 may not call
  // the onUpdate option reliably in all cases.
  useEffect(() => {
    if (!editor) return

    const handleUpdate = () => {
      const html = editor.getHTML()
      if (skipNextCleanupRef.current) {
        skipNextCleanupRef.current = false
        currentHtmlRef.current = html
        setMentionState(detectMentions(editor, teamsRef.current, membersRef.current))
        return
      }
      const removedDraftAttachmentIds = diffRemovedInlineAttachmentIds(currentHtmlRef.current, html)
      currentHtmlRef.current = html
      if (removedDraftAttachmentIds.length > 0) {
        void cleanupDraftAttachments(removedDraftAttachmentIds)
      }
      setMentionState(detectMentions(editor, teamsRef.current, membersRef.current))
    }
    const handleBlur = () => setMentionState(null)

    editor.on('update', handleUpdate)
    editor.on('blur', handleBlur)

    return () => {
      editor.off('update', handleUpdate)
      editor.off('blur', handleBlur)
    }
  }, [cleanupDraftAttachments, editor])

  useEffect(
    () => () => {
      const currentConfig = uploadConfigRef.current
      if (!currentConfig || currentConfig.entityType !== 'editor_upload') {
        return
      }
      const draftAttachmentIds = extractInlineAttachmentIds(currentHtmlRef.current)
      if (draftAttachmentIds.length > 0) {
        void cleanupDraftAttachments(draftAttachmentIds)
      }
    },
    [cleanupDraftAttachments],
  )

  if (!editor) return null

  const canSubmit = !loading && pendingUploads === 0 && (hasContent || uploadedFiles.length > 0)

  return (
    <div className="rounded-lg border border-border/60 bg-background transition-colors focus-within:border-border">
      <EditorContent editor={editor} />
      {mentionState && mentionState.items.length > 0 ? (
        <div className="border-t border-border/60 bg-muted/40 px-2 py-2">
          <MentionSuggestionsList
            items={mentionState.items}
            selectedIndex={mentionState.selectedIndex}
            onSelect={(item) => {
              if (!editorRef.current) return
              editorRef.current
                .chain()
                .focus()
                .insertContentAt(
                  { from: mentionState.from, to: mentionState.to },
                  `@${item.handle} `,
                )
                .run()
              setMentionState(null)
            }}
          />
        </div>
      ) : null}
      {/* Uploaded files preview */}
      {uploadedFiles.length > 0 && onRemoveUploadedFile && (
        <div className="flex flex-wrap gap-1.5 border-t border-border/60 px-3 py-2">
          {uploadedFiles.map((f) => {
            const ext = getFileExtension(f.name)
            return (
              <div
                key={f.id}
                className="flex items-center gap-1.5 rounded-md border border-border/60 bg-muted/20 pl-2 pr-1 py-1 text-xs"
              >
                <span className="uppercase text-[10px] font-medium text-muted-foreground/70 w-6">{ext || 'FILE'}</span>
                <span className="truncate max-w-[120px] text-muted-foreground">{f.name}</span>
                <button
                  type="button"
                  onClick={() => onRemoveUploadedFile(f.id)}
                  className="flex h-4 w-4 items-center justify-center rounded text-muted-foreground hover:text-foreground cursor-pointer"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            )
          })}
        </div>
      )}
      <div className="flex items-center justify-between px-2 py-1">
        <div className="flex items-center gap-1">
          <span className="text-[10px] text-muted-foreground">
            {hasMentionables ? 'Type @ to mention' : ''}
          </span>
          {uploadConfig && (
            <button
              type="button"
              className="inline-flex h-6 w-6 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
              title="Add image"
              onClick={() => {
                const input = document.createElement('input')
                input.type = 'file'
                input.accept = 'image/*'
                input.onchange = () => {
                  const file = input.files?.[0]
                  if (file) handleImageUpload(file, editor)
                }
                input.click()
              }}
            >
              <ImageIcon className="h-3.5 w-3.5" />
            </button>
          )}
          {onFileSelect && (
            <button
              type="button"
              className="inline-flex h-6 w-6 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
              title="Attach file"
              onClick={onFileSelect}
            >
              <Paperclip className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
        <button
          type="button"
          className="inline-flex h-7 w-7 items-center justify-center rounded-full border border-border/60 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer disabled:opacity-40"
          disabled={!canSubmit}
          onClick={handleSubmit}
        >
          {loading ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Send className="h-3.5 w-3.5" />
          )}
        </button>
      </div>
    </div>
  )
}
