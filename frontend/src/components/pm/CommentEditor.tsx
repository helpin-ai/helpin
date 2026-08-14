import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { MentionHighlight } from '@/components/pm/mention-highlight'
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList'
import { useRef, useState, useCallback, useEffect, useMemo } from 'react'
import { Loading01Icon, SentIcon, Image01Icon, AttachmentIcon, Cancel01Icon } from '@/lib/icons'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { ImageLightbox } from '@/components/pm/ImageLightbox'
import { EmojiPicker } from '@/components/support/EmojiPicker'
import type { WorkspaceTeam, AssignableMember } from '@/lib/types'
import { cn } from '@/lib/utils'
import {
  getMemberMentionHandle,
  getMentionSuggestions,
  type MentionSuggestionItem,
} from '@/components/pm/mentionSuggestions'

interface CommentEditorProps {
  onSubmit: (text: string) => void | Promise<void>
  loading?: boolean
  placeholder?: string
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  members?: AssignableMember[]
  onImageSelect?: (files: File[]) => void
  onFileSelect?: () => void
  uploadedFiles?: { id: string; name: string; url?: string }[]
  onRemoveUploadedFile?: (id: string) => void
  initialContent?: string
  onCancel?: () => void
  autoFocus?: boolean
  enableEmojiPicker?: boolean
  /** Visual variant — 'update' is the divider-based task activity composer. */
  variant?: 'primary' | 'reply' | 'legacy' | 'update'
  contentVariant?: 'default' | 'pm'
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
  onImageSelect,
  onFileSelect,
  uploadedFiles = [],
  onRemoveUploadedFile,
  initialContent,
  onCancel,
  variant = 'legacy',
  contentVariant = 'default',
  autoFocus = false,
  enableEmojiPicker = false,
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
  const onImageSelectRef = useRef(onImageSelect)
  onImageSelectRef.current = onImageSelect
  const [hasContent, setHasContent] = useState(false)
  const [lightboxFileId, setLightboxFileId] = useState<string | null>(null)
  const currentHtmlRef = useRef('')
  const skipNextCleanupRef = useRef(false)

  const getContent = useCallback((editor: ReturnType<typeof useEditor>) => {
    if (!editor) return ''
    // Always return HTML so links, formatting, and mentions are preserved
    return editor.getHTML()
  }, [])

  const handleImageFiles = useCallback((files: File[]) => {
    const imageFiles = files.filter((f) => f.type.startsWith('image/'))
    if (imageFiles.length > 0) onImageSelectRef.current?.(imageFiles)
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

  const insertEmoji = useCallback((emoji: string) => {
    editorRef.current?.chain().focus().insertContent(emoji).run()
  }, [])

  const extensions = useMemo(() => [
    StarterKit.configure({
      heading: false,
      blockquote: false,
      codeBlock: false,
      horizontalRule: false,
      bulletList: false,
      orderedList: false,
      listItem: false,
      link: {
        openOnClick: false,
        autolink: true,
        HTMLAttributes: { class: 'text-blue-600 dark:text-blue-400 underline cursor-pointer', target: '_blank', rel: 'noopener noreferrer' },
      },
    }),
    Placeholder.configure({ placeholder, showOnlyCurrent: false, emptyNodeClass: 'is-empty', emptyEditorClass: 'is-editor-empty' }),
    MentionHighlight.configure({
      validHandles: () => {
        const handles = new Set<string>()
        for (const m of membersRef.current) {
          const handle = getMemberMentionHandle(m)
          if (handle) handles.add(handle.toLowerCase())
        }
        for (const t of teamsRef.current) {
          if (t.handle) handles.add(t.handle.toLowerCase())
        }
        return handles
      },
    }),
  ], [placeholder])

  const editor = useEditor({
    extensions,
    content: initialContent ?? '',
    autofocus: autoFocus,
    editorProps: {
      attributes: {
        class: cn(
          'rich-text-soft prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[40px] max-h-[120px] overflow-y-auto px-3 py-2 bg-transparent',
          contentVariant === 'pm' ? 'pm-rich-text' : 'text-[13px]',
        ),
      },
      handlePaste: (_view, event) => {
        if (!onImageSelectRef.current) return false
        const items = event.clipboardData?.items
        if (!items) return false
        const images: File[] = []
        for (const item of items) {
          if (item.type.startsWith('image/')) {
            const file = item.getAsFile()
            if (file) images.push(file)
          }
        }
        if (images.length > 0) {
          event.preventDefault()
          handleImageFiles(images)
          return true
        }
        return false
      },
      handleDrop: (_view, event, _slice, moved) => {
        if (!onImageSelectRef.current || moved) return false
        const files = event.dataTransfer?.files
        if (!files?.length) return false
        const images = Array.from(files).filter((f) => f.type.startsWith('image/'))
        if (images.length > 0) {
          event.preventDefault()
          handleImageFiles(images)
          return true
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
    onUpdate: ({ editor: e }) => {
      setHasContent(!e.isEmpty)
      currentHtmlRef.current = editorRef.current?.getHTML() ?? ''
      if (skipNextCleanupRef.current) {
        skipNextCleanupRef.current = false
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
      currentHtmlRef.current = editor.getHTML()
      if (skipNextCleanupRef.current) {
        skipNextCleanupRef.current = false
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
  }, [editor])

  if (!editor) return null

  const canSubmit = !loading && (hasContent || uploadedFiles.length > 0)
  const previewFiles = uploadedFiles.filter((file) => {
    const ext = getFileExtension(file.name)
    return Boolean(file.url) && /^(png|jpg|jpeg|gif|webp|svg|bmp)$/i.test(ext)
  })
  const activePreviewIndex = lightboxFileId
    ? previewFiles.findIndex((file) => file.id === lightboxFileId)
    : -1
  const activePreviewFile = activePreviewIndex >= 0 ? previewFiles[activePreviewIndex] : null

  const wrapperClass =
    variant === 'update'
      ? 'group/update-composer relative bg-transparent pt-2 pb-1.5 [&_.ProseMirror]:min-h-14 [&_.ProseMirror]:px-0 [&_.ProseMirror]:py-2'
      : variant === 'primary'
      ? 'relative rounded-lg border border-border bg-muted/70 px-3 pt-2 pb-1.5 transition-[color,box-shadow,background-color] focus-within:bg-background focus-within:ring-1 focus-within:ring-ring/40'
      : variant === 'reply'
        ? 'relative rounded-md border border-border/60 bg-background px-2.5 pt-1.5 pb-1 transition-[color,box-shadow,background-color] focus-within:ring-1 focus-within:ring-ring/40'
        : 'relative bg-muted/50 px-3 pt-2 pb-1.5 rounded-b-lg transition-[color,box-shadow,background-color] focus-within:ring-1 focus-within:ring-ring/40'

  return (
    <div data-variant={variant} className={wrapperClass}>
      {mentionState && mentionState.items.length > 0 ? (
        <div
          className="absolute bottom-full left-0 right-0 z-50 mb-1.5"
          onMouseDown={(e) => e.preventDefault()}
        >
          <div className="mx-1 max-h-[260px] overflow-y-auto rounded-xl border border-border/60 bg-popover p-1.5 shadow-lg">
            <p className="px-2 pb-1 pt-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">
              Suggestions
            </p>
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
        </div>
      ) : null}
      <EditorContent editor={editor} />
      {/* Uploaded files / image previews */}
      {uploadedFiles.length > 0 && onRemoveUploadedFile && (
        <div className="flex flex-wrap gap-1 px-1 pt-1">
          {uploadedFiles.map((f) => {
            const ext = getFileExtension(f.name)
            const isImage = /^(png|jpg|jpeg|gif|webp|svg|bmp)$/i.test(ext)
            return (
              <div
                key={f.id}
                className="group/file relative flex items-center gap-1.5 rounded-md border border-border/60 bg-background/60 pl-1 pr-0.5 py-0.5 text-xs"
              >
                {f.url && isImage ? (
                  <button
                    type="button"
                    onClick={() => setLightboxFileId(f.id)}
                    className="flex items-center gap-1.5 min-w-0 hover:text-foreground cursor-pointer"
                  >
                    <img src={f.url} alt={f.name} className="h-6 w-6 rounded object-cover" />
                    <span className="truncate max-w-[120px] text-muted-foreground">{f.name}</span>
                  </button>
                ) : f.url ? (
                  <a
                    href={f.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-1.5 min-w-0 hover:text-foreground"
                  >
                    <span className="uppercase text-[10px] font-medium text-muted-foreground/70 w-5">{ext || 'FILE'}</span>
                    <span className="truncate max-w-[120px] text-muted-foreground">{f.name}</span>
                  </a>
                ) : (
                  <>
                    <span className="uppercase text-[10px] font-medium text-muted-foreground/70 w-5">{ext || 'FILE'}</span>
                    <span className="truncate max-w-[120px] text-muted-foreground">{f.name}</span>
                  </>
                )}
                <button
                  type="button"
                  onClick={() => onRemoveUploadedFile(f.id)}
                  className="flex h-4 w-4 items-center justify-center rounded text-muted-foreground hover:text-foreground cursor-pointer"
                >
                  <Cancel01Icon className="h-3 w-3" />
                </button>
              </div>
            )
          })}
        </div>
      )}
      <div className={cn(
        'flex items-center justify-between gap-2 px-1 pt-1.5 pb-0.5',
        variant === 'update' && 'border-t border-border/40 px-0 pt-1.5 transition-colors group-focus-within/update-composer:border-foreground/70',
      )}>
        <div className="flex items-center gap-0.5">
          {onImageSelect && (
            <QuickTooltip label="Add image">
              <button
                type="button"
                className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                onClick={() => {
                  const input = document.createElement('input')
                  input.type = 'file'
                  input.accept = 'image/*'
                  input.multiple = true
                  input.onchange = () => {
                    if (input.files?.length) handleImageFiles(Array.from(input.files))
                  }
                  input.click()
                }}
              >
                <Image01Icon className="h-4 w-4" />
              </button>
            </QuickTooltip>
          )}
          {onFileSelect && (
            <QuickTooltip label="Attach file">
              <button
                type="button"
                className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
                onClick={onFileSelect}
              >
                <AttachmentIcon className="h-4 w-4" />
              </button>
            </QuickTooltip>
          )}
          {enableEmojiPicker && (
            <EmojiPicker
              align="start"
              side="top"
              onEmojiSelect={insertEmoji}
            />
          )}
        </div>
        <div className="flex items-center gap-1.5">
          {onCancel && (
            <button
              type="button"
              onClick={onCancel}
              className="inline-flex h-8 items-center justify-center rounded-md px-2.5 text-xs text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer"
            >
              Cancel
            </button>
          )}
          <kbd className={cn(
            'hidden items-center gap-1 leading-none text-muted-foreground sm:inline-flex',
            variant === 'update' ? 'font-sans text-xs' : 'font-mono text-[15px]',
          )}>
            <span>{navigator.platform?.includes('Mac') ? '⌘' : 'Ctrl'}</span>
            <span>{'↵'}</span>
          </kbd>
          <button
            type="button"
            className={
              canSubmit
                ? 'inline-flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-sm transition-all hover:bg-primary/90 hover:shadow active:scale-95 cursor-pointer'
                : 'inline-flex h-8 w-8 items-center justify-center rounded-full border border-border bg-background/60 text-muted-foreground/50 transition-all duration-200 cursor-not-allowed'
            }
            disabled={!canSubmit}
            onClick={handleSubmit}
          >
            {loading ? (
              <Loading01Icon className="h-4 w-4 animate-spin" />
            ) : (
              <SentIcon className="h-4 w-4 rotate-45" />
            )}
          </button>
        </div>
      </div>
      {activePreviewFile?.url && (
        <ImageLightbox
          src={activePreviewFile.url}
          alt={activePreviewFile.name}
          onClose={() => setLightboxFileId(null)}
          hasPrevious={activePreviewIndex > 0}
          hasNext={activePreviewIndex < previewFiles.length - 1}
          onPrevious={() => {
            if (activePreviewIndex > 0) {
              setLightboxFileId(previewFiles[activePreviewIndex - 1].id)
            }
          }}
          onNext={() => {
            if (activePreviewIndex < previewFiles.length - 1) {
              setLightboxFileId(previewFiles[activePreviewIndex + 1].id)
            }
          }}
          positionLabel={previewFiles.length > 1 ? `${activePreviewIndex + 1} / ${previewFiles.length}` : undefined}
        />
      )}
    </div>
  )
}
