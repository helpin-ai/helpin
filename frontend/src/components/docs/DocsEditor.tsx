import { useCallback, useEffect, useRef, useState } from 'react'
import { useEditor, EditorContent, type JSONContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Placeholder from '@tiptap/extension-placeholder'
import Link from '@tiptap/extension-link'
import { Markdown } from 'tiptap-markdown'
import {
  Bold,
  Check,
  ChevronDown,
  Code2,
  Copy,
  FileDown,
  FileUp,
  Heading2,
  ImagePlus,
  Italic,
  Link2,
  List,
  ListOrdered,
  Loader2,
  Quote,
  Strikethrough,
  X,
} from 'lucide-react'
import { ResizableImageExtension } from '@/components/ui/resizable-image-extension'
import { uploadEditorImage, type EditorUploadConfig } from '@/hooks/useEditorImageUpload'
import { toast } from 'sonner'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

// ── Toolbar button ──────────────────────────────────────────────────────────

function ToolbarButton({
  onClick,
  active,
  children,
  title,
}: {
  onClick: () => void
  active?: boolean
  children: React.ReactNode
  title: string
}) {
  return (
    <QuickTooltip label={title} side="bottom">
      <button
        type="button"
        onMouseDown={(e) => {
          e.preventDefault() // prevent losing selection
          onClick()
        }}
        className={`rounded p-1.5 transition-colors ${
          active
            ? 'bg-white/20 text-white'
            : 'text-white/70 hover:bg-white/10 hover:text-white'
        }`}
      >
        {children}
      </button>
    </QuickTooltip>
  )
}

// ── Save status indicator ───────────────────────────────────────────────────

type SaveStatus = 'idle' | 'saved' | 'saving' | 'unsaved'

function formatLastSaved(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 60) return 'a few seconds ago'
  const minutes = Math.floor(seconds / 60)
  if (minutes === 1) return '1 minute ago'
  if (minutes < 60) return `${minutes} minutes ago`
  const hours = Math.floor(minutes / 60)
  if (hours === 1) return '1 hour ago'
  return `${hours} hours ago`
}

function SaveIndicator({ status, lastSavedAt }: { status: SaveStatus; lastSavedAt: Date | null }) {
  const [, setTick] = useState(0)

  // Re-render every 30s to update "last saved X ago"
  useEffect(() => {
    if (!lastSavedAt || status === 'saving') return
    const interval = setInterval(() => setTick((t) => t + 1), 30_000)
    return () => clearInterval(interval)
  }, [lastSavedAt, status])

  if (status === 'idle' && !lastSavedAt) return null

  switch (status) {
    case 'saving':
      return (
        <span className="flex items-center gap-1 text-[11px] text-muted-foreground">
          <Loader2 className="h-3 w-3 animate-spin" />
          Saving...
        </span>
      )
    case 'saved':
      return (
        <span className="flex items-center gap-1 text-[11px] text-green-600">
          <Check className="h-3 w-3" />
          Saved
        </span>
      )
    case 'unsaved':
      return (
        <span className="text-[11px] text-muted-foreground">Unsaved changes</span>
      )
    default:
      // idle but has lastSavedAt — show "Last saved X ago"
      if (lastSavedAt) {
        return (
          <span className="text-[11px] text-muted-foreground">
            Last saved {formatLastSaved(lastSavedAt)}
          </span>
        )
      }
      return null
  }
}

// ── Floating toolbar ────────────────────────────────────────────────────────

function FloatingToolbar({ editor, uploadConfig, onInsertImage }: {
  editor: ReturnType<typeof useEditor>
  uploadConfig?: EditorUploadConfig
  onInsertImage: () => void
}) {
  const toolbarRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null)

  useEffect(() => {
    if (!editor) return

    const updatePosition = () => {
      const { from, to, empty } = editor.state.selection
      if (empty || from === to) {
        setPos(null)
        return
      }

      const domSelection = window.getSelection()
      if (!domSelection || domSelection.rangeCount === 0) {
        setPos(null)
        return
      }

      const range = domSelection.getRangeAt(0)
      const rect = range.getBoundingClientRect()
      if (rect.width === 0) {
        setPos(null)
        return
      }

      const toolbar = toolbarRef.current
      const toolbarWidth = toolbar?.offsetWidth ?? 300

      setPos({
        top: rect.top + window.scrollY - 45,
        left: rect.left + window.scrollX + rect.width / 2 - toolbarWidth / 2,
      })
    }

    editor.on('selectionUpdate', updatePosition)
    editor.on('blur', () => setPos(null))

    return () => {
      editor.off('selectionUpdate', updatePosition)
      editor.off('blur', () => setPos(null))
    }
  }, [editor])

  if (!editor || !pos) return null

  const addLink = () => {
    const url = window.prompt('URL')
    if (!url) return
    editor.chain().focus().setLink({ href: url }).run()
  }

  return (
    <div
      ref={toolbarRef}
      className="fixed z-50 flex items-center gap-0.5 rounded-lg bg-foreground px-1 py-0.5 shadow-xl animate-in fade-in zoom-in-95 duration-150"
      style={{ top: pos.top, left: pos.left }}
    >
      <ToolbarButton
        title="Bold"
        onClick={() => editor.chain().focus().toggleBold().run()}
        active={editor.isActive('bold')}
      >
        <Bold className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton
        title="Italic"
        onClick={() => editor.chain().focus().toggleItalic().run()}
        active={editor.isActive('italic')}
      >
        <Italic className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton
        title="Strikethrough"
        onClick={() => editor.chain().focus().toggleStrike().run()}
        active={editor.isActive('strike')}
      >
        <Strikethrough className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton
        title="Code"
        onClick={() => editor.chain().focus().toggleCode().run()}
        active={editor.isActive('code')}
      >
        <Code2 className="h-3.5 w-3.5" />
      </ToolbarButton>

      <div className="mx-0.5 h-4 w-px bg-white/20" />

      <ToolbarButton
        title="Heading"
        onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
        active={editor.isActive('heading')}
      >
        <Heading2 className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton
        title="Bullet list"
        onClick={() => editor.chain().focus().toggleBulletList().run()}
        active={editor.isActive('bulletList')}
      >
        <List className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton
        title="Ordered list"
        onClick={() => editor.chain().focus().toggleOrderedList().run()}
        active={editor.isActive('orderedList')}
      >
        <ListOrdered className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton
        title="Blockquote"
        onClick={() => editor.chain().focus().toggleBlockquote().run()}
        active={editor.isActive('blockquote')}
      >
        <Quote className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Link" onClick={addLink} active={editor.isActive('link')}>
        <Link2 className="h-3.5 w-3.5" />
      </ToolbarButton>

      {uploadConfig && (
        <>
          <div className="mx-0.5 h-4 w-px bg-white/20" />
          <ToolbarButton title="Insert image" onClick={onInsertImage}>
            <ImagePlus className="h-3.5 w-3.5" />
          </ToolbarButton>
        </>
      )}
    </div>
  )
}

// ── Markdown menu ──────────────────────────────────────────────────────────

function MarkdownMenu({
  onCopyMarkdown,
  onImportMarkdown,
  onDownloadMarkdown,
  onUploadMarkdownFile,
  onToggleSource,
  sourceView,
}: {
  onCopyMarkdown: () => void
  onImportMarkdown: () => void
  onDownloadMarkdown: () => void
  onUploadMarkdownFile: () => void
  onToggleSource: () => void
  sourceView: boolean
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        >
          MD
          <ChevronDown className="h-3 w-3" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuItem onClick={onCopyMarkdown}>
          <Copy className="h-3.5 w-3.5 mr-2" />
          Copy as Markdown
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onDownloadMarkdown}>
          <FileDown className="h-3.5 w-3.5 mr-2" />
          Download as .md
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onImportMarkdown}>
          <FileUp className="h-3.5 w-3.5 mr-2" />
          Import from text
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onUploadMarkdownFile}>
          <FileUp className="h-3.5 w-3.5 mr-2" />
          Upload .md file
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onToggleSource}>
          <Code2 className="h-3.5 w-3.5 mr-2" />
          {sourceView ? 'Back to Rich Editor' : 'Markdown Source'}
          <span className="ml-auto text-[10px] text-muted-foreground">
            {navigator.platform.includes('Mac') ? '⌘⇧M' : 'Ctrl+⇧+M'}
          </span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ── Main editor ─────────────────────────────────────────────────────────────

interface DocsEditorProps {
  title?: string
  onTitleChange?: (title: string) => void
  initialContent?: JSONContent | null
  onSave: (content: JSONContent) => Promise<void>
  autoSaveMs?: number
  readOnly?: boolean
  uploadConfig?: EditorUploadConfig
}

export function DocsEditor({
  title,
  onTitleChange,
  initialContent,
  onSave,
  autoSaveMs = 2000,
  readOnly = false,
  uploadConfig,
}: DocsEditorProps) {
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('idle')
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null)
  const saveTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const savedFadeTimerRef = useRef<ReturnType<typeof setTimeout>>()
  const savingRef = useRef(false)
  const pendingContentRef = useRef<JSONContent | null>(null)

  // Markdown feature state
  const [sourceView, setSourceView] = useState(false)
  const [sourceMarkdown, setSourceMarkdown] = useState('')
  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [importText, setImportText] = useState('')

  const doSave = useCallback(
    async (json: JSONContent) => {
      if (savingRef.current) {
        pendingContentRef.current = json
        return
      }
      savingRef.current = true
      setSaveStatus('saving')
      try {
        await onSave(json)
        setSaveStatus('saved')
        setLastSavedAt(new Date())
        if (savedFadeTimerRef.current) clearTimeout(savedFadeTimerRef.current)
        savedFadeTimerRef.current = setTimeout(() => setSaveStatus('idle'), 5000)
      } catch {
        setSaveStatus('unsaved')
      } finally {
        savingRef.current = false
        if (pendingContentRef.current) {
          const next = pendingContentRef.current
          pendingContentRef.current = null
          doSave(next)
        }
      }
    },
    [onSave],
  )

  const scheduleSave = useCallback(
    (json: JSONContent) => {
      setSaveStatus('unsaved')
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
      saveTimerRef.current = setTimeout(() => doSave(json), autoSaveMs)
    },
    [doSave, autoSaveMs],
  )

  // Cleanup timers on unmount
  useEffect(() => () => {
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
    if (savedFadeTimerRef.current) clearTimeout(savedFadeTimerRef.current)
  }, [])

  const uploadConfigRef = useRef(uploadConfig)
  uploadConfigRef.current = uploadConfig
  const editorRef = useRef<ReturnType<typeof useEditor>>(null)

  const handleImageUpload = useCallback(
    async (file: File, editorInstance: ReturnType<typeof useEditor>) => {
      if (!editorInstance || !uploadConfigRef.current) return
      if (!file.type.startsWith('image/')) return

      const uploadId = `img-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`

      // Insert a placeholder with data-uri
      const reader = new FileReader()
      reader.onload = async () => {
        const dataUri = reader.result as string
        editorInstance
          .chain()
          .focus()
          .setResizableImage({ src: dataUri, alt: file.name, title: uploadId })
          .run()

        try {
          const publicUrl = await uploadEditorImage(file, uploadConfigRef.current!)

          // Replace data-uri with permanent URL
          const { doc } = editorInstance.state
          let targetPos: number | null = null
          doc.descendants((node, pos) => {
            if (node.type.name === 'resizableImage' && node.attrs.title === uploadId) {
              targetPos = pos
              return false
            }
          })

          if (targetPos !== null) {
            const tr = editorInstance.state.tr
            const node = doc.nodeAt(targetPos)
            if (node) {
              tr.setNodeMarkup(targetPos, undefined, {
                ...node.attrs,
                src: publicUrl,
                title: null,
              })
              editorInstance.view.dispatch(tr)
            }
          }
        } catch (err) {
          toast.error(err instanceof Error ? err.message : 'Failed to upload image')
        }
      }
      reader.readAsDataURL(file)
    },
    [],
  )

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
      }),
      Placeholder.configure({ placeholder: 'Start writing your document...' }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { class: 'text-primary underline cursor-pointer' },
      }),
      ResizableImageExtension,
      Markdown.configure({
        html: true,
        tightLists: true,
        bulletListMarker: '-',
        transformPastedText: true,
        transformCopiedText: false, // Don't force clipboard to markdown — we have explicit "Copy as Markdown"
      }),
    ],
    content: initialContent ?? { type: 'doc', content: [{ type: 'paragraph' }] },
    editable: !readOnly,
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[400px] px-6 py-4',
      },
      handlePaste(view, event) {
        const items = event.clipboardData?.items
        if (!items) return false
        for (const item of items) {
          if (item.type.startsWith('image/')) {
            event.preventDefault()
            const file = item.getAsFile()
            if (file) {
              handleImageUpload(file, editorRef.current)
            }
            return true
          }
        }
        return false
      },
      handleDrop(view, event) {
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
    },
    onUpdate: ({ editor: e }) => {
      if (!readOnly) {
        scheduleSave(e.getJSON())
      }
    },
  })

  editorRef.current = editor

  // Sync editable state when readOnly prop changes (e.g. after unlock)
  useEffect(() => {
    if (editor) {
      editor.setEditable(!readOnly)
    }
  }, [editor, readOnly])

  // Update content if initial content changes (e.g. after revert)
  useEffect(() => {
    if (editor && initialContent) {
      const currentJson = JSON.stringify(editor.getJSON())
      const newJson = JSON.stringify(initialContent)
      if (currentJson !== newJson) {
        editor.commands.setContent(initialContent)
        setSaveStatus('idle')
      }
    }
  }, [editor, initialContent])

  const insertImage = useCallback(() => {
    if (!editor) return
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/*'
    input.onchange = () => {
      const file = input.files?.[0]
      if (file) {
        handleImageUpload(file, editor)
      }
    }
    input.click()
  }, [editor, handleImageUpload])

  // ── Markdown actions ───────────────────────────────────────────────────────

  const getMarkdown = useCallback((): string => {
    if (!editor) return ''
    return editor.storage.markdown.getMarkdown()
  }, [editor])

  const copyAsMarkdown = useCallback(() => {
    const md = getMarkdown()
    navigator.clipboard.writeText(md).then(
      () => toast.success('Copied as Markdown'),
      () => toast.error('Failed to copy'),
    )
  }, [getMarkdown])

  const handleImportMarkdown = useCallback(() => {
    if (!editor || !importText.trim()) return
    editor.commands.setContent(importText.trim())
    scheduleSave(editor.getJSON())
    setImportDialogOpen(false)
    setImportText('')
    toast.success('Markdown imported')
  }, [editor, importText, scheduleSave])

  const toggleSourceView = useCallback(() => {
    if (!editor) return
    if (!sourceView) {
      // Entering source view — snapshot current markdown
      setSourceMarkdown(getMarkdown())
      setSourceView(true)
    } else {
      // Leaving source view — apply markdown changes back to editor
      editor.commands.setContent(sourceMarkdown)
      scheduleSave(editor.getJSON())
      setSourceView(false)
    }
  }, [editor, sourceView, sourceMarkdown, getMarkdown, scheduleSave])

  const discardSourceView = useCallback(() => {
    setSourceView(false)
  }, [])

  // ── .md file download ──────────────────────────────────────────────────────

  const downloadAsMarkdown = useCallback(() => {
    const md = getMarkdown()
    const filename = `${(title || 'document').replace(/[^a-z0-9_-]/gi, '_').toLowerCase()}.md`
    const blob = new Blob([md], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }, [getMarkdown, title])

  // ── .md file upload ────────────────────────────────────────────────────────

  const uploadMarkdownFile = useCallback(() => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.md,.markdown,text/markdown'
    input.onchange = () => {
      const file = input.files?.[0]
      if (!file || !editor) return
      const reader = new FileReader()
      reader.onload = () => {
        const md = reader.result as string
        editor.commands.setContent(md)
        scheduleSave(editor.getJSON())
        toast.success(`Imported "${file.name}"`)
      }
      reader.onerror = () => toast.error('Failed to read file')
      reader.readAsText(file)
    }
    input.click()
  }, [editor, scheduleSave])

  // ── Keyboard shortcut: Ctrl+Shift+M → toggle source view ──────────────────

  useEffect(() => {
    if (readOnly) return
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'M') {
        e.preventDefault()
        toggleSourceView()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [readOnly, toggleSourceView])

  // ── Detect _markdown_source from backend AI import ─────────────────────────

  useEffect(() => {
    if (!editor || !initialContent) return
    const raw = initialContent as Record<string, unknown>
    if (typeof raw._markdown_source === 'string') {
      // Backend stored raw markdown — parse it via tiptap-markdown and auto-save as JSON
      editor.commands.setContent(raw._markdown_source as string)
      scheduleSave(editor.getJSON())
    }
  }, [editor, initialContent, scheduleSave])

  if (!editor) return null

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      {/* Floating toolbar — appears on text selection */}
      {!readOnly && !sourceView && (
        <FloatingToolbar
          editor={editor}
          uploadConfig={uploadConfig}
          onInsertImage={insertImage}
        />
      )}

      {/* Editor content with title */}
      <div className="relative flex-1 overflow-y-auto">
        {/* Save indicator + Markdown menu — top-right floating */}
        {!readOnly ? (
          <div className="sticky top-2 z-10 flex justify-end px-4 pointer-events-none">
            <div className="pointer-events-auto flex items-center gap-1.5 rounded-md bg-background/80 backdrop-blur-sm px-2 py-0.5 shadow-sm border border-border/40">
              <SaveIndicator status={saveStatus} lastSavedAt={lastSavedAt} />
              <MarkdownMenu
                onCopyMarkdown={copyAsMarkdown}
                onImportMarkdown={() => setImportDialogOpen(true)}
                onDownloadMarkdown={downloadAsMarkdown}
                onUploadMarkdownFile={uploadMarkdownFile}
                onToggleSource={toggleSourceView}
                sourceView={sourceView}
              />
            </div>
          </div>
        ) : (
          <div className="sticky top-2 z-10 flex justify-end px-4 pointer-events-none">
            <div className="pointer-events-auto">
              <QuickTooltip label="Copy as Markdown" side="left">
                <button
                  type="button"
                  onClick={copyAsMarkdown}
                  className="inline-flex items-center gap-1 rounded-md bg-background/80 backdrop-blur-sm px-2 py-1 text-[11px] text-muted-foreground shadow-sm border border-border/40 transition-colors hover:text-foreground"
                >
                  <Copy className="h-3 w-3" />
                  MD
                </button>
              </QuickTooltip>
            </div>
          </div>
        )}

        <div className="mx-auto max-w-3xl">
          {/* Title */}
          {title !== undefined && (
            <div className="px-6 pt-10 pb-1">
              {onTitleChange && !readOnly ? (
                <input
                  value={title}
                  onChange={(e) => onTitleChange(e.target.value)}
                  placeholder="Untitled"
                  className="w-full bg-transparent text-3xl font-bold text-left outline-none placeholder:text-muted-foreground/40"
                />
              ) : (
                <h1 className="text-3xl font-bold text-left">{title || 'Untitled'}</h1>
              )}
            </div>
          )}

          {/* Rich editor (default) or Markdown source view */}
          {sourceView ? (
            <div className="px-6 py-4">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  Markdown Source
                </span>
                <div className="flex items-center gap-1.5">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 gap-1.5 text-xs"
                    onClick={discardSourceView}
                  >
                    <X className="h-3 w-3" />
                    Discard
                  </Button>
                  <Button
                    size="sm"
                    className="h-7 gap-1.5 text-xs"
                    onClick={toggleSourceView}
                  >
                    <Check className="h-3 w-3" />
                    Apply
                  </Button>
                </div>
              </div>
              <textarea
                value={sourceMarkdown}
                onChange={(e) => setSourceMarkdown(e.target.value)}
                className="w-full min-h-[400px] rounded-lg border border-border/60 bg-muted/30 p-4 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground/40 focus:outline-none focus:ring-1 focus:ring-ring resize-y"
                placeholder="Markdown content..."
                spellCheck={false}
              />
            </div>
          ) : (
            <EditorContent editor={editor} />
          )}
        </div>
      </div>

      {/* Import Markdown dialog */}
      <Dialog open={importDialogOpen} onOpenChange={setImportDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Import Markdown</DialogTitle>
            <DialogDescription>
              Paste Markdown content below. It will replace the current document content.
            </DialogDescription>
          </DialogHeader>
          <textarea
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
            className="w-full min-h-[250px] rounded-lg border border-border/60 bg-muted/30 p-4 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground/40 focus:outline-none focus:ring-1 focus:ring-ring resize-y"
            placeholder="# Paste your Markdown here..."
            autoFocus
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => { setImportDialogOpen(false); setImportText('') }}>
              Cancel
            </Button>
            <Button onClick={handleImportMarkdown} disabled={!importText.trim()}>
              <FileDown className="h-3.5 w-3.5 mr-1.5" />
              Import
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
