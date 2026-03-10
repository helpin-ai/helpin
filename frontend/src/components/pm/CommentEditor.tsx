import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import { MentionHighlight } from '@/components/pm/mention-highlight'
import { ResizableImageExtension } from '@/components/ui/resizable-image-extension'
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload'
import { useRef, useState, useCallback, useEffect, useMemo } from 'react'
import { Loader2, Send, ImageIcon } from 'lucide-react'
import { toast } from 'sonner'
import type { WorkspaceTeam, AssignableMember } from '@/lib/types'
import { UserAvatar } from '@/components/pm/UserAvatar'

interface MentionItem {
  id: string
  name: string
  handle: string
  type: 'member' | 'team'
  avatarUrl?: string
}

interface CommentEditorProps {
  onSubmit: (text: string) => void | Promise<void>
  loading?: boolean
  placeholder?: string
  teams?: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[]
  members?: AssignableMember[]
  uploadConfig?: EditorUploadConfig
}

function buildMemberHandle(member: AssignableMember): string {
  return member.display_name.toLowerCase().replace(/\s+/g, '.')
}

function detectMentions(
  editorInstance: ReturnType<typeof useEditor>,
  teams: Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>[],
  members: AssignableMember[],
): { from: number; to: number; items: MentionItem[]; selectedIndex: number } | null {
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
  const items: MentionItem[] = []

  for (const m of members) {
    if (m.status !== 'active') continue
    const handle = buildMemberHandle(m)
    if (
      !query ||
      handle.includes(query) ||
      m.display_name.toLowerCase().includes(query) ||
      m.email.toLowerCase().includes(query)
    ) {
      items.push({
        id: m.user_id || m.id,
        name: m.display_name,
        handle,
        type: 'member',
        avatarUrl: m.avatar_url,
      })
    }
  }

  for (const t of teams) {
    if (!t.handle) continue
    if (
      !query ||
      t.handle.toLowerCase().includes(query) ||
      t.name.toLowerCase().includes(query)
    ) {
      items.push({
        id: t.id,
        name: t.name,
        handle: t.handle,
        type: 'team',
      })
    }
  }

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
}: CommentEditorProps) {
  const [mentionState, setMentionState] = useState<{
    from: number
    to: number
    items: MentionItem[]
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

      try {
        const publicUrl = await uploadEditorImage(file, uploadConfigRef.current!)

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
                src: publicUrl,
                title: null,
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
      }
    },
    [],
  )

  const handleSubmit = useCallback(() => {
    if (!editorRef.current || loading) return
    const text = getContent(editorRef.current)
    if (!text.trim()) return
    onSubmit(text.trim())
    editorRef.current.commands.clearContent()
  }, [onSubmit, loading, getContent])

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

  const content = getContent(editor)
  const canSubmit = !loading && content.trim().length > 0

  return (
    <div className="rounded-lg border border-border/60 bg-background transition-colors focus-within:border-border">
      <EditorContent editor={editor} />
      {mentionState && mentionState.items.length > 0 ? (
        <div className="border-t border-border/60 bg-muted/40 px-2 py-2">
          <div className="mb-1 px-2 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
            Mention
          </div>
          <div className="max-h-[200px] overflow-y-auto space-y-0.5">
            {mentionState.items.map((item, index) => (
              <button
                key={`${item.type}-${item.id}`}
                type="button"
                className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors ${
                  index === mentionState.selectedIndex
                    ? 'bg-accent text-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                }`}
                onMouseDown={(e) => {
                  e.preventDefault()
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
              >
                {item.type === 'member' ? (
                  <UserAvatar
                    name={item.name}
                    avatarUrl={item.avatarUrl}
                    className="h-5 w-5 text-[10px]"
                  />
                ) : (
                  <div className="flex h-5 w-5 items-center justify-center rounded-full bg-muted text-[10px] font-medium">
                    T
                  </div>
                )}
                <span className="flex-1 truncate">{item.name}</span>
                <span className="font-mono text-xs text-muted-foreground">
                  @{item.handle}
                </span>
              </button>
            ))}
          </div>
        </div>
      ) : null}
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
